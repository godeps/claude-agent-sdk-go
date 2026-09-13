package types

// Compaction / context-observation types mirroring @anthropic-ai/claude-agent-sdk
// (TS SDK). Wire shapes follow sdk.d.ts exactly: SDKCompactBoundaryMessage,
// SDKContextUsage, SDKControlGetContextUsageResponse, SDKControlInitializeResponse.

// System message subtypes related to compaction.
const (
	SystemSubtypeCompactBoundary = "compact_boundary"
	SystemSubtypeCommandsChanged = "commands_changed"
)

// CompactTrigger describes what caused a compaction.
type CompactTrigger string

const (
	CompactTriggerManual CompactTrigger = "manual"
	CompactTriggerAuto   CompactTrigger = "auto"
)

// CompactPreservedSegment is relink info for messagesToKeep: loaders splice
// the preserved segment at anchor_uuid so resume includes preserved content.
// Nil when compaction summarized everything.
type CompactPreservedSegment struct {
	HeadUUID   string `json:"head_uuid"`
	AnchorUUID string `json:"anchor_uuid"`
	TailUUID   string `json:"tail_uuid"`
}

// CompactPreservedMessages lists ordered messagesToKeep UUIDs. Supersedes
// PreservedSegment — readers look up each UUID directly and relink uuids[i]
// to uuids[i-1] (uuids[0] to anchor_uuid). Nil when everything was summarized.
type CompactPreservedMessages struct {
	AnchorUUID string   `json:"anchor_uuid"`
	UUIDs      []string `json:"uuids"`
}

// CompactBoundaryMetadata is the compact_metadata payload of a
// system/compact_boundary message. The CLI emits this message when it
// compacts the conversation (auto-compact at the window threshold, or a
// manual /compact). Hosts watch it to learn the conversation summary
// boundary and token delta without parsing transcript files.
type CompactBoundaryMetadata struct {
	// Trigger is "auto" (window threshold) or "manual" (/compact).
	Trigger CompactTrigger `json:"trigger"`
	// PreTokens is the context size before compaction.
	PreTokens int `json:"pre_tokens"`
	// PostTokens is the context size after compaction (absent on older CLIs).
	PostTokens *int `json:"post_tokens,omitempty"`
	// DurationMS is how long compaction took (absent on older CLIs).
	DurationMS *int64 `json:"duration_ms,omitempty"`
	// PreservedSegment is relink info for messagesToKeep, when present.
	PreservedSegment *CompactPreservedSegment `json:"preserved_segment,omitempty"`
	// PreservedMessages lists kept message UUIDs; supersedes PreservedSegment.
	PreservedMessages *CompactPreservedMessages `json:"preserved_messages,omitempty"`
}

// TokensSaved returns pre−post tokens (0 when PostTokens is unavailable).
func (m *CompactBoundaryMetadata) TokensSaved() int {
	if m == nil || m.PostTokens == nil {
		return 0
	}
	saved := m.PreTokens - *m.PostTokens
	if saved < 0 {
		return 0
	}
	return saved
}

// ContextUsageCategoryKind classifies a /context usage row. Classify on this,
// never on the display Name.
type ContextUsageCategoryKind string

const (
	// ContextUsageKindUsed content occupies the window.
	ContextUsageKindUsed ContextUsageCategoryKind = "used"
	// ContextUsageKindFree is the remaining window.
	ContextUsageKindFree ContextUsageCategoryKind = "free"
	// ContextUsageKindBuffer is the compaction reserve.
	ContextUsageKindBuffer ContextUsageCategoryKind = "buffer"
	// ContextUsageKindDeferred rows are out-of-window tool schemas — listed
	// for awareness, excluded from usage math.
	ContextUsageKindDeferred ContextUsageCategoryKind = "deferred"
)

// ContextUsageCategory is one row of the usage-by-category breakdown.
type ContextUsageCategory struct {
	Name   string                   `json:"name"`
	Tokens int                      `json:"tokens"`
	Kind   ContextUsageCategoryKind `json:"kind"`
}

// ContextUsageOverLimit is present when total_tokens exceeds raw_max_tokens.
// Kind says how the window was resolved, not whether the API will accept the
// next request: hard_limit means the model's believed limit (API will refuse
// past it); compaction_window means a compaction-policy window.
type ContextUsageOverLimit struct {
	TokensOver int    `json:"tokens_over"`
	Kind       string `json:"kind"` // "hard_limit" | "compaction_window"
}

// ContextUsageMCPTool is the per-MCP-tool token cost. Name is the wire name,
// e.g. "mcp__linear__create_issue".
type ContextUsageMCPTool struct {
	Name       string `json:"name"`
	ServerName string `json:"server_name"`
	Tokens     int    `json:"tokens"`
}

// ContextUsageMemoryFile is a memory file (CLAUDE.md etc.) token cost.
type ContextUsageMemoryFile struct {
	Path   string `json:"path"`
	Type   string `json:"type"` // display label, e.g. "Project" or "User"
	Tokens int    `json:"tokens"`
}

// ContextUsageAgent is a per-agent-definition token cost. Source is the raw
// source identifier, e.g. 'projectSettings', 'userSettings', 'plugin'.
type ContextUsageAgent struct {
	AgentType string `json:"agent_type"`
	Source    string `json:"source"`
	Tokens    int    `json:"tokens"`
}

// ContextUsageSkill is a per-skill token cost (omitted when none contribute).
type ContextUsageSkill struct {
	Name       string `json:"name"`
	Source     string `json:"source"`
	PluginName string `json:"plugin_name,omitempty"`
	Tokens     int    `json:"tokens"`
}

// ContextUsage is the structured twin of the /context report, carried on the
// synthetic assistant message that delivers the markdown table (wrapper-level
// sibling of message.content — never inside content, not replayed to the
// model). Evolves additively; consumers can trust the fields they know.
//
// Wire shape: snake_case (SDKContextUsage in sdk.d.ts).
type ContextUsage struct {
	// Model the usage was computed for.
	Model string `json:"model"`
	// TotalTokens is the estimated tokens in use, unclamped — may exceed
	// RawMaxTokens when over limit.
	TotalTokens int `json:"total_tokens"`
	// RawMaxTokens is the window usage is measured against: the resolved
	// autocompact window (model limit or a smaller compaction-policy window).
	RawMaxTokens int `json:"raw_max_tokens"`
	// Percentage is rounded total/raw, 0-100+.
	Percentage int `json:"percentage"`
	// OverLimit is present when TotalTokens exceeds RawMaxTokens.
	OverLimit *ContextUsageOverLimit `json:"over_limit,omitempty"`

	Categories  []ContextUsageCategory   `json:"categories"`
	MCPTools    []ContextUsageMCPTool    `json:"mcp_tools"`
	MemoryFiles []ContextUsageMemoryFile `json:"memory_files"`
	Agents      []ContextUsageAgent      `json:"agents"`
	Skills      []ContextUsageSkill      `json:"skills,omitempty"`
}

// ContextUsageDetail controls how the CLI computes a get_context_usage answer.
type ContextUsageDetail string

const (
	// ContextUsageDetailFull counts each category with the token-count API
	// (default).
	ContextUsageDetailFull ContextUsageDetail = "full"
	// ContextUsageDetailSummary answers from the last response's usage and
	// local estimates without per-category token-count calls.
	ContextUsageDetailSummary ContextUsageDetail = "summary"
)

// ContextUsageReport is the get_context_usage control response
// (SDKControlGetContextUsageResponse). Wire shape: camelCase — this is a
// different producer from ContextUsage (the /context assistant-message twin);
// do not confuse the two.
type ContextUsageReport struct {
	Categories []struct {
		Name       string                   `json:"name"`
		Tokens     int                      `json:"tokens"`
		Color      string                   `json:"color"`
		IsDeferred bool                     `json:"isDeferred,omitempty"`
		Kind       ContextUsageCategoryKind `json:"kind"`
	} `json:"categories"`
	TotalTokens  int    `json:"totalTokens"`
	MaxTokens    int    `json:"maxTokens"`
	RawMaxTokens int    `json:"rawMaxTokens"`
	Percentage   int    `json:"percentage"`
	Model        string `json:"model"`
	MemoryFiles  []struct {
		Path   string `json:"path"`
		Type   string `json:"type"`
		Tokens int    `json:"tokens"`
	} `json:"memoryFiles"`
	MCPTools []struct {
		Name       string `json:"name"`
		ServerName string `json:"serverName"`
		Tokens     int    `json:"tokens"`
		IsLoaded   *bool  `json:"isLoaded,omitempty"`
	} `json:"mcpTools"`
	DeferredBuiltinTools []struct {
		Name     string `json:"name"`
		Tokens   int    `json:"tokens"`
		IsLoaded bool   `json:"isLoaded"`
	} `json:"deferredBuiltinTools,omitempty"`
	SystemTools []struct {
		Name   string `json:"name"`
		Tokens int    `json:"tokens"`
	} `json:"systemTools,omitempty"`
	SystemPromptSections []struct {
		Name   string `json:"name"`
		Tokens int    `json:"tokens"`
	} `json:"systemPromptSections,omitempty"`
	Agents []struct {
		AgentType string `json:"agentType"`
		Source    string `json:"source"`
		Tokens    int    `json:"tokens"`
	} `json:"agents"`
	SlashCommands *struct {
		TotalCommands    int `json:"totalCommands"`
		IncludedCommands int `json:"includedCommands"`
		Tokens           int `json:"tokens"`
	} `json:"slashCommands,omitempty"`
	Skills *struct {
		TotalSkills      int `json:"totalSkills"`
		IncludedSkills   int `json:"includedSkills"`
		Tokens           int `json:"tokens"`
		SkillFrontmatter []struct {
			Name   string `json:"name"`
			Source string `json:"source"`
			Tokens int    `json:"tokens"`
		} `json:"skillFrontmatter"`
	} `json:"skills,omitempty"`
	AutoCompactThreshold *int `json:"autoCompactThreshold,omitempty"`
	IsAutoCompactEnabled bool `json:"isAutoCompactEnabled"`
	MessageBreakdown     *struct {
		ToolCallTokens          int `json:"toolCallTokens"`
		ToolResultTokens        int `json:"toolResultTokens"`
		AttachmentTokens        int `json:"attachmentTokens"`
		AssistantMessageTokens  int `json:"assistantMessageTokens"`
		UserMessageTokens       int `json:"userMessageTokens"`
		RedirectedContextTokens int `json:"redirectedContextTokens"`
		UnattributedTokens      int `json:"unattributedTokens"`
		ToolCallsByType         []struct {
			Name         string `json:"name"`
			CallTokens   int    `json:"callTokens"`
			ResultTokens int    `json:"resultTokens"`
		} `json:"toolCallsByType"`
		AttachmentsByType []struct {
			Name   string `json:"name"`
			Tokens int    `json:"tokens"`
		} `json:"attachmentsByType"`
	} `json:"messageBreakdown,omitempty"`
	APIUsage *struct {
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	} `json:"apiUsage"`
}

// Headroom returns the remaining window tokens (0 when at/over the limit).
func (r *ContextUsageReport) Headroom() int {
	if r == nil {
		return 0
	}
	h := r.RawMaxTokens - r.TotalTokens
	if h < 0 {
		return 0
	}
	return h
}

// ModelInfo describes a model the CLI can use (initialize response "models").
type ModelInfo struct {
	Value       string `json:"value"`
	DisplayName string `json:"displayName"`
	Description string `json:"description,omitempty"`
}

// SlashCommand describes an available slash command (initialize "commands").
type SlashCommand struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// Input may carry hint/required info depending on CLI version.
	Input interface{} `json:"input,omitempty"`
	Kind  string      `json:"kind,omitempty"`
}

// AgentInfo describes an available agent definition (initialize "agents").
type AgentInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName,omitempty"`
	Description string `json:"description,omitempty"`
}

// InitializeResult is the typed SDKControlInitializeResponse.
type InitializeResult struct {
	Commands              []SlashCommand `json:"commands"`
	Agents                []AgentInfo    `json:"agents"`
	OutputStyle           string         `json:"output_style"`
	AvailableOutputStyles []string       `json:"available_output_styles"`
	Models                []ModelInfo    `json:"models"`
	HooksApplied          *bool          `json:"hooks_applied,omitempty"`
	PluginsApplied        *bool          `json:"plugins_applied,omitempty"`
}
