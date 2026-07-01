package transcript

import "encoding/json"

type Provider string

const (
	ProviderUnknown Provider = "unknown"
	ProviderClaude  Provider = "claude"
	ProviderCodex   Provider = "codex"
)

type Kind string

const (
	KindUser          Kind = "user"
	KindAssistant     Kind = "assistant"
	KindThinking      Kind = "thinking"
	KindToolCall      Kind = "tool_call"
	KindToolResult    Kind = "tool_result"
	KindCommand       Kind = "command"
	KindCommandResult Kind = "command_result"
	KindSystem        Kind = "system"
)

type Block struct {
	Index     int             `json:"index"`
	Provider  Provider        `json:"provider"`
	Kind      Kind            `json:"kind"`
	Role      string          `json:"role,omitempty"`
	Text      string          `json:"text,omitempty"`
	ToolID    string          `json:"tool_id,omitempty"`
	ToolName  string          `json:"tool_name,omitempty"`
	ToolInput map[string]any  `json:"tool_input,omitempty"`
	Timestamp string          `json:"timestamp,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
	Raw       json.RawMessage `json:"-"`
}

type Transcript struct {
	Path     string   `json:"path,omitempty"`
	Provider Provider `json:"provider"`
	Blocks   []Block  `json:"blocks"`
}

type RenderOptions struct {
	Format          string
	HideThinking    bool
	HideTools       bool
	HideToolResults bool
	HideCommands    bool
	MessagesOnly    bool
	ShowTimestamps  bool
	ShowRawInput    bool
	OnlyTools       []string
	HideToolNames   []string
}

func (o RenderOptions) normalizedFormat() string {
	if o.Format == "" {
		return "text"
	}
	return o.Format
}
