package types

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"time"
)

// SettingSource represents where settings are loaded from.
type SettingSource string

const (
	SettingSourceUser    SettingSource = "user"
	SettingSourceProject SettingSource = "project"
	SettingSourceLocal   SettingSource = "local"
)

// SdkBeta represents supported Anthropic API beta headers.
type SdkBeta string

const (
	// SdkBetaContext1M enables extended context window.
	SdkBetaContext1M SdkBeta = "context-1m-2025-08-07"
)

// SystemPromptPreset represents a preset system prompt configuration.
type SystemPromptPreset struct {
	Type   string  `json:"type"`   // "preset"
	Preset string  `json:"preset"` // "claude_code"
	Append *string `json:"append,omitempty"`
}

// ToolsPreset represents a preset tool configuration.
type ToolsPreset struct {
	Type   string `json:"type"`   // "preset"
	Preset string `json:"preset"` // "claude_code"
}

// AgentDefinition represents a custom agent definition.
type AgentDefinition struct {
	Description string   `json:"description"`
	Prompt      string   `json:"prompt"`
	Tools       []string `json:"tools,omitempty"`
	Model       *string  `json:"model,omitempty"` // "sonnet", "opus", "haiku", "inherit"
}

// SdkPluginConfig represents a plugin configuration.
type SdkPluginConfig struct {
	Type string `json:"type"` // "local"
	Path string `json:"path"`
}

// McpStdioServerConfig represents an MCP stdio server configuration.
type McpStdioServerConfig struct {
	Type    *string           `json:"type,omitempty"` // "stdio" - optional for backwards compatibility
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// McpSSEServerConfig represents an MCP SSE server configuration.
type McpSSEServerConfig struct {
	Type    string            `json:"type"` // "sse"
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

// McpHTTPServerConfig represents an MCP HTTP server configuration.
type McpHTTPServerConfig struct {
	Type    string            `json:"type"` // "http"
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

// McpStreamableHTTPServerConfig represents an MCP Streamable HTTP server configuration.
type McpStreamableHTTPServerConfig struct {
	Type    string            `json:"type"` // "streamable-http"
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

// McpSdkServerConfig represents an SDK MCP server configuration.
type McpSdkServerConfig struct {
	Type     string      `json:"type"` // "sdk"
	Name     string      `json:"name"`
	Instance interface{} `json:"instance"` // MCP Server instance - type depends on MCP SDK
}

// CanUseToolFunc is a callback function for tool permission requests.
// It receives the tool name, input parameters, and context, and returns a permission result.
type CanUseToolFunc func(ctx context.Context, toolName string, input map[string]interface{}, permCtx ToolPermissionContext) (interface{}, error)

// HookCallbackFunc is a callback function for hook events.
// It receives the hook input, optional tool use ID, and context, and returns hook output.
type HookCallbackFunc func(ctx context.Context, input interface{}, toolUseID *string, hookCtx HookContext) (interface{}, error)

// HookMatcher represents a hook matcher configuration.
type HookMatcher struct {
	Matcher *string            `json:"matcher,omitempty"` // Regex pattern for matching (e.g., "Bash", "Write|Edit")
	Hooks   []HookCallbackFunc `json:"-"`                 // List of hook callback functions (not marshaled)
}

// StderrCallbackFunc is a callback function for stderr output from the CLI.
type StderrCallbackFunc func(line string)

// ClaudeAgentOptions represents configuration options for the Claude SDK.
type ClaudeAgentOptions struct {
	// Base tool configuration (set of tools available to Claude).
	// Supports []string or ToolsPreset. An empty slice disables all built-in tools.
	Tools interface{} `json:"tools,omitempty"`

	// Tool configuration
	AllowedTools    []string `json:"allowed_tools,omitempty"`
	DisallowedTools []string `json:"disallowed_tools,omitempty"`

	// System prompt - can be string or SystemPromptPreset
	SystemPrompt interface{} `json:"system_prompt,omitempty"`

	// MCP servers - can be map[string]interface{} (config), string (path), or actual path
	McpServers interface{} `json:"mcp_servers,omitempty"`

	// Permission configuration
	PermissionMode           *PermissionMode `json:"permission_mode,omitempty"`
	PermissionPromptToolName *string         `json:"permission_prompt_tool_name,omitempty"` // Tool name to use for permission prompts

	// Permission bypass configuration (use with caution - only for sandboxed environments)
	// These flags disable ALL permission checks, allowing Claude to execute any tool without approval.
	//
	// DangerouslySkipPermissions: Actually bypass all permissions (requires AllowDangerouslySkipPermissions)
	// AllowDangerouslySkipPermissions: Enable permission bypass as an option
	//
	// Security Warning: Only use in isolated environments with no internet access.
	DangerouslySkipPermissions      bool `json:"dangerously_skip_permissions,omitempty"`
	AllowDangerouslySkipPermissions bool `json:"allow_dangerously_skip_permissions,omitempty"`

	// Session configuration
	ContinueConversation bool    `json:"continue_conversation,omitempty"`
	Resume               *string `json:"resume,omitempty"`
	ForkSession          bool    `json:"fork_session,omitempty"`

	// Model and execution limits
	Model             *string   `json:"model,omitempty"`
	FallbackModel     *string   `json:"fallback_model,omitempty"` // Fallback model if primary model is unavailable
	MaxTurns          *int      `json:"max_turns,omitempty"`
	MaxThinkingTokens *int      `json:"max_thinking_tokens,omitempty"` // Maximum tokens for extended thinking
	MaxBudgetUSD      *float64  `json:"max_budget_usd,omitempty"`      // Maximum budget in USD for this query
	Betas             []SdkBeta `json:"betas,omitempty"`               // Beta feature flags

	// API configuration
	BaseURL *string `json:"base_url,omitempty"` // Custom Anthropic API base URL (ANTHROPIC_BASE_URL)

	// Working directory and CLI path
	CWD     *string `json:"cwd,omitempty"`
	CLIPath *string `json:"cli_path,omitempty"`

	// Settings
	Settings       *string         `json:"settings,omitempty"`
	SettingSources []SettingSource `json:"setting_sources,omitempty"`
	AddDirs        []string        `json:"add_dirs,omitempty"`

	// Environment and extra arguments
	Env       map[string]string  `json:"env,omitempty"`
	ExtraArgs map[string]*string `json:"extra_args,omitempty"` // Pass arbitrary CLI flags

	// Buffer configuration
	MaxBufferSize          *int `json:"max_buffer_size,omitempty"`          // Max bytes when buffering CLI stdout
	MessageChannelCapacity *int `json:"message_channel_capacity,omitempty"` // Capacity for message channels

	// Streaming configuration
	IncludePartialMessages bool `json:"include_partial_messages,omitempty"`

	// Output format for structured outputs (e.g., JSON schema)
	OutputFormat map[string]interface{} `json:"output_format,omitempty"`

	// User identifier
	User *string `json:"user,omitempty"`

	// Agent definitions
	Agents map[string]AgentDefinition `json:"agents,omitempty"`

	// Plugin configurations
	Plugins []SdkPluginConfig `json:"plugins,omitempty"`

	// File checkpointing
	EnableFileCheckpointing bool `json:"enable_file_checkpointing,omitempty"`

	// Debug and diagnostics
	Verbose bool `json:"-"` // Enable verbose debug logging

	// CLI behavior flags
	BareMode         bool                   `json:"bare_mode,omitempty"`
	NoMarkdown       bool                   `json:"no_markdown,omitempty"`
	SettingsOverride map[string]interface{} `json:"settings_override,omitempty"`

	// Callbacks (not marshaled to JSON)
	CanUseTool CanUseToolFunc              `json:"-"`
	Hooks      map[HookEvent][]HookMatcher `json:"-"`
	Stderr     StderrCallbackFunc          `json:"-"`

	// Tool execution handlers - intercept tool execution and provide results directly.
	// Keys are tool names (e.g., "AskUserQuestion"). A non-nil handler uses callback mode;
	// a nil handler uses event-stream mode (ToolExecutionRequest emitted via ReceiveResponse).
	ToolHandlers       map[string]ToolHandlerFunc `json:"-"`
	ToolHandlerTimeout *time.Duration             `json:"-"`

	// Event callbacks
	OnToolEvent ToolEventHandler `json:"-"`
	OnProgress  ProgressHandler  `json:"-"`

	// Structured logging
	Logger   *slog.Logger `json:"-"`
	LogLevel *slog.Level  `json:"-"`

	// Retry configuration
	RetryConfig *RetryConfig `json:"-"`

	// Cost guard
	CostLimitUSD      *float64           `json:"-"`
	OnCostLimitExceed func(spent float64) `json:"-"`

	// Auth provider
	AuthProvider AuthProvider `json:"-"`
}

// NewClaudeAgentOptions creates a new ClaudeAgentOptions with sensible defaults.
func NewClaudeAgentOptions() *ClaudeAgentOptions {
	return &ClaudeAgentOptions{
		AllowedTools:           []string{},
		DisallowedTools:        []string{},
		Env:                    make(map[string]string),
		ExtraArgs:              make(map[string]*string),
		ContinueConversation:   false,
		ForkSession:            false,
		IncludePartialMessages: false,
	}
}

// WithAllowedTools sets the allowed tools.
func (o *ClaudeAgentOptions) WithAllowedTools(tools ...string) *ClaudeAgentOptions {
	o.AllowedTools = tools
	return o
}

// WithTools sets the base tool set available to Claude (overrides default preset).
// Pass an empty slice to disable all built-in tools.
func (o *ClaudeAgentOptions) WithTools(tools ...string) *ClaudeAgentOptions {
	o.Tools = tools
	return o
}

// WithToolsPreset sets a preset tool configuration (e.g., claude_code).
func (o *ClaudeAgentOptions) WithToolsPreset(preset ToolsPreset) *ClaudeAgentOptions {
	o.Tools = preset
	return o
}

// WithDisallowedTools sets the disallowed tools.
func (o *ClaudeAgentOptions) WithDisallowedTools(tools ...string) *ClaudeAgentOptions {
	o.DisallowedTools = tools
	return o
}

// WithSystemPrompt sets the system prompt (can be string or SystemPromptPreset).
func (o *ClaudeAgentOptions) WithSystemPrompt(prompt interface{}) *ClaudeAgentOptions {
	o.SystemPrompt = prompt
	return o
}

// WithSystemPromptString sets the system prompt as a string.
func (o *ClaudeAgentOptions) WithSystemPromptString(prompt string) *ClaudeAgentOptions {
	o.SystemPrompt = prompt
	return o
}

// WithSystemPromptPreset sets the system prompt as a preset.
func (o *ClaudeAgentOptions) WithSystemPromptPreset(preset SystemPromptPreset) *ClaudeAgentOptions {
	o.SystemPrompt = preset
	return o
}

// WithMcpServers sets the MCP servers configuration.
func (o *ClaudeAgentOptions) WithMcpServers(servers interface{}) *ClaudeAgentOptions {
	o.McpServers = servers
	return o
}

// WithPermissionMode sets the permission mode.
func (o *ClaudeAgentOptions) WithPermissionMode(mode PermissionMode) *ClaudeAgentOptions {
	o.PermissionMode = &mode
	return o
}

// WithPermissionPromptToolName sets the permission prompt tool name.
func (o *ClaudeAgentOptions) WithPermissionPromptToolName(toolName string) *ClaudeAgentOptions {
	o.PermissionPromptToolName = &toolName
	return o
}

// WithContinueConversation sets whether to continue the conversation.
func (o *ClaudeAgentOptions) WithContinueConversation(continue_ bool) *ClaudeAgentOptions {
	o.ContinueConversation = continue_
	return o
}

// WithResume sets the session ID to resume.
func (o *ClaudeAgentOptions) WithResume(sessionID string) *ClaudeAgentOptions {
	o.Resume = &sessionID
	return o
}

// WithForkSession sets whether to fork the session.
func (o *ClaudeAgentOptions) WithForkSession(fork bool) *ClaudeAgentOptions {
	o.ForkSession = fork
	return o
}

// WithModel sets the model to use.
func (o *ClaudeAgentOptions) WithModel(model string) *ClaudeAgentOptions {
	o.Model = &model
	return o
}

// WithBetas enables Anthropic API beta features.
func (o *ClaudeAgentOptions) WithBetas(betas ...SdkBeta) *ClaudeAgentOptions {
	o.Betas = betas
	return o
}

// WithFallbackModel sets the fallback model to use if primary model is unavailable.
func (o *ClaudeAgentOptions) WithFallbackModel(fallbackModel string) *ClaudeAgentOptions {
	o.FallbackModel = &fallbackModel
	return o
}

// WithMaxTurns sets the maximum number of turns.
func (o *ClaudeAgentOptions) WithMaxTurns(maxTurns int) *ClaudeAgentOptions {
	o.MaxTurns = &maxTurns
	return o
}

// WithMaxThinkingTokens sets the maximum tokens for extended thinking.
// This limits how many tokens Claude can use for internal reasoning before responding.
func (o *ClaudeAgentOptions) WithMaxThinkingTokens(maxTokens int) *ClaudeAgentOptions {
	o.MaxThinkingTokens = &maxTokens
	return o
}

// WithMaxBudgetUSD sets the maximum budget in USD for this query.
// This helps prevent unexpectedly high API costs by stopping execution when the limit is reached.
func (o *ClaudeAgentOptions) WithMaxBudgetUSD(maxBudget float64) *ClaudeAgentOptions {
	o.MaxBudgetUSD = &maxBudget
	return o
}

// WithBaseURL sets the custom Anthropic API base URL.
func (o *ClaudeAgentOptions) WithBaseURL(baseURL string) *ClaudeAgentOptions {
	o.BaseURL = &baseURL
	return o
}

// WithCWD sets the working directory.
func (o *ClaudeAgentOptions) WithCWD(cwd string) *ClaudeAgentOptions {
	o.CWD = &cwd
	return o
}

// WithCLIPath sets the CLI binary path.
func (o *ClaudeAgentOptions) WithCLIPath(cliPath string) *ClaudeAgentOptions {
	o.CLIPath = &cliPath
	return o
}

// WithSettings sets the settings file path.
func (o *ClaudeAgentOptions) WithSettings(settings string) *ClaudeAgentOptions {
	o.Settings = &settings
	return o
}

// WithSettingSources sets the setting sources to load.
func (o *ClaudeAgentOptions) WithSettingSources(sources ...SettingSource) *ClaudeAgentOptions {
	o.SettingSources = sources
	return o
}

// WithAddDirs sets the directories to add.
func (o *ClaudeAgentOptions) WithAddDirs(dirs ...string) *ClaudeAgentOptions {
	o.AddDirs = dirs
	return o
}

// WithEnv sets environment variables.
func (o *ClaudeAgentOptions) WithEnv(env map[string]string) *ClaudeAgentOptions {
	o.Env = env
	return o
}

// WithEnvVar sets a single environment variable.
func (o *ClaudeAgentOptions) WithEnvVar(key, value string) *ClaudeAgentOptions {
	if o.Env == nil {
		o.Env = make(map[string]string)
	}
	o.Env[key] = value
	return o
}

// WithExtraArgs sets extra CLI arguments.
func (o *ClaudeAgentOptions) WithExtraArgs(args map[string]*string) *ClaudeAgentOptions {
	o.ExtraArgs = args
	return o
}

// WithExtraArg sets a single extra CLI argument.
func (o *ClaudeAgentOptions) WithExtraArg(key string, value *string) *ClaudeAgentOptions {
	if o.ExtraArgs == nil {
		o.ExtraArgs = make(map[string]*string)
	}
	o.ExtraArgs[key] = value
	return o
}

// WithMaxBufferSize sets the maximum buffer size.
func (o *ClaudeAgentOptions) WithMaxBufferSize(size int) *ClaudeAgentOptions {
	o.MaxBufferSize = &size
	return o
}

// WithOutputFormat sets the output format for structured outputs.
func (o *ClaudeAgentOptions) WithOutputFormat(format map[string]interface{}) *ClaudeAgentOptions {
	o.OutputFormat = format
	return o
}

// WithJSONSchemaOutput sets output_format to a JSON schema for structured outputs.
func (o *ClaudeAgentOptions) WithJSONSchemaOutput(schema interface{}) *ClaudeAgentOptions {
	o.OutputFormat = map[string]interface{}{
		"type":   "json_schema",
		"schema": schema,
	}
	return o
}

// WithMessageChannelCapacity sets the capacity for message channels.
func (o *ClaudeAgentOptions) WithMessageChannelCapacity(capacity int) *ClaudeAgentOptions {
	o.MessageChannelCapacity = &capacity
	return o
}

// WithIncludePartialMessages sets whether to include partial messages.
func (o *ClaudeAgentOptions) WithIncludePartialMessages(include bool) *ClaudeAgentOptions {
	o.IncludePartialMessages = include
	return o
}

// WithUser sets the user identifier.
func (o *ClaudeAgentOptions) WithUser(user string) *ClaudeAgentOptions {
	o.User = &user
	return o
}

// WithAgents sets the agent definitions.
func (o *ClaudeAgentOptions) WithAgents(agents map[string]AgentDefinition) *ClaudeAgentOptions {
	o.Agents = agents
	return o
}

// WithAgent sets a single agent definition.
func (o *ClaudeAgentOptions) WithAgent(name string, agent AgentDefinition) *ClaudeAgentOptions {
	if o.Agents == nil {
		o.Agents = make(map[string]AgentDefinition)
	}
	o.Agents[name] = agent
	return o
}

// WithCanUseTool sets the tool permission callback.
func (o *ClaudeAgentOptions) WithCanUseTool(callback CanUseToolFunc) *ClaudeAgentOptions {
	o.CanUseTool = callback
	return o
}

// WithHooks sets the hook configurations.
func (o *ClaudeAgentOptions) WithHooks(hooks map[HookEvent][]HookMatcher) *ClaudeAgentOptions {
	o.Hooks = hooks
	return o
}

// WithHook adds a hook matcher for a specific event.
func (o *ClaudeAgentOptions) WithHook(event HookEvent, matcher HookMatcher) *ClaudeAgentOptions {
	if o.Hooks == nil {
		o.Hooks = make(map[HookEvent][]HookMatcher)
	}
	o.Hooks[event] = append(o.Hooks[event], matcher)
	return o
}

// WithToolHandler registers a tool execution handler for the given tool name.
// When Claude calls this tool, the SDK invokes the handler instead of letting the CLI execute it.
//
// Pass a non-nil handler for callback mode (handler is called directly).
// Pass nil for event-stream mode (ToolExecutionRequest emitted via ReceiveResponse,
// caller must respond via Client.SubmitToolResult).
func (o *ClaudeAgentOptions) WithToolHandler(toolName string, handler ToolHandlerFunc) *ClaudeAgentOptions {
	if o.ToolHandlers == nil {
		o.ToolHandlers = make(map[string]ToolHandlerFunc)
	}
	o.ToolHandlers[toolName] = handler
	return o
}

// WithToolHandlerTimeout sets the timeout for event-stream mode tool handlers.
// If the caller does not submit a result within this duration, the SDK returns
// a deny response with a timeout error message. Default is 5 minutes.
func (o *ClaudeAgentOptions) WithToolHandlerTimeout(d time.Duration) *ClaudeAgentOptions {
	o.ToolHandlerTimeout = &d
	return o
}

// WithStderr sets the stderr callback.
func (o *ClaudeAgentOptions) WithStderr(callback StderrCallbackFunc) *ClaudeAgentOptions {
	o.Stderr = callback
	return o
}

// WithVerbose enables or disables verbose debug logging.
func (o *ClaudeAgentOptions) WithVerbose(enabled bool) *ClaudeAgentOptions {
	o.Verbose = enabled
	return o
}

// WithDangerouslySkipPermissions bypasses all permission checks.
// This is DANGEROUS and should only be used in sandboxed environments.
// Requires AllowDangerouslySkipPermissions to be enabled first.
//
// Security Warning: This disables ALL safety checks. Only use in isolated environments
// with no internet access and no sensitive data.
func (o *ClaudeAgentOptions) WithDangerouslySkipPermissions(skip bool) *ClaudeAgentOptions {
	o.DangerouslySkipPermissions = skip
	return o
}

// WithAllowDangerouslySkipPermissions enables the option to bypass permissions.
// This must be set to true before DangerouslySkipPermissions can be used.
//
// This is the "safety switch" that allows the dangerous flag to work.
func (o *ClaudeAgentOptions) WithAllowDangerouslySkipPermissions(allow bool) *ClaudeAgentOptions {
	o.AllowDangerouslySkipPermissions = allow
	return o
}

// WithPlugins sets the plugin configurations.
func (o *ClaudeAgentOptions) WithPlugins(plugins []SdkPluginConfig) *ClaudeAgentOptions {
	o.Plugins = plugins
	return o
}

// WithPlugin adds a single plugin configuration.
func (o *ClaudeAgentOptions) WithPlugin(plugin SdkPluginConfig) *ClaudeAgentOptions {
	o.Plugins = append(o.Plugins, plugin)
	return o
}

// WithLocalPlugin adds a local plugin configuration.
func (o *ClaudeAgentOptions) WithLocalPlugin(path string) *ClaudeAgentOptions {
	plugin := SdkPluginConfig{
		Type: "local",
		Path: path,
	}
	o.Plugins = append(o.Plugins, plugin)
	return o
}

// WithEnableFileCheckpointing toggles file checkpointing support.
func (o *ClaudeAgentOptions) WithEnableFileCheckpointing(enabled bool) *ClaudeAgentOptions {
	o.EnableFileCheckpointing = enabled
	return o
}

// --- P0: MCP Structured Config ---

// ensureMcpServersMap initializes McpServers as a map if nil or not already a map.
func (o *ClaudeAgentOptions) ensureMcpServersMap() {
	if o.McpServers == nil {
		o.McpServers = make(map[string]interface{})
		return
	}
	if _, ok := o.McpServers.(map[string]interface{}); !ok {
		o.McpServers = make(map[string]interface{})
	}
}

// WithMcpStdioServer adds a named stdio MCP server with validation.
func (o *ClaudeAgentOptions) WithMcpStdioServer(name string, config McpStdioServerConfig) *ClaudeAgentOptions {
	if name == "" {
		panic("claude-agent-sdk: WithMcpStdioServer: name must not be empty")
	}
	if config.Command == "" {
		panic("claude-agent-sdk: WithMcpStdioServer: command must not be empty")
	}
	if config.Type == nil {
		t := "stdio"
		config.Type = &t
	}
	o.ensureMcpServersMap()
	o.McpServers.(map[string]interface{})[name] = config
	return o
}

// WithMcpHTTPServer adds a named HTTP MCP server with URL validation.
func (o *ClaudeAgentOptions) WithMcpHTTPServer(name string, config McpHTTPServerConfig) *ClaudeAgentOptions {
	if name == "" {
		panic("claude-agent-sdk: WithMcpHTTPServer: name must not be empty")
	}
	if config.URL == "" {
		panic("claude-agent-sdk: WithMcpHTTPServer: URL must not be empty")
	}
	if _, err := url.Parse(config.URL); err != nil {
		panic(fmt.Sprintf("claude-agent-sdk: WithMcpHTTPServer: invalid URL %q: %v", config.URL, err))
	}
	if config.Type == "" {
		config.Type = "http"
	}
	o.ensureMcpServersMap()
	o.McpServers.(map[string]interface{})[name] = config
	return o
}

// WithMcpStreamableHTTPServer adds a named Streamable HTTP MCP server.
func (o *ClaudeAgentOptions) WithMcpStreamableHTTPServer(name string, config McpStreamableHTTPServerConfig) *ClaudeAgentOptions {
	if name == "" {
		panic("claude-agent-sdk: WithMcpStreamableHTTPServer: name must not be empty")
	}
	if config.URL == "" {
		panic("claude-agent-sdk: WithMcpStreamableHTTPServer: URL must not be empty")
	}
	if config.Type == "" {
		config.Type = "streamable-http"
	}
	o.ensureMcpServersMap()
	o.McpServers.(map[string]interface{})[name] = config
	return o
}

// WithMcpSSEServer adds a named SSE MCP server with URL validation.
func (o *ClaudeAgentOptions) WithMcpSSEServer(name string, config McpSSEServerConfig) *ClaudeAgentOptions {
	if name == "" {
		panic("claude-agent-sdk: WithMcpSSEServer: name must not be empty")
	}
	if config.URL == "" {
		panic("claude-agent-sdk: WithMcpSSEServer: URL must not be empty")
	}
	if config.Type == "" {
		config.Type = "sse"
	}
	o.ensureMcpServersMap()
	o.McpServers.(map[string]interface{})[name] = config
	return o
}

// WithMcpSdkServer adds a named SDK (in-process) MCP server.
func (o *ClaudeAgentOptions) WithMcpSdkServer(name string, server *ToolServerConfig) *ClaudeAgentOptions {
	if name == "" {
		panic("claude-agent-sdk: WithMcpSdkServer: name must not be empty")
	}
	o.ensureMcpServersMap()
	o.McpServers.(map[string]interface{})[name] = server
	return o
}

// --- P0: CLI Flags ---

// WithBareMode enables bare mode (minimal output, no status messages).
func (o *ClaudeAgentOptions) WithBareMode() *ClaudeAgentOptions {
	o.BareMode = true
	return o
}

// WithNoMarkdown disables markdown rendering in Claude's output.
func (o *ClaudeAgentOptions) WithNoMarkdown() *ClaudeAgentOptions {
	o.NoMarkdown = true
	return o
}

// WithSettingsOverride provides runtime settings overrides as a JSON-compatible map.
func (o *ClaudeAgentOptions) WithSettingsOverride(overrides map[string]interface{}) *ClaudeAgentOptions {
	o.SettingsOverride = overrides
	return o
}

// --- P1: Event Callbacks ---

// WithOnToolEvent sets a callback for tool use events.
func (o *ClaudeAgentOptions) WithOnToolEvent(handler ToolEventHandler) *ClaudeAgentOptions {
	o.OnToolEvent = handler
	return o
}

// WithOnProgress sets a callback for progress updates.
func (o *ClaudeAgentOptions) WithOnProgress(handler ProgressHandler) *ClaudeAgentOptions {
	o.OnProgress = handler
	return o
}

// --- P1: Structured Logging ---

// WithLogger sets a custom slog.Logger for structured logging. Overrides Verbose.
func (o *ClaudeAgentOptions) WithLogger(logger *slog.Logger) *ClaudeAgentOptions {
	o.Logger = logger
	return o
}

// WithLogLevel sets the log level for the default stderr logger.
func (o *ClaudeAgentOptions) WithLogLevel(level slog.Level) *ClaudeAgentOptions {
	o.LogLevel = &level
	return o
}

// --- P2: Retry ---

// WithRetry enables automatic retry with the given configuration.
func (o *ClaudeAgentOptions) WithRetry(config *RetryConfig) *ClaudeAgentOptions {
	o.RetryConfig = config
	return o
}

// --- P2: Cost Guard ---

// WithCostLimit sets a client-side cost limit in USD.
func (o *ClaudeAgentOptions) WithCostLimit(maxUSD float64, callback func(spent float64)) *ClaudeAgentOptions {
	o.CostLimitUSD = &maxUSD
	o.OnCostLimitExceed = callback
	return o
}

// --- P3: Auth ---

// WithAuthProvider sets the authentication provider.
func (o *ClaudeAgentOptions) WithAuthProvider(provider AuthProvider) *ClaudeAgentOptions {
	o.AuthProvider = provider
	return o
}
