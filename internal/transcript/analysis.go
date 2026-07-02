package transcript

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
)

type CommandEntry struct {
	Index       int    `json:"index"`
	ResultIndex int    `json:"result_index,omitempty"`
	Command     string `json:"command"`
	Output      string `json:"output,omitempty"`
	IsError     bool   `json:"is_error,omitempty"`
	Timestamp   string `json:"timestamp,omitempty"`
}

func CommandEntries(tr Transcript, failedOnly bool, grep string, regex, caseSensitive, withOutput bool) ([]CommandEntry, error) {
	resultByID := map[string]Block{}
	for _, b := range tr.Blocks {
		if b.Kind == KindCommandResult {
			resultByID[b.ToolID] = b
		}
	}
	var re *regexp.Regexp
	var err error
	if regex && grep != "" {
		pat := grep
		if !caseSensitive {
			pat = "(?i)" + pat
		}
		re, err = regexp.Compile(pat)
		if err != nil {
			return nil, err
		}
	}
	var out []CommandEntry
	for _, b := range tr.Blocks {
		if b.Kind != KindCommand {
			continue
		}
		cmd := toolInputText(b)
		res := resultByID[b.ToolID]
		entry := CommandEntry{Index: b.Index, Command: cmd, IsError: res.IsError, Timestamp: b.Timestamp}
		if res.Index > 0 {
			entry.ResultIndex = res.Index
		}
		if withOutput {
			entry.Output = res.Text
		}
		if failedOnly && !entry.IsError {
			continue
		}
		if grep != "" {
			hay := cmd + "\n" + res.Text
			ok := false
			if regex {
				ok = re.MatchString(hay)
			} else if caseSensitive {
				ok = strings.Contains(hay, grep)
			} else {
				ok = strings.Contains(strings.ToLower(hay), strings.ToLower(grep))
			}
			if !ok {
				continue
			}
		}
		out = append(out, entry)
	}
	return out, nil
}

func RenderCommands(w io.Writer, entries []CommandEntry, format string) error {
	if format == "json" {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	}
	for _, e := range entries {
		status := "ok"
		if e.IsError {
			status = "ERROR"
		}
		if e.ResultIndex > 0 {
			_, _ = fmt.Fprintf(w, "#%03d command -> #%03d result %s\n", e.Index, e.ResultIndex, status)
		} else {
			_, _ = fmt.Fprintf(w, "#%03d command %s\n", e.Index, status)
		}
		_, _ = fmt.Fprintf(w, "  %s\n", strings.ReplaceAll(strings.TrimSpace(e.Command), "\n", "\n  "))
		if e.Output != "" {
			_, _ = fmt.Fprintf(w, "  output:\n%s\n", indentRight(strings.TrimRight(e.Output, "\n"), "    "))
		}
		_, _ = fmt.Fprintln(w)
	}
	return nil
}

type FileRef struct {
	Path    string   `json:"path"`
	Indexes []int    `json:"indexes"`
	Ops     []string `json:"ops"`
}

var fileLikeRE = regexp.MustCompile(`(?:^|[\s'"(])((?:[A-Za-z0-9_./-]+/)?[A-Za-z0-9_.-]+\.(?:go|rs|ts|tsx|js|jsx|mjs|cjs|json|md|mdx|css|html|astro|yml|yaml|toml|sh|py|rb|java|kt|swift|sql|txt|lock|mod|sum))`)

func FileRefs(tr Transcript, changedOnly bool) []FileRef {
	m := map[string]*FileRef{}
	add := func(path string, idx int, op string) {
		path = strings.Trim(path, "'\"` ,:;()[]{}")
		if path == "" || strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
			return
		}
		ref := m[path]
		if ref == nil {
			ref = &FileRef{Path: path}
			m[path] = ref
		}
		if !intInList(idx, ref.Indexes) {
			ref.Indexes = append(ref.Indexes, idx)
		}
		if op != "" && !stringInList(op, ref.Ops) {
			ref.Ops = append(ref.Ops, op)
		}
	}
	for _, b := range tr.Blocks {
		op := opForBlock(b)
		if changedOnly && op != "write" && op != "edit" && op != "delete" && op != "patch" {
			continue
		}
		for _, key := range []string{"file_path", "path", "old_file", "new_file"} {
			if v, ok := b.ToolInput[key]; ok {
				add(fmt.Sprint(v), b.Index, op)
			}
		}
		if b.Kind != KindCommandResult && b.Kind != KindToolResult {
			for _, match := range fileLikeRE.FindAllStringSubmatch(blockBodyText(b), -1) {
				if len(match) > 1 {
					add(match[1], b.Index, op)
				}
			}
		}
	}
	out := make([]FileRef, 0, len(m))
	for _, ref := range m {
		sort.Ints(ref.Indexes)
		sort.Strings(ref.Ops)
		out = append(out, *ref)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func RenderFileRefs(w io.Writer, refs []FileRef, format string) error {
	if format == "json" {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(refs)
	}
	for _, ref := range refs {
		_, _ = fmt.Fprintf(w, "%s\n", ref.Path)
		if len(ref.Ops) > 0 {
			_, _ = fmt.Fprintf(w, "  ops: %s\n", strings.Join(ref.Ops, ", "))
		}
		parts := make([]string, len(ref.Indexes))
		for i, idx := range ref.Indexes {
			parts[i] = fmt.Sprintf("#%03d", idx)
		}
		_, _ = fmt.Fprintf(w, "  blocks: %s\n\n", strings.Join(parts, ", "))
	}
	return nil
}

func opForBlock(b Block) string {
	name := strings.ToLower(b.ToolName)
	if b.Kind == KindCommand {
		cmd := strings.ToLower(toolInputText(b))
		switch {
		case strings.Contains(cmd, "apply_patch") || strings.Contains(cmd, "git apply"):
			return "patch"
		case strings.Contains(cmd, " rm ") || strings.HasPrefix(cmd, "rm "):
			return "delete"
		default:
			return "command"
		}
	}
	switch {
	case name == "write":
		return "write"
	case name == "edit" || name == "multiedit":
		return "edit"
	case strings.Contains(name, "patch"):
		return "patch"
	case name == "read":
		return "read"
	default:
		return name
	}
}

type GitActivity struct {
	Branches    []string `json:"branches,omitempty"`
	Commits     []string `json:"commits,omitempty"`
	PRs         []string `json:"prs,omitempty"`
	Pushes      []string `json:"pushes,omitempty"`
	Validations []string `json:"validations,omitempty"`
	Failures    []string `json:"failures,omitempty"`
}

var prURLRE = regexp.MustCompile(`https://github\.com/[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+/pull/[0-9]+`)
var commitRE = regexp.MustCompile(`\b[0-9a-f]{7,40}\b`)
var branchRE = regexp.MustCompile(`(?:branch|refs/heads/)[:\s']+([A-Za-z0-9_.\/-]+)`)

func GitAndPRActivity(tr Transcript) GitActivity {
	var a GitActivity
	addUnique := func(dst *[]string, v string) {
		v = strings.TrimSpace(v)
		if v == "" || stringInList(v, *dst) {
			return
		}
		*dst = append(*dst, v)
	}
	for _, b := range tr.Blocks {
		text := blockBodyText(b)
		for _, pr := range prURLRE.FindAllString(text, -1) {
			addUnique(&a.PRs, pr)
		}
		if b.Kind == KindCommand {
			cmd := strings.TrimSpace(toolInputText(b))
			low := strings.ToLower(cmd)
			switch {
			case strings.Contains(low, "git push"):
				addUnique(&a.Pushes, fmt.Sprintf("#%03d %s", b.Index, cmd))
			case strings.Contains(low, "git commit"):
				addUnique(&a.Commits, fmt.Sprintf("#%03d %s", b.Index, cmd))
			case strings.Contains(low, "go test") || strings.Contains(low, "make check") || strings.Contains(low, "make test") || strings.Contains(low, "npm run") || strings.Contains(low, "bun run"):
				addUnique(&a.Validations, fmt.Sprintf("#%03d %s", b.Index, cmd))
			}
		}
		if b.Kind == KindCommandResult || b.Kind == KindToolResult {
			low := strings.ToLower(text)
			if b.IsError || strings.Contains(low, "remote rejected") || strings.Contains(low, "error:") || strings.Contains(low, "failed") || strings.Contains(low, "permission denied") {
				addUnique(&a.Failures, fmt.Sprintf("#%03d %s", b.Index, firstLine(text)))
			}
			for _, c := range commitRE.FindAllString(text, -1) {
				addUnique(&a.Commits, c)
			}
			for _, m := range branchRE.FindAllStringSubmatch(text, -1) {
				if len(m) > 1 {
					addUnique(&a.Branches, m[1])
				}
			}
		}
	}
	sort.Strings(a.Branches)
	sort.Strings(a.Commits)
	sort.Strings(a.PRs)
	return a
}

func RenderGitActivity(w io.Writer, a GitActivity, format string) error {
	if format == "json" {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(a)
	}
	sections := []struct {
		Title string
		Items []string
	}{
		{"Branches", a.Branches},
		{"Commits", a.Commits},
		{"Pull requests", a.PRs},
		{"Pushes", a.Pushes},
		{"Validations", a.Validations},
		{"Failures", a.Failures},
	}
	for _, section := range sections {
		_, _ = fmt.Fprintf(w, "%s:\n", section.Title)
		if len(section.Items) == 0 {
			_, _ = fmt.Fprintln(w, "  none")
		} else {
			for _, item := range section.Items {
				_, _ = fmt.Fprintf(w, "  - %s\n", item)
			}
		}
		_, _ = fmt.Fprintln(w)
	}
	return nil
}

func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			if len(line) > 160 {
				return line[:157] + "..."
			}
			return line
		}
	}
	return ""
}

func stringInList(v string, vals []string) bool {
	for _, x := range vals {
		if x == v {
			return true
		}
	}
	return false
}
