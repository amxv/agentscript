package transcript

import (
	"fmt"
	"regexp"
	"sort"
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
	Session  Session `json:"session"`
	Block    Block   `json:"block"`
	EndIndex int     `json:"end_index,omitempty"`
	Snippet  string  `json:"snippet,omitempty"`
}

type SearchGroup struct {
	Session Session `json:"session"`
	Hits    []Match `json:"hits"`
	Count   int     `json:"count"`
}

func Search(query string, sessions []Session, opts RenderOptions) ([]Match, error) {
	return SearchAdvanced(sessions, SearchOptions{Queries: []string{query}, Mode: SearchAll, RenderOptions: opts})
}

func SearchAdvanced(sessions []Session, opts SearchOptions) ([]Match, error) {
	if len(opts.Queries) == 0 {
		return nil, fmt.Errorf("query cannot be empty")
	}
	if opts.Mode == "" {
		opts.Mode = SearchAll
	}
	compiled, err := compileQueries(opts)
	if err != nil {
		return nil, err
	}
	var matches []Match
	for _, s := range sessions {
		tr, err := ParseFile(s.Path)
		if err != nil {
			continue
		}
		s.Provider = tr.Provider
		matches = append(matches, searchTranscript(s, tr.Blocks, opts, compiled)...)
	}
	return dedupeMatches(matches), nil
}

func searchTranscript(session Session, rawBlocks []Block, opts SearchOptions, compiled []*regexp.Regexp) []Match {
	blocks := FilterBlocks(rawBlocks, opts.RenderOptions)
	filtered := make([]Block, 0, len(blocks))
	for _, b := range blocks {
		if len(opts.Kinds) > 0 && !kindInList(b.Kind, opts.Kinds) {
			continue
		}
		if len(opts.Tools) > 0 && !nameInList(b.ToolName, opts.Tools) {
			continue
		}
		filtered = append(filtered, b)
	}
	if opts.Near > 0 && len(opts.Queries) > 1 {
		return nearMatches(session, filtered, opts, compiled)
	}
	var out []Match
	for _, b := range filtered {
		if blockMatches(b, opts, compiled) {
			out = append(out, Match{Session: session, Block: b, Snippet: snippetForQueries(blockBodyText(b), opts.Queries)})
		}
	}
	return out
}

func compileQueries(opts SearchOptions) ([]*regexp.Regexp, error) {
	if !opts.Regex {
		return nil, nil
	}
	compiled := make([]*regexp.Regexp, 0, len(opts.Queries))
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
	return compiled, nil
}

func blockMatches(b Block, opts SearchOptions, compiled []*regexp.Regexp) bool {
	hayRaw := strings.Join([]string{string(b.Kind), b.ToolName, b.Text, toolInputText(b)}, "\n")
	hay := hayRaw
	if !opts.CaseSensitive && !opts.Regex {
		hay = strings.ToLower(hay)
	}
	matched := 0
	for i, q := range opts.Queries {
		ok := false
		if opts.Regex {
			ok = compiled[i].MatchString(hayRaw)
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

// nearMatches returns spans anchored on blocks that actually matched query text.
// A span qualifies when every query has a hit and the stable block-index distance
// from the first hit to the last hit is <= Near.
func nearMatches(session Session, blocks []Block, opts SearchOptions, compiled []*regexp.Regexp) []Match {
	if len(blocks) == 0 {
		return nil
	}
	type tagged struct {
		block Block
		mask  []bool
	}
	events := make([]tagged, 0, len(blocks))
	for _, b := range blocks {
		mask := make([]bool, len(opts.Queries))
		any := false
		for qi := range opts.Queries {
			one := opts
			one.Queries = []string{opts.Queries[qi]}
			one.Mode = SearchAny
			var oneRE []*regexp.Regexp
			if opts.Regex {
				oneRE = []*regexp.Regexp{compiled[qi]}
			}
			if blockMatches(b, one, oneRE) {
				mask[qi] = true
				any = true
			}
		}
		if any {
			events = append(events, tagged{block: b, mask: mask})
		}
	}

	var out []Match
	for i := 0; i < len(events); i++ {
		found := make([]bool, len(opts.Queries))
		for j := i; j < len(events); j++ {
			if events[j].block.Index-events[i].block.Index > opts.Near {
				break
			}
			for qi, ok := range events[j].mask {
				found[qi] = found[qi] || ok
			}
			if allTrue(found) {
				start, end := events[i].block, events[j].block
				out = append(out, Match{
					Session:  session,
					Block:    start,
					EndIndex: end.Index,
					Snippet:  fmt.Sprintf("all query terms matched in blocks #%03d–#%03d", start.Index, end.Index),
				})
				break
			}
		}
	}
	return dedupeMatches(out)
}

func allTrue(values []bool) bool {
	for _, value := range values {
		if !value {
			return false
		}
	}
	return true
}

func dedupeMatches(matches []Match) []Match {
	seen := map[string]bool{}
	out := make([]Match, 0, len(matches))
	for _, m := range matches {
		key := fmt.Sprintf("%s:%d:%d", m.Session.Path, m.Block.Index, m.EndIndex)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, m)
	}
	return out
}

func GroupMatches(matches []Match) []SearchGroup {
	byPath := map[string]*SearchGroup{}
	var order []string
	for _, match := range matches {
		group := byPath[match.Session.Path]
		if group == nil {
			order = append(order, match.Session.Path)
			group = &SearchGroup{Session: match.Session}
			byPath[match.Session.Path] = group
		}
		group.Hits = append(group.Hits, match)
		group.Count++
	}
	groups := make([]SearchGroup, 0, len(order))
	for _, path := range order {
		groups = append(groups, *byPath[path])
	}
	sort.SliceStable(groups, func(i, j int) bool {
		if (groups[i].Session.Auxiliary == "") != (groups[j].Session.Auxiliary == "") {
			return groups[i].Session.Auxiliary == ""
		}
		return groups[i].Session.ModTime.After(groups[j].Session.ModTime)
	})
	return groups
}

func snippetForQueries(text string, queries []string) string {
	for _, query := range queries {
		if value := snippet(text, query); value != "" {
			return value
		}
	}
	return snippet(text, "")
}

func snippet(text, query string) string {
	flat := strings.Join(strings.Fields(strings.ReplaceAll(text, "\n", " ")), " ")
	if flat == "" {
		return ""
	}
	idx := 0
	if query != "" {
		idx = strings.Index(strings.ToLower(flat), strings.ToLower(query))
		if idx < 0 {
			idx = 0
		}
	}
	start := idx - 50
	if start < 0 {
		start = 0
	}
	end := idx + len(query) + 80
	if query == "" {
		end = 140
	}
	if end > len(flat) {
		end = len(flat)
	}
	prefix, suffix := "", ""
	if start > 0 {
		prefix = "..."
	}
	if end < len(flat) {
		suffix = "..."
	}
	return prefix + flat[start:end] + suffix
}
