package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sync"
	"sync/atomic"
	"time"

	"github.com/godeps/claude-agent-sdk-go/internal/log"
	"github.com/godeps/claude-agent-sdk-go/internal/mcp"
	"github.com/godeps/claude-agent-sdk-go/internal/transport"
	"github.com/godeps/claude-agent-sdk-go/types"
)

// Query manages bidirectional control message handling.
// It orchestrates message routing between the transport and application callbacks,
// handling permissions, hooks, and MCP message routing.
type Query struct {
	// Transport and lifecycle
	transport transport.Transport
	ctx       context.Context
	cancel    context.CancelFunc
	logger    *log.Logger

	// Request tracking
	mu                 sync.Mutex
	requestMap         map[string]chan responseResult
	nextRequestID      int64
	hookCallbacks      map[string]types.HookCallbackFunc
	nextHookCallbackID int64

	// In-flight control request cancellation
	inflightMu     sync.Mutex
	inflightCancel map[string]context.CancelFunc

	// Callbacks
	canUseTool types.CanUseToolFunc
	hooks      map[types.HookEvent][]types.HookMatcher
	mcpServers map[string]types.MCPServer

	// Tool execution handlers
	toolHandlers       map[string]types.ToolHandlerFunc
	toolHandlerTimeout time.Duration
	pendingToolResults map[string]chan *types.ToolResult
	pendingMu          sync.Mutex

	// Message handling
	messagesChan     chan types.Message
	stopChan         chan struct{}
	readLoopDone     chan struct{}
	started          bool
	initialized      bool
	initializeResult map[string]interface{}
	isStreamingMode  bool

	// Event tracking
	eventTracker *EventTracker
}

// responseResult wraps the response or error from a control request.
type responseResult struct {
	response map[string]interface{}
	err      error
}

// NewQuery creates a new Query handler.
func NewQuery(ctx context.Context, transport transport.Transport, opts *types.ClaudeAgentOptions, logger *log.Logger, isStreamingMode bool) *Query {
	queryCtx, cancel := context.WithCancel(ctx)

	// Use configurable message channel capacity with default of 100
	capacity := 100
	if opts != nil && opts.MessageChannelCapacity != nil {
		capacity = *opts.MessageChannelCapacity
	}

	q := &Query{
		transport:          transport,
		ctx:                queryCtx,
		cancel:             cancel,
		logger:             logger,
		requestMap:         make(map[string]chan responseResult),
		hookCallbacks:      make(map[string]types.HookCallbackFunc),
		inflightCancel:     make(map[string]context.CancelFunc),
		messagesChan:       make(chan types.Message, capacity),
		stopChan:           make(chan struct{}),
		readLoopDone:       make(chan struct{}),
		isStreamingMode:    isStreamingMode,
		mcpServers:         make(map[string]types.MCPServer),
		pendingToolResults: make(map[string]chan *types.ToolResult),
		toolHandlerTimeout: 5 * time.Minute,
	}

	if opts != nil {
		q.canUseTool = opts.CanUseTool
		q.hooks = opts.Hooks
		q.toolHandlers = opts.ToolHandlers
		if opts.ToolHandlerTimeout != nil {
			q.toolHandlerTimeout = *opts.ToolHandlerTimeout
		}
	}

	q.eventTracker = NewEventTracker(opts, cancel)

	return q
}

// Initialize sends initialization control request if in streaming mode.
func (q *Query) Initialize(ctx context.Context) (map[string]interface{}, error) {
	if !q.isStreamingMode {
		return nil, nil
	}

	if q.initialized {
		return q.initializeResult, nil
	}

	q.logger.Debug("Initializing control protocol...")

	// Build hooks configuration
	hooksConfig := make(map[string]interface{})
	if q.hooks != nil {
		for event, matchers := range q.hooks {
			if len(matchers) == 0 {
				continue
			}

			eventHooks := make([]map[string]interface{}, 0, len(matchers))
			for _, matcher := range matchers {
				callbackIDs := make([]string, 0, len(matcher.Hooks))
				for _, callback := range matcher.Hooks {
					callbackID := q.registerHookCallback(callback)
					callbackIDs = append(callbackIDs, callbackID)
				}

				hookConfig := map[string]interface{}{
					"hookCallbackIds": callbackIDs,
				}
				if matcher.Matcher != nil {
					hookConfig["matcher"] = *matcher.Matcher
				}
				eventHooks = append(eventHooks, hookConfig)
			}
			hooksConfig[string(event)] = eventHooks
		}
	}

	// Send initialize request
	request := map[string]interface{}{
		"subtype": "initialize",
	}
	if len(hooksConfig) > 0 {
		request["hooks"] = hooksConfig
	}

	result, err := q.sendControlRequest(ctx, request)
	if err != nil {
		q.logger.Error("Control protocol initialization failed: %v", err)
		return nil, types.NewControlProtocolErrorWithCause("initialization failed", err)
	}

	q.initialized = true
	q.initializeResult = result
	q.logger.Debug("Control protocol initialized successfully")
	return result, nil
}

// Start begins the control message handling loop.
func (q *Query) Start(ctx context.Context) error {
	q.mu.Lock()
	if q.started {
		q.mu.Unlock()
		return types.NewControlProtocolError("query already started")
	}
	q.started = true
	q.mu.Unlock()

	// Start message reading loop
	go q.messageLoop()

	return nil
}

// Stop gracefully stops the query handler.
func (q *Query) Stop(ctx context.Context) error {
	// Signal stop
	select {
	case <-q.stopChan:
		// Already stopped
		return nil
	default:
		close(q.stopChan)
	}

	// Cancel context to stop all operations
	q.cancel()

	// Wait for read loop to complete
	select {
	case <-q.readLoopDone:
	case <-ctx.Done():
		return ctx.Err()
	}

	// Close message channel
	close(q.messagesChan)

	return nil
}

// GetMessages returns a channel for consuming normal (non-control) messages.
func (q *Query) GetMessages(ctx context.Context) <-chan types.Message {
	return q.messagesChan
}

// messageLoop reads messages from transport and routes them.
func (q *Query) messageLoop() {
	defer close(q.readLoopDone)

	messages := q.transport.ReadMessages(q.ctx)
	q.logger.Debug("Message routing loop started")

	for {
		select {
		case <-q.ctx.Done():
			q.logger.Debug("Message loop stopped: context cancelled")
			return
		case <-q.stopChan:
			q.logger.Debug("Message loop stopped: stop signal received")
			return
		case msg, ok := <-messages:
			if !ok {
				q.logger.Debug("Message loop stopped: transport channel closed")
				// Channel closed - transport has stopped
				return
			}

			// Route message based on type
			if err := q.routeMessage(msg); err != nil {
				q.logger.Warning("Message routing error: %v", err)
				// Log error but continue processing
				// In a production system, we might want to report this via an error channel
				continue
			}
		}
	}
}

// routeMessage routes a message to the appropriate handler.
func (q *Query) routeMessage(msg types.Message) error {
	// Check message type
	msgType := msg.GetMessageType()
	q.logger.Debug("Routing message: type=%s", msgType)

	// Handle control responses
	if msgType == "control_response" {
		if sysMsg, ok := msg.(*types.SystemMessage); ok {
			return q.handleControlResponse(sysMsg)
		}
		return types.NewControlProtocolError("invalid control_response message type")
	}

	// Handle control requests
	if msgType == "control_request" {
		q.logger.Debug("Handling control request from CLI")
		if sysMsg, ok := msg.(*types.SystemMessage); ok {
			go q.handleControlRequest(sysMsg)
			return nil
		}
		return types.NewControlProtocolError("invalid control_request message type")
	}

	// Handle control cancel requests
	if msgType == "control_cancel_request" {
		if sysMsg, ok := msg.(*types.SystemMessage); ok {
			cancelID := sysMsg.RequestID
			if cancelID == "" {
				if sysMsg.Request != nil {
					cancelID, _ = sysMsg.Request["request_id"].(string)
				}
			}
			if cancelID != "" {
				q.inflightMu.Lock()
				cancelFn, exists := q.inflightCancel[cancelID]
				if exists {
					delete(q.inflightCancel, cancelID)
				}
				q.inflightMu.Unlock()
				if exists {
					q.logger.Debug("Cancelling in-flight request: %s", cancelID)
					cancelFn()
				}
			}
			return nil
		}
		return types.NewControlProtocolError("invalid control_cancel_request message type")
	}

	// Regular message - observe for events, then send to consumer
	if q.eventTracker != nil {
		q.eventTracker.Observe(msg)
	}
	select {
	case q.messagesChan <- msg:
		return nil
	case <-q.ctx.Done():
		return q.ctx.Err()
	}
}

// handleControlResponse handles a control response message.
func (q *Query) handleControlResponse(msg *types.SystemMessage) error {
	// Parse response - use msg.Response for control_response messages
	responseData := msg.Response
	if responseData == nil {
		return types.NewControlProtocolError("invalid control response format: response field is nil")
	}

	requestID, ok := responseData["request_id"].(string)
	if !ok {
		return types.NewControlProtocolError("missing request_id in control response")
	}

	// Find pending request
	q.mu.Lock()
	responseChan, exists := q.requestMap[requestID]
	if exists {
		delete(q.requestMap, requestID)
	}
	q.mu.Unlock()

	if !exists {
		// Orphaned response - might be a timeout or duplicate
		return nil
	}

	// Check for error response
	subtype, _ := responseData["subtype"].(string)
	if subtype == "error" {
		errMsg, _ := responseData["error"].(string)
		if errMsg == "" {
			errMsg = "unknown control protocol error"
		}
		select {
		case responseChan <- responseResult{err: types.NewControlProtocolError(errMsg)}:
		case <-q.ctx.Done():
		}
		return nil
	}

	// Success response
	response, _ := responseData["response"].(map[string]interface{})
	select {
	case responseChan <- responseResult{response: response}:
	case <-q.ctx.Done():
	}

	return nil
}

// handleControlRequest handles an incoming control request from CLI.
func (q *Query) handleControlRequest(msg *types.SystemMessage) {
	q.logger.Debug("handleControlRequest: entered, msg.RequestID='%s', msg.Request=%+v", msg.RequestID, msg.Request)

	// Get request ID from top-level field (CLI sends it here)
	requestID := msg.RequestID

	// Get request data from Request field
	requestData := msg.Request

	q.logger.Debug("handleControlRequest: requestID='%s', requestData=%+v", requestID, requestData)

	// For CLI-initiated requests (like can_use_tool), there might not be a request_id
	// Generate one if needed
	if requestID == "" {
		requestID = fmt.Sprintf("cli-request-%d", atomic.AddInt64(&q.nextRequestID, 1))
		q.logger.Debug("handleControlRequest: generated requestID=%s for CLI-initiated request", requestID)
	}

	if requestData == nil {
		q.logger.Error("handleControlRequest: invalid control request format: requestData is nil")
		q.sendErrorResponse(requestID, "invalid control request format")
		return
	}

	subtype, _ := requestData["subtype"].(string)
	q.logger.Debug("handleControlRequest: subtype=%s", subtype)

	// Register cancellable context for this request
	reqCtx, reqCancel := context.WithCancel(q.ctx)
	q.inflightMu.Lock()
	q.inflightCancel[requestID] = reqCancel
	q.inflightMu.Unlock()
	defer func() {
		q.inflightMu.Lock()
		delete(q.inflightCancel, requestID)
		q.inflightMu.Unlock()
		reqCancel()
	}()

	var response map[string]interface{}
	var err error

	switch subtype {
	case "can_use_tool":
		response, err = q.handlePermissionRequest(reqCtx, requestData)
	case "hook_callback":
		response, err = q.handleHookCallback(requestData)
	case "mcp_message":
		response, err = q.handleMCPMessage(requestData)
	case "interrupt":
		response = make(map[string]interface{})
	case "set_permission_mode":
		response = make(map[string]interface{})
	default:
		err = types.NewControlProtocolError("unsupported control request subtype: " + subtype)
	}

	// If cancelled, don't send a response (CLI already abandoned the request)
	if reqCtx.Err() != nil {
		q.logger.Debug("handleControlRequest: request %s was cancelled, skipping response", requestID)
		return
	}

	if err != nil {
		q.sendErrorResponse(requestID, err.Error())
		return
	}

	q.sendSuccessResponse(requestID, response)
}

// handlePermissionRequest handles a permission request for tool use.
func (q *Query) handlePermissionRequest(reqCtx context.Context, requestData map[string]interface{}) (map[string]interface{}, error) {
	q.logger.Debug("handlePermissionRequest: entered, requestData=%+v", requestData)

	toolName, _ := requestData["tool_name"].(string)
	input, _ := requestData["input"].(map[string]interface{})
	suggestions, _ := requestData["permission_suggestions"].([]interface{})

	q.logger.Debug("handlePermissionRequest: toolName=%s, input=%+v", toolName, input)

	if toolName == "" || input == nil {
		q.logger.Error("handlePermissionRequest: missing tool_name or input")
		return nil, types.NewControlProtocolError("missing tool_name or input in permission request")
	}

	// Check tool handlers first (takes priority over canUseTool)
	if q.toolHandlers != nil {
		if handler, registered := q.toolHandlers[toolName]; registered {
			return q.executeToolHandler(toolName, input, requestData, handler)
		}
	}

	if q.canUseTool == nil {
		q.logger.Error("handlePermissionRequest: canUseTool callback is nil!")
		return nil, types.NewControlProtocolError("canUseTool callback is not provided")
	}

	q.logger.Debug("handlePermissionRequest: canUseTool callback is set")

	// Build permission context with all available fields
	permissionUpdates := make([]types.PermissionUpdate, 0)
	for _, s := range suggestions {
		if suggestionMap, ok := s.(map[string]interface{}); ok {
			suggestionJSON, _ := json.Marshal(suggestionMap)
			var update types.PermissionUpdate
			if err := json.Unmarshal(suggestionJSON, &update); err == nil {
				permissionUpdates = append(permissionUpdates, update)
			}
		}
	}

	toolUseID, _ := requestData["tool_use_id"].(string)
	agentID, _ := requestData["agent_id"].(string)
	decisionReason, _ := requestData["decision_reason"].(string)
	title, _ := requestData["title"].(string)
	displayName, _ := requestData["display_name"].(string)
	description, _ := requestData["description"].(string)

	blockedPath := ""
	if bp, ok := requestData["blocked_path"].(string); ok {
		blockedPath = bp
	}

	permCtx := types.ToolPermissionContext{
		Suggestions:    permissionUpdates,
		ToolUseID:      toolUseID,
		AgentID:        agentID,
		BlockedPath:    blockedPath,
		DecisionReason: decisionReason,
		Title:          title,
		DisplayName:    displayName,
		Description:    description,
	}

	// Call permission callback with cancellable context
	q.logger.Debug("handlePermissionRequest: CALLING canUseTool callback for tool=%s", toolName)
	result, err := q.canUseTool(reqCtx, toolName, input, permCtx)
	q.logger.Debug("handlePermissionRequest: canUseTool callback returned: result=%+v, err=%v", result, err)
	if err != nil {
		q.logger.Error("handlePermissionRequest: canUseTool callback returned error: %v", err)
		return nil, err
	}

	// Convert result to response format
	response := make(map[string]interface{})

	switch r := result.(type) {
	case types.PermissionResultAllow:
		response["behavior"] = "allow"
		if r.UpdatedInput != nil {
			response["updatedInput"] = *r.UpdatedInput
		} else {
			response["updatedInput"] = input
		}
		if len(r.UpdatedPermissions) > 0 {
			response["updatedPermissions"] = r.UpdatedPermissions
		}

	case *types.PermissionResultAllow:
		response["behavior"] = "allow"
		if r.UpdatedInput != nil {
			response["updatedInput"] = *r.UpdatedInput
		} else {
			response["updatedInput"] = input
		}
		if len(r.UpdatedPermissions) > 0 {
			response["updatedPermissions"] = r.UpdatedPermissions
		}

	case types.PermissionResultDeny:
		response["behavior"] = "deny"
		if r.Message != "" {
			response["message"] = r.Message
		}
		if r.Interrupt {
			response["interrupt"] = r.Interrupt
		}

	case *types.PermissionResultDeny:
		response["behavior"] = "deny"
		if r.Message != "" {
			response["message"] = r.Message
		}
		if r.Interrupt {
			response["interrupt"] = r.Interrupt
		}

	default:
		return nil, types.NewControlProtocolError("permission callback returned invalid type")
	}

	return response, nil
}

// handleHookCallback handles a hook callback request.
func (q *Query) handleHookCallback(requestData map[string]interface{}) (map[string]interface{}, error) {
	callbackID, _ := requestData["callback_id"].(string)
	input := requestData["input"]
	toolUseID, _ := requestData["tool_use_id"].(*string)

	if callbackID == "" {
		return nil, types.NewControlProtocolError("missing callback_id in hook callback request")
	}

	// Find callback
	q.mu.Lock()
	callback, exists := q.hookCallbacks[callbackID]
	q.mu.Unlock()

	if !exists {
		return nil, types.NewControlProtocolError("no hook callback found for ID: " + callbackID)
	}

	// Build hook context
	hookCtx := types.HookContext{}

	// Call hook callback
	hookOutput, err := callback(q.ctx, input, toolUseID, hookCtx)
	if err != nil {
		return nil, err
	}

	// Convert hook output to response
	// The callback should return a map[string]interface{} representing the hook output
	response, ok := hookOutput.(map[string]interface{})
	if !ok {
		return nil, types.NewControlProtocolError("hook callback must return map[string]interface{}")
	}

	return response, nil
}

// handleMCPMessage handles an MCP message request.
func (q *Query) handleMCPMessage(requestData map[string]interface{}) (map[string]interface{}, error) {
	serverName, _ := requestData["server_name"].(string)
	message, _ := requestData["message"].(map[string]interface{})

	if serverName == "" || message == nil {
		return nil, types.NewControlProtocolError("missing server_name or message in MCP request")
	}

	// Find MCP server
	q.mu.Lock()
	server, exists := q.mcpServers[serverName]
	q.mu.Unlock()

	if !exists {
		// Return JSONRPC error response
		messageID := message["id"]
		return map[string]interface{}{
			"mcp_response": map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      messageID,
				"error": map[string]interface{}{
					"code":    -32601,
					"message": fmt.Sprintf("Server '%s' not found", serverName),
				},
			},
		}, nil
	}

	// Route message to MCP server
	mcpResponse, err := server.HandleMessage(message)
	if err != nil {
		// Return JSONRPC error response
		messageID := message["id"]
		return map[string]interface{}{
			"mcp_response": map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      messageID,
				"error": map[string]interface{}{
					"code":    -32603,
					"message": err.Error(),
				},
			},
		}, nil
	}

	return map[string]interface{}{
		"mcp_response": mcpResponse,
	}, nil
}

// sendControlRequest sends a control request to CLI and waits for response.
func (q *Query) sendControlRequest(ctx context.Context, request map[string]interface{}) (map[string]interface{}, error) {
	if !q.isStreamingMode {
		return nil, types.NewControlProtocolError("control requests require streaming mode")
	}

	// Generate unique request ID
	requestID := q.generateRequestID()

	// Create response channel
	responseChan := make(chan responseResult, 1)
	q.mu.Lock()
	q.requestMap[requestID] = responseChan
	q.mu.Unlock()

	// Build control request
	controlRequest := map[string]interface{}{
		"type":       "control_request",
		"request_id": requestID,
		"request":    request,
	}

	// Marshal and send
	data, err := json.Marshal(controlRequest)
	if err != nil {
		q.mu.Lock()
		delete(q.requestMap, requestID)
		q.mu.Unlock()
		return nil, types.NewControlProtocolErrorWithCause("failed to marshal control request", err)
	}

	if err := q.transport.Write(ctx, string(data)); err != nil {
		q.mu.Lock()
		delete(q.requestMap, requestID)
		q.mu.Unlock()
		return nil, types.NewControlProtocolErrorWithCause("failed to send control request", err)
	}

	// Wait for response with timeout
	select {
	case result := <-responseChan:
		if result.err != nil {
			return nil, result.err
		}
		return result.response, nil
	case <-ctx.Done():
		q.mu.Lock()
		delete(q.requestMap, requestID)
		q.mu.Unlock()
		return nil, ctx.Err()
	}
}

// SendControlRequest exposes control requests to callers (streaming mode only).
func (q *Query) SendControlRequest(ctx context.Context, request map[string]interface{}) (map[string]interface{}, error) {
	return q.sendControlRequest(ctx, request)
}

// sendSuccessResponse sends a success control response.
func (q *Query) sendSuccessResponse(requestID string, response map[string]interface{}) {
	controlResponse := map[string]interface{}{
		"type": "control_response",
		"response": map[string]interface{}{
			"subtype":    "success",
			"request_id": requestID,
			"response":   response,
		},
	}

	data, err := json.Marshal(controlResponse)
	if err != nil {
		q.logger.Error("sendSuccessResponse: failed to marshal response: %v", err)
		return
	}

	q.logger.Debug("sendSuccessResponse: sending control_response: %s", string(data))
	if err := q.transport.Write(q.ctx, string(data)); err != nil {
		q.logger.Error("sendSuccessResponse: failed to write: %v", err)
	}
}

// sendErrorResponse sends an error control response.
func (q *Query) sendErrorResponse(requestID string, errorMsg string) {
	controlResponse := map[string]interface{}{
		"type": "control_response",
		"response": map[string]interface{}{
			"subtype":    "error",
			"request_id": requestID,
			"error":      errorMsg,
		},
	}

	data, err := json.Marshal(controlResponse)
	if err != nil {
		return
	}

	_ = q.transport.Write(q.ctx, string(data))
}

// generateRequestID generates a unique request ID.
func (q *Query) generateRequestID() string {
	id := atomic.AddInt64(&q.nextRequestID, 1)
	return fmt.Sprintf("req_%d", id)
}

// registerHookCallback registers a hook callback and returns its ID.
func (q *Query) registerHookCallback(callback types.HookCallbackFunc) string {
	q.mu.Lock()
	defer q.mu.Unlock()

	id := atomic.AddInt64(&q.nextHookCallbackID, 1)
	callbackID := fmt.Sprintf("hook_%d", id)
	q.hookCallbacks[callbackID] = callback
	return callbackID
}

// AddMCPServer adds an MCP server for handling MCP messages.
func (q *Query) AddMCPServer(name string, server types.MCPServer) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.mcpServers[name] = server
}

// ConfigureMCPServers registers SDK MCP servers defined in options with the query handler.
// This allows the control protocol to route mcp_message control requests to in-process tools.
func (q *Query) ConfigureMCPServers(opts *types.ClaudeAgentOptions) error {
	if opts == nil || opts.McpServers == nil {
		return nil
	}

	servers, ok := opts.McpServers.(map[string]interface{})
	if !ok {
		return nil
	}

	for name, serverConfig := range servers {
		toolConfig, ok := serverConfig.(*types.ToolServerConfig)
		if !ok {
			continue // Not an SDK MCP server configuration
		}

		if toolConfig.Type != "" && toolConfig.Type != "sdk" {
			continue
		}

		server, err := instantiateSdkServer(toolConfig)
		if err != nil {
			return fmt.Errorf("configure MCP server %s: %w", name, err)
		}

		q.AddMCPServer(name, server)
	}

	return nil
}

// instantiateSdkServer converts a ToolServerConfig into a concrete MCP server implementation.
func instantiateSdkServer(config *types.ToolServerConfig) (types.MCPServer, error) {
	if config == nil {
		return nil, fmt.Errorf("nil SDK MCP server configuration")
	}

	switch instance := config.Instance.(type) {
	case nil:
		return nil, fmt.Errorf("SDK MCP server %s has no instance", config.Name)
	case types.MCPServer:
		return instance, nil
	case []types.McpTool:
		server := mcp.NewSdkMCPServer(config.Name, config.Version, instance)
		config.Instance = server
		return server, nil
	default:
		if server, ok := instance.(types.MCPServer); ok {
			return server, nil
		}
		return nil, fmt.Errorf("unsupported SDK MCP server instance type %T", instance)
	}
}

// matchesToolName checks if a tool name matches a matcher pattern.
// nolint:unused
func matchesToolName(toolName string, pattern *string) bool {
	if pattern == nil || *pattern == "" {
		return true // No pattern means match all
	}

	// Use regex for pattern matching
	regex, err := regexp.Compile(*pattern)
	if err != nil {
		return false
	}

	return regex.MatchString(toolName)
}

// executeToolHandler handles a tool execution request via registered ToolHandler.
// If handler is non-nil, it calls the handler directly (callback mode).
// If handler is nil, it emits a ToolExecutionRequest and waits for SubmitToolResult (event-stream mode).
func (q *Query) executeToolHandler(toolName string, input map[string]interface{}, requestData map[string]interface{}, handler types.ToolHandlerFunc) (map[string]interface{}, error) {
	toolUseID, _ := requestData["tool_use_id"].(string)
	if toolUseID == "" {
		toolUseID = fmt.Sprintf("tool_%d", atomic.AddInt64(&q.nextRequestID, 1))
	}

	q.logger.Debug("executeToolHandler: toolName=%s, toolUseID=%s, callbackMode=%v", toolName, toolUseID, handler != nil)

	var result *types.ToolResult
	var err error

	if handler != nil {
		// Callback mode: call handler directly
		req := types.ToolHandlerRequest{
			ToolUseID: toolUseID,
			ToolName:  toolName,
			Input:     input,
		}
		result, err = handler(q.ctx, req)
		if err != nil {
			q.logger.Error("executeToolHandler: handler error: %v", err)
			return q.buildDenyResponse(fmt.Sprintf("tool handler error: %v", err)), nil
		}
	} else {
		// Event-stream mode: emit request and wait for SubmitToolResult
		resultChan := make(chan *types.ToolResult, 1)
		q.pendingMu.Lock()
		q.pendingToolResults[toolUseID] = resultChan
		q.pendingMu.Unlock()

		defer func() {
			q.pendingMu.Lock()
			delete(q.pendingToolResults, toolUseID)
			q.pendingMu.Unlock()
		}()

		// Emit ToolExecutionRequest to message channel
		execReq := &types.ToolExecutionRequest{
			Type:      "tool_execution_request",
			ToolUseID: toolUseID,
			ToolName:  toolName,
			Input:     input,
		}

		select {
		case q.messagesChan <- execReq:
		case <-q.ctx.Done():
			return nil, q.ctx.Err()
		}

		// Wait for result with timeout
		timeout := time.NewTimer(q.toolHandlerTimeout)
		defer timeout.Stop()

		select {
		case result = <-resultChan:
			// Got result from SubmitToolResult
		case <-timeout.C:
			q.logger.Warning("executeToolHandler: timeout waiting for tool result, toolUseID=%s", toolUseID)
			return q.buildDenyResponse("tool execution timed out: no result submitted within deadline"), nil
		case <-q.ctx.Done():
			return nil, q.ctx.Err()
		}
	}

	if result == nil {
		return q.buildDenyResponse("tool handler returned nil result"), nil
	}

	return q.buildExecuteResponse(result), nil
}

// SubmitToolResult provides a tool execution result for event-stream mode.
// This unblocks the pending handlePermissionRequest waiting for this toolUseID.
func (q *Query) SubmitToolResult(toolUseID string, result *types.ToolResult) error {
	q.pendingMu.Lock()
	ch, exists := q.pendingToolResults[toolUseID]
	q.pendingMu.Unlock()

	if !exists {
		return fmt.Errorf("no pending tool execution request for toolUseID: %s", toolUseID)
	}

	select {
	case ch <- result:
		return nil
	default:
		return fmt.Errorf("result already submitted for toolUseID: %s", toolUseID)
	}
}

// buildExecuteResponse constructs a permission response with behavior "result".
func (q *Query) buildExecuteResponse(result *types.ToolResult) map[string]interface{} {
	content := make([]interface{}, 0, len(result.Content))
	for _, block := range result.Content {
		switch b := block.(type) {
		case types.TextBlock:
			content = append(content, map[string]interface{}{
				"type": "text",
				"text": b.Text,
			})
		case *types.TextBlock:
			content = append(content, map[string]interface{}{
				"type": "text",
				"text": b.Text,
			})
		default:
			content = append(content, block)
		}
	}

	return map[string]interface{}{
		"behavior": "result",
		"result": map[string]interface{}{
			"content":  content,
			"is_error": result.IsError,
		},
	}
}

// buildDenyResponse constructs a deny permission response with a message.
func (q *Query) buildDenyResponse(message string) map[string]interface{} {
	return map[string]interface{}{
		"behavior": "deny",
		"message":  message,
	}
}
