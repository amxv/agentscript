package transcript

import (
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

type Session struct {
	Path     string
	Provider Provider
	ModTime  time.Time
	Size     int64
	Title    string
}

func Discover(limit int, provider Provider, roots []string) ([]Session, error) {
	if len(roots) == 0 {
		roots = DefaultRoots()
	}
	var sessions []Session
	for _, root := range roots {
		root = expandHome(root)
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
			p := ProviderUnknown
			if strings.Contains(path, string(os.PathSeparator)+".claude"+string(os.PathSeparator)) || strings.Contains(root, ".claude") {
				p = ProviderClaude
			} else if strings.Contains(path, string(os.PathSeparator)+".codex"+string(os.PathSeparator)) || strings.Contains(root, ".codex") {
				p = ProviderCodex
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

func DefaultRoots() []string {
	return append(DefaultClaudeRoots(), DefaultCodexRoots()...)
}

func DefaultClaudeRoots() []string {
	return []string{configSubdir("CLAUDE_CONFIG_DIR", "projects", "~/.claude/projects")}
}

func DefaultCodexRoots() []string {
	return []string{configSubdir("CODEX_HOME", "sessions", "~/.codex/sessions")}
}

func IsCodexSessionID(value string) bool {
	return codexSessionIDPattern.MatchString(value)
}

func ResolveCodexSessionID(sessionID string, roots []string) (string, error) {
	if !IsCodexSessionID(sessionID) {
		return "", fmt.Errorf("invalid Codex session ID %q", sessionID)
	}
	if len(roots) == 0 {
		roots = DefaultCodexRoots()
	}

	sessionID = strings.ToLower(sessionID)
	var matches []string
	for _, root := range roots {
		root = expandHome(root)
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			continue
		}
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".jsonl") {
				return nil
			}
			base := strings.ToLower(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
			if base == sessionID || strings.HasSuffix(base, "-"+sessionID) {
				matches = append(matches, path)
			}
			return nil
		})
	}

	sort.Strings(matches)
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
