package transcript

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var codexSessionIDPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func Discover(limit int, provider Provider, roots []string) ([]Session, error) {
	sources := SourcesFromRoots(roots, provider)
	return DiscoverSources(limit, provider, sources)
}

func DiscoverSources(limit int, provider Provider, sources []SessionSource) ([]Session, error) {
	if len(sources) == 0 {
		sources = DefaultSources()
	}
	var sessions []Session
	for _, source := range sources {
		root := expandHome(source.Path)
		if absolute, err := filepath.Abs(root); err == nil {
			root = absolute
		}
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			continue
		}
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if !strings.EqualFold(filepath.Ext(path), ".jsonl") {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			p := source.Provider
			if p == ProviderUnknown || p == "" {
				p = providerFromPath(path)
				if p == ProviderUnknown {
					p = detectProviderFile(path)
				}
			}
			if provider != "" && provider != ProviderUnknown && p != provider {
				return nil
			}
			sessions = append(sessions, Session{Path: path, Provider: p, ModTime: info.ModTime(), Size: info.Size(), Title: titleFromPath(path)})
			return nil
		})
	}
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].ModTime.After(sessions[j].ModTime) })
	if limit > 0 && len(sessions) > limit {
		sessions = sessions[:limit]
	}
	return sessions, nil
}

func DefaultSources() []SessionSource {
	return []SessionSource{
		{Path: configSubdir("CLAUDE_CONFIG_DIR", "projects", "~/.claude/projects"), Provider: ProviderClaude},
		{Path: configSubdir("CODEX_HOME", "sessions", "~/.codex/sessions"), Provider: ProviderCodex},
	}
}

func SourcesFromRoots(roots []string, provider Provider) []SessionSource {
	if len(roots) == 0 {
		return DefaultSources()
	}
	p := provider
	if p == "" {
		p = ProviderUnknown
	}
	out := make([]SessionSource, 0, len(roots))
	for _, root := range roots {
		if strings.TrimSpace(root) == "" {
			continue
		}
		out = append(out, SessionSource{Path: root, Provider: p})
	}
	return out
}

func DefaultRoots() []string {
	sources := DefaultSources()
	out := make([]string, 0, len(sources))
	for _, source := range sources {
		out = append(out, source.Path)
	}
	return out
}

func DefaultClaudeRoots() []string {
	return []string{DefaultSources()[0].Path}
}

func DefaultCodexRoots() []string {
	return []string{DefaultSources()[1].Path}
}

func providerFromPath(path string) Provider {
	normalized := filepath.ToSlash(strings.ToLower(path))
	if strings.Contains(normalized, "/.claude/") || strings.Contains(normalized, "/claude/projects/") {
		return ProviderClaude
	}
	if strings.Contains(normalized, "/.codex/") || strings.Contains(normalized, "/codex/sessions/") {
		return ProviderCodex
	}
	return ProviderUnknown
}

func detectProviderFile(path string) Provider {
	f, err := os.Open(path)
	if err != nil {
		return ProviderUnknown
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for lines := 0; scanner.Scan() && lines < 80; lines++ {
		var env struct {
			Type string          `json:"type"`
			Item json.RawMessage `json:"item"`
		}
		if json.Unmarshal(scanner.Bytes(), &env) != nil {
			continue
		}
		switch env.Type {
		case "session_meta", "thread.started":
			return ProviderCodex
		case "item.completed":
			if len(env.Item) > 0 {
				return ProviderCodex
			}
		case "user", "assistant":
			return ProviderClaude
		}
	}
	return ProviderUnknown
}

func HydrateSession(session Session) Session {
	f, err := os.Open(session.Path)
	if err != nil {
		return session
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 32*1024*1024)
	for lines := 0; scanner.Scan() && lines < 40; lines++ {
		line := scanner.Bytes()
		var env struct {
			Type      string          `json:"type"`
			Timestamp string          `json:"timestamp"`
			SessionID string          `json:"sessionId"`
			CWD       string          `json:"cwd"`
			Message   *message        `json:"message"`
			Payload   json.RawMessage `json:"payload"`
		}
		if json.Unmarshal(line, &env) != nil {
			continue
		}
		if session.ID == "" && env.SessionID != "" {
			session.ID = env.SessionID
		}
		if session.CWD == "" && env.CWD != "" {
			session.CWD = env.CWD
		}
		if session.StartedAt.IsZero() && env.Timestamp != "" {
			if ts, err := time.Parse(time.RFC3339Nano, env.Timestamp); err == nil {
				session.StartedAt = ts
			}
		}
		if len(env.Payload) > 0 {
			var payload struct {
				ID        string          `json:"id"`
				CWD       string          `json:"cwd"`
				Timestamp string          `json:"timestamp"`
				Type      string          `json:"type"`
				Message   string          `json:"message"`
				Role      string          `json:"role"`
				Content   json.RawMessage `json:"content"`
			}
			if json.Unmarshal(env.Payload, &payload) == nil {
				if session.ID == "" {
					session.ID = payload.ID
				}
				if session.CWD == "" {
					session.CWD = payload.CWD
				}
				if session.StartedAt.IsZero() && payload.Timestamp != "" {
					if ts, err := time.Parse(time.RFC3339Nano, payload.Timestamp); err == nil {
						session.StartedAt = ts
					}
				}
				if session.FirstPrompt == "" && payload.Type == "user_message" {
					session.FirstPrompt = cleanPrompt(payload.Message)
				}
				if session.FirstPrompt == "" && payload.Type == "message" && payload.Role == "user" {
					session.FirstPrompt = cleanPrompt(messageText(payload.Content))
				}
			}
		}
		if session.FirstPrompt == "" && env.Type == "user" && env.Message != nil {
			session.FirstPrompt = cleanPrompt(messageText(env.Message.Content))
		}
		if session.ID != "" && session.CWD != "" && session.FirstPrompt != "" {
			break
		}
	}
	return finalizeSessionMetadata(session)
}

func HydrateSessionFromTranscript(session Session, tr Transcript) Session {
	session = HydrateSession(session)
	if session.FirstPrompt == "" {
		for _, block := range tr.Blocks {
			if block.Kind != KindUser {
				continue
			}
			if prompt := cleanPrompt(block.Text); prompt != "" {
				session.FirstPrompt = prompt
				break
			}
		}
	}
	if session.StartedAt.IsZero() {
		for _, block := range tr.Blocks {
			if block.Timestamp == "" {
				continue
			}
			if ts, err := time.Parse(time.RFC3339Nano, block.Timestamp); err == nil {
				session.StartedAt = ts
				break
			}
		}
	}
	return finalizeSessionMetadata(session)
}

func finalizeSessionMetadata(session Session) Session {
	if isInjectedAgentBootstrap(session.FirstPrompt) {
		session.FirstPrompt = ""
	}
	session.Auxiliary = auxiliarySessionKind(session.FirstPrompt)
	if session.CWD != "" {
		session.Project = filepath.Base(filepath.Clean(session.CWD))
	}
	if session.Project != "" && session.Auxiliary != "" {
		session.Title = session.Project + " — " + session.Auxiliary
	} else if session.Project != "" && session.FirstPrompt != "" {
		session.Title = session.Project + " — " + truncateInline(session.FirstPrompt, 72)
	} else if session.Project != "" {
		session.Title = session.Project
	}
	return session
}

func auxiliarySessionKind(prompt string) string {
	trimmed := strings.TrimSpace(prompt)
	switch {
	case strings.HasPrefix(trimmed, "You are helping with git commits."):
		return "git commit helper"
	default:
		return ""
	}
}

func messageText(content json.RawMessage) string {
	if isJSONString(content) {
		return readJSONString(content)
	}
	var parts []string
	for _, cb := range decodeContentArray(content) {
		if cb.Type == "text" && strings.TrimSpace(cb.Text) != "" {
			parts = append(parts, cb.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func cleanPrompt(text string) string {
	text = stripInternalContextBlocks(text)
	text = stripInternalGoalBlocks(text)
	if isInjectedAgentsInstructions(text) || isInjectedAgentBootstrap(text) {
		return ""
	}
	return strings.TrimSpace(text)
}

func isInjectedAgentBootstrap(text string) bool {
	trimmed := strings.TrimSpace(text)
	return strings.HasPrefix(trimmed, "You were added to the workspace team ") ||
		(strings.HasPrefix(trimmed, "Your name is ") && strings.Contains(trimmed, "You are now assigned"))
}

func truncateInline(text string, max int) string {
	text = strings.Join(strings.Fields(text), " ")
	if len(text) <= max {
		return text
	}
	if max <= 1 {
		return text[:max]
	}
	return text[:max-1] + "…"
}

func cacheBaseDir() string {
	if v := strings.TrimSpace(os.Getenv("AGENTSCRIPT_CACHE_DIR")); v != "" {
		return expandHome(v)
	}
	if dir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(dir, "agentscript")
	}
	return expandHome("~/.cache/agentscript")
}

func cacheKey(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:16])
}

func IsCodexSessionID(value string) bool {
	return codexSessionIDPattern.MatchString(value)
}

func ResolveCodexSessionID(sessionID string, roots []string) (string, error) {
	if !IsCodexSessionID(sessionID) {
		return "", fmt.Errorf("invalid Codex session ID %q", sessionID)
	}
	sessionID = strings.ToLower(sessionID)
	find := func(refresh bool) ([]string, error) {
		sessions, err := DiscoverCached(0, ProviderCodex, roots, refresh)
		if err != nil {
			return nil, err
		}
		var matches []string
		for _, session := range sessions {
			base := strings.ToLower(strings.TrimSuffix(filepath.Base(session.Path), filepath.Ext(session.Path)))
			if base == sessionID || strings.HasSuffix(base, "-"+sessionID) {
				matches = append(matches, session.Path)
			}
		}
		sort.Strings(matches)
		return matches, nil
	}
	matches, err := find(false)
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		// A just-created session may not yet be present in the short-lived catalog.
		matches, err = find(true)
		if err != nil {
			return "", err
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("Codex session %s was not found in the configured roots", sessionID)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("Codex session %s is ambiguous; found %d matching transcripts: %s", sessionID, len(matches), strings.Join(matches, ", "))
	}
}

func configSubdir(envName, subdir, fallback string) string {
	configDir := strings.TrimSpace(os.Getenv(envName))
	if configDir == "" {
		return fallback
	}
	root := filepath.Join(expandHome(configDir), subdir)
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return fallback
	}
	return root
}

func expandHome(path string) string {
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func titleFromPath(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	parent := filepath.Base(filepath.Dir(path))
	if parent != "." && parent != "" {
		return fmt.Sprintf("%s / %s", parent, base)
	}
	return base
}
