package transcript

import (
	"encoding/json"
	"time"
)

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
	Turn      int             `json:"turn,omitempty"`
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

type SessionSource struct {
	Path     string   `json:"path"`
	Provider Provider `json:"provider"`
}

type Session struct {
	ID          string    `json:"id,omitempty"`
	Path        string    `json:"path"`
	Provider    Provider  `json:"provider"`
	Project     string    `json:"project,omitempty"`
	CWD         string    `json:"cwd,omitempty"`
	ModTime     time.Time `json:"mod_time"`
	StartedAt   time.Time `json:"started_at,omitempty"`
	Size        int64     `json:"size"`
	Title       string    `json:"title"`
	FirstPrompt string    `json:"first_prompt,omitempty"`
	Auxiliary   string    `json:"auxiliary,omitempty"`
}

type RenderOptions struct {
	Format           string
	HideThinking     bool
	HideTools        bool
	HideToolResults  bool
	HideCommands     bool
	MessagesOnly     bool
	ShowTimestamps   bool
	ShowRawInput     bool
	ShowTurns        bool
	ShowInternalGoal bool
	ShowInternal     bool
	OnlyTools        []string
	HideToolNames    []string
	OnlyKinds        []Kind
	HideKinds        []Kind
	MaxLines         int
	ExpandIndexes    []int
	ExpandAll        bool
	MarkdownStyle    string
}

func (o RenderOptions) normalizedFormat() string {
	if o.Format == "" {
		return "text"
	}
	return o.Format
}
