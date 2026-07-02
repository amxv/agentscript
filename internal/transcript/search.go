package transcript

import (
	"fmt"
	"regexp"
	"strings"
)

type SearchMode string

const (
	SearchAny SearchMode = "any"
	SearchAll SearchMode = "all"
)

type SearchOptions struct {
	Queries       []string
	Mode          SearchMode
	Near          int
	Regex         bool
	CaseSensitive bool
	Kinds         []Kind
	Tools         []string
	RenderOptions RenderOptions
}

type Match struct {
	Session Session
	Block   Block
	Snippet string
}

func Search(query string, sessions []Session, opts RenderOptions) ([]Match, error) {
	return SearchAdvanced(sessions, SearchOptions{Queries: []string{query}, Mode: SearchAny, RenderOptions: opts})
}

func SearchAdvanced(sessions []Session, opts SearchOptions) ([]Match, error) {
	if len(opts.Queries) == 0 {
		return nil, fmt.Errorf("query cannot be empty")
	}
	if opts.Mode == "" {
		opts.Mode = SearchAny
	}
	var compiled []*regexp.Regexp
	if opts.Regex {
		for _, q := range opts.Queries {
			pattern := q
			if !opts.CaseSensitive {
				pattern = "(?i)" + pattern
			}
			re, err := regexp.Compile(pattern)
			if err != nil {
				return nil, err
			}
			compiled = append(compiled, re)
		}
	}
	var matches []Match
	for _, s := range sessions {
		tr, err := ParseFile(s.Path)
		if err != nil {
			continue
		}
		blocks := FilterBlocks(tr.Blocks, opts.RenderOptions)
		for _, b := range blocks {
			if len(opts.Kinds) > 0 && !kindInList(b.Kind, opts.Kinds) {
				continue
			}
			if len(opts.Tools) > 0 && !nameInList(b.ToolName, opts.Tools) {
				continue
			}
			if blockMatches(b, opts, compiled) {
				session := s
				session.Provider = tr.Provider
				matches = append(matches, Match{Session: session, Block: b, Snippet: snippet(blockBodyText(b), opts.Queries[0])})
			}
		}
		if opts.Near > 0 && len(opts.Queries) > 1 {
			near := nearMatches(s, tr, blocks, opts, compiled)
			matches = append(matches, near...)
		}
	}
	return dedupeMatches(matches), nil
}

func blockMatches(b Block, opts SearchOptions, compiled []*regexp.Regexp) bool {
	hay := strings.Join([]string{string(b.Kind), b.ToolName, b.Text, toolInputText(b)}, "\n")
	if !opts.CaseSensitive {
		hay = strings.ToLower(hay)
	}
	matched := 0
	for i, q := range opts.Queries {
		ok := false
		if opts.Regex {
			ok = compiled[i].MatchString(strings.Join([]string{string(b.Kind), b.ToolName, b.Text, toolInputText(b)}, "\n"))
		} else {
			needle := q
			if !opts.CaseSensitive {
				needle = strings.ToLower(needle)
			}
			ok = strings.Contains(hay, needle)
		}
		if ok {
			matched++
			if opts.Mode == SearchAny {
				return true
			}
		}
	}
	return opts.Mode == SearchAll && matched == len(opts.Queries)
}

func nearMatches(session Session, tr Transcript, blocks []Block, opts SearchOptions, compiled []*regexp.Regexp) []Match {
	var out []Match
	for i, b := range blocks {
		windowStart := i - opts.Near
		if windowStart < 0 {
			windowStart = 0
		}
		windowEnd := i + opts.Near
		if windowEnd >= len(blocks) {
			windowEnd = len(blocks) - 1
		}
		found := make([]bool, len(opts.Queries))
		for j := windowStart; j <= windowEnd; j++ {
			for qi := range opts.Queries {
				one := opts
				one.Queries = []string{opts.Queries[qi]}
				one.Mode = SearchAny
				var oneRE []*regexp.Regexp
				if opts.Regex {
					oneRE = []*regexp.Regexp{compiled[qi]}
				}
				if blockMatches(blocks[j], one, oneRE) {
					found[qi] = true
				}
			}
		}
		all := true
		for _, ok := range found {
			if !ok {
				all = false
				break
			}
		}
		if all {
			s := session
			s.Provider = tr.Provider
			out = append(out, Match{Session: s, Block: b, Snippet: fmt.Sprintf("all query terms found within %d blocks of #%03d", opts.Near, b.Index)})
		}
	}
	return out
}

func dedupeMatches(matches []Match) []Match {
	seen := map[string]bool{}
	out := make([]Match, 0, len(matches))
	for _, m := range matches {
		key := fmt.Sprintf("%s:%d", m.Session.Path, m.Block.Index)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, m)
	}
	return out
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
