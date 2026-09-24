package transcript

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// NormalizeTranscriptPath makes copied shell paths and file:// URLs usable by
// every command without requiring the caller to pre-normalize them.
func NormalizeTranscriptPath(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(strings.ToLower(value), "file://") {
		if parsed, err := url.Parse(value); err == nil {
			path := parsed.Path
			if parsed.Host != "" && !strings.EqualFold(parsed.Host, "localhost") {
				path = "//" + parsed.Host + path
			}
			if unescaped, err := url.PathUnescape(path); err == nil {
				path = unescaped
			}
			if runtime.GOOS == "windows" && len(path) >= 3 && path[0] == '/' && path[2] == ':' {
				path = path[1:]
			}
			value = filepath.FromSlash(path)
		}
	}
	value = expandHome(value)
	if value == "" {
		return value
	}
	return filepath.Clean(value)
}

func IsSessionID(value string) bool {
	return codexSessionIDPattern.MatchString(strings.TrimSpace(value))
}

func LooksLikeTranscriptReference(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	return strings.HasSuffix(lower, ".jsonl") || strings.HasPrefix(lower, "file://") || IsSessionID(trimmed)
}

// ResolveSessionReference accepts an existing transcript path, a file:// URL,
// or a full UUID-like Claude/Codex session ID. Session IDs are resolved from
// the cached default/custom transcript catalog.
func ResolveSessionReference(value string, roots []string) (string, error) {
	path := NormalizeTranscriptPath(value)
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		return path, nil
	}

	token := strings.ToLower(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
	if !IsSessionID(token) {
		return "", fmt.Errorf("transcript %q was not found", value)
	}

	find := func(refresh bool) ([]string, error) {
		sessions, err := DiscoverCached(0, ProviderUnknown, roots, refresh)
		if err != nil {
			return nil, err
		}
		var matches []string
		for _, session := range sessions {
			base := strings.ToLower(strings.TrimSuffix(filepath.Base(session.Path), filepath.Ext(session.Path)))
			if base == token || strings.HasSuffix(base, "-"+token) || strings.EqualFold(session.ID, token) {
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
		matches, err = find(true)
		if err != nil {
			return "", err
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("session %s was not found in the configured transcript roots", token)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("session %s is ambiguous; found %d transcripts: %s", token, len(matches), strings.Join(matches, ", "))
	}
}
