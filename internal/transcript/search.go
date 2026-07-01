package transcript

import (
	"fmt"
	"strings"
)

type Match struct {
	Session Session
	Block   Block
	Snippet string
}

func Search(query string, sessions []Session, opts RenderOptions) ([]Match, error) {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}
	var matches []Match
	for _, s := range sessions {
		tr, err := ParseFile(s.Path)
		if err != nil {
			continue
		}
		for _, b := range FilterBlocks(tr.Blocks, opts) {
			hay := strings.ToLower(strings.Join([]string{string(b.Kind), b.ToolName, b.Text, toolInputText(b)}, "\n"))
			if strings.Contains(hay, q) {
				session := s
				session.Provider = tr.Provider
				matches = append(matches, Match{Session: session, Block: b, Snippet: snippet(blockBodyText(b), query)})
			}
		}
	}
	return matches, nil
}

func snippet(text, query string) string {
	flat := strings.Join(strings.Fields(strings.ReplaceAll(text, "\n", " ")), " ")
	if flat == "" {
		return ""
	}
	lower := strings.ToLower(flat)
	idx := strings.Index(lower, strings.ToLower(query))
	if idx < 0 {
		if len(flat) > 140 {
			return flat[:137] + "..."
		}
		return flat
	}
	start := idx - 50
	if start < 0 {
		start = 0
	}
	end := idx + len(query) + 80
	if end > len(flat) {
		end = len(flat)
	}
	prefix := ""
	if start > 0 {
		prefix = "..."
	}
	suffix := ""
	if end < len(flat) {
		suffix = "..."
	}
	return prefix + flat[start:end] + suffix
}
