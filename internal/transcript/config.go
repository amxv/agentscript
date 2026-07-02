package transcript

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	DefaultProfile string                   `json:"default_profile,omitempty"`
	Profiles       map[string]ProfileConfig `json:"profiles,omitempty"`
}

type ProfileConfig struct {
	HideThinking    *bool    `json:"hide_thinking,omitempty"`
	HideTools       *bool    `json:"hide_tools,omitempty"`
	HideToolResults *bool    `json:"hide_tool_results,omitempty"`
	HideCommands    *bool    `json:"hide_commands,omitempty"`
	MessagesOnly    *bool    `json:"messages_only,omitempty"`
	ShowTurns       *bool    `json:"show_turns,omitempty"`
	MaxLines        *int     `json:"max_lines,omitempty"`
	Kinds           []string `json:"kinds,omitempty"`
	HideKinds       []string `json:"hide_kinds,omitempty"`
	Tools           []string `json:"tools,omitempty"`
	HideToolsNamed  []string `json:"hide_tools_named,omitempty"`
	MarkdownStyle   string   `json:"markdown_style,omitempty"`
	Format          string   `json:"format,omitempty"`
}

func DefaultConfig() Config {
	bTrue := true
	bFalse := false
	max40 := 40
	max120 := 120
	return Config{Profiles: map[string]ProfileConfig{
		"full":     {HideThinking: &bFalse, HideTools: &bFalse, HideToolResults: &bFalse, HideCommands: &bFalse},
		"compact":  {HideThinking: &bTrue, HideToolResults: &bTrue, MaxLines: &max40},
		"messages": {MessagesOnly: &bTrue},
		"handoff":  {HideThinking: &bTrue, HideToolResults: &bTrue, ShowTurns: &bTrue, MarkdownStyle: "llm-context", Format: "md"},
		"debug":    {ShowTurns: &bTrue, MaxLines: &max120},
		"commands": {Kinds: []string{"command", "command_result"}, MaxLines: &max120},
		"tools":    {Kinds: []string{"tool_call", "tool_result", "command", "command_result"}, MaxLines: &max120},
	}}
}

func ConfigPath() string {
	if v := strings.TrimSpace(os.Getenv("AGENTSCRIPT_CONFIG")); v != "" {
		return expandHome(v)
	}
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "agentscript", "config.json")
	}
	return expandHome("~/.agentscript/config.json")
}

func LoadConfig() (Config, error) {
	cfg := DefaultConfig()
	path := ConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	var user Config
	if err := json.Unmarshal(data, &user); err != nil {
		return cfg, err
	}
	if user.DefaultProfile != "" {
		cfg.DefaultProfile = user.DefaultProfile
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]ProfileConfig{}
	}
	for k, v := range user.Profiles {
		cfg.Profiles[k] = v
	}
	return cfg, nil
}

func WriteDefaultConfig(path string) error {
	if path == "" {
		path = ConfigPath()
	}
	cfg := DefaultConfig()
	cfg.DefaultProfile = "compact"
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func ApplyProfile(opts RenderOptions, cfg Config, profile string) (RenderOptions, error) {
	if profile == "" {
		profile = cfg.DefaultProfile
	}
	if profile == "" {
		return opts, nil
	}
	p, ok := cfg.Profiles[profile]
	if !ok {
		return opts, fmt.Errorf("unknown profile %q", profile)
	}
	if p.HideThinking != nil {
		opts.HideThinking = *p.HideThinking
	}
	if p.HideTools != nil {
		opts.HideTools = *p.HideTools
	}
	if p.HideToolResults != nil {
		opts.HideToolResults = *p.HideToolResults
	}
	if p.HideCommands != nil {
		opts.HideCommands = *p.HideCommands
	}
	if p.MessagesOnly != nil {
		opts.MessagesOnly = *p.MessagesOnly
	}
	if p.ShowTurns != nil {
		opts.ShowTurns = *p.ShowTurns
	}
	if p.MaxLines != nil {
		opts.MaxLines = *p.MaxLines
	}
	if p.MarkdownStyle != "" {
		opts.MarkdownStyle = p.MarkdownStyle
	}
	if p.Format != "" {
		opts.Format = p.Format
	}
	if len(p.Tools) > 0 {
		opts.OnlyTools = p.Tools
	}
	if len(p.HideToolsNamed) > 0 {
		opts.HideToolNames = p.HideToolsNamed
	}
	if len(p.Kinds) > 0 {
		opts.OnlyKinds = ParseKinds(p.Kinds)
	}
	if len(p.HideKinds) > 0 {
		opts.HideKinds = ParseKinds(p.HideKinds)
	}
	return opts, nil
}

func ParseKinds(vals []string) []Kind {
	var out []Kind
	for _, raw := range vals {
		for _, part := range strings.Split(raw, ",") {
			s := strings.ToLower(strings.TrimSpace(part))
			s = strings.ReplaceAll(s, "-", "_")
			switch s {
			case "user":
				out = append(out, KindUser)
			case "assistant":
				out = append(out, KindAssistant)
			case "thinking", "thought":
				out = append(out, KindThinking)
			case "tool", "tool_call", "toolcall":
				out = append(out, KindToolCall)
			case "tool_result", "toolresult", "result":
				out = append(out, KindToolResult)
			case "command", "cmd", "bash":
				out = append(out, KindCommand)
			case "command_result", "cmd_result", "bash_result":
				out = append(out, KindCommandResult)
			case "system":
				out = append(out, KindSystem)
			}
		}
	}
	return out
}
