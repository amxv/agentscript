package transcript

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
)

const searchCacheVersion = 2

type searchCacheEntry struct {
	Version     int     `json:"version"`
	Path        string  `json:"path"`
	Size        int64   `json:"size"`
	ModUnixNano int64   `json:"mod_unix_nano"`
	Session     Session `json:"session"`
	Blocks      []Block `json:"blocks"`
}

type searchCacheMeta struct {
	Version     int     `json:"version"`
	Path        string  `json:"path"`
	Size        int64   `json:"size"`
	ModUnixNano int64   `json:"mod_unix_nano"`
	Session     Session `json:"session"`
}

type SearchStats struct {
	Sessions        int `json:"sessions"`
	IndexedSessions int `json:"indexed_sessions"`
	Candidates      int `json:"candidates"`
	CacheHits       int `json:"cache_hits"`
	CacheMisses     int `json:"cache_misses"`
	Parsed          int `json:"parsed"`
	ParseErrors     int `json:"parse_errors"`
	MatchedHits     int `json:"matched_hits"`
	MatchedFiles    int `json:"matched_files"`
}

type IndexStatus struct {
	Sessions   int    `json:"sessions"`
	Cached     int    `json:"cached"`
	Stale      int    `json:"stale"`
	Missing    int    `json:"missing"`
	CacheBytes int64  `json:"cache_bytes"`
	CacheDir   string `json:"cache_dir"`
}

func SearchHistory(sessions []Session, opts SearchOptions) ([]Match, SearchStats, error) {
	stats := SearchStats{Sessions: len(sessions)}
	if len(opts.Queries) == 0 {
		return nil, stats, fmt.Errorf("query cannot be empty")
	}
	if opts.Mode == "" {
		opts.Mode = SearchAll
	}
	compiled, err := compileQueries(opts)
	if err != nil {
		return nil, stats, err
	}

	var matches []Match
	indexed := make([]Session, 0, len(sessions))
	uncached := make([]Session, 0, len(sessions))
	for _, session := range sessions {
		session = refreshSessionStat(session)
		if _, ok := loadSearchCacheMeta(session); !ok {
			uncached = append(uncached, session)
			continue
		}
		indexed = append(indexed, session)
	}
	stats.IndexedSessions = len(indexed)

	indexedCandidates, err := indexedCandidateSessions(indexed, opts)
	if err != nil {
		return nil, stats, err
	}
	for _, session := range indexedCandidates {
		entry, ok := loadSearchCache(session)
		if !ok {
			// A partial/corrupt cache should heal through the source path below.
			uncached = append(uncached, session)
			continue
		}
		stats.CacheHits++
		session = mergeSession(session, entry.Session)
		if session.FirstPrompt == "" {
			session = HydrateSessionFromTranscript(session, Transcript{Path: session.Path, Provider: session.Provider, Blocks: entry.Blocks})
		}
		matches = append(matches, searchTranscript(session, entry.Blocks, opts, compiled)...)
	}

	candidates, err := candidateSessions(uncached, opts)
	if err != nil {
		return nil, stats, err
	}
	stats.Candidates = len(indexedCandidates) + len(candidates)
	for _, session := range candidates {
		session = refreshSessionStat(session)
		stats.CacheMisses++
		tr, err := ParseFile(session.Path)
		if err != nil {
			stats.ParseErrors++
			continue
		}
		stats.Parsed++
		session.Provider = tr.Provider
		session = HydrateSessionFromTranscript(session, tr)
		_ = writeSearchCache(session, tr.Blocks)
		matches = append(matches, searchTranscript(session, tr.Blocks, opts, compiled)...)
	}
	matches = dedupeMatches(matches)
	stats.MatchedHits = len(matches)
	stats.MatchedFiles = len(GroupMatches(matches))
	return matches, stats, nil
}

func mergeSession(base, cached Session) Session {
	cached.Path = base.Path
	cached.ModTime = base.ModTime
	cached.Size = base.Size
	if cached.Provider == "" || cached.Provider == ProviderUnknown {
		cached.Provider = base.Provider
	}
	return finalizeSessionMetadata(cached)
}

func refreshSessionStat(session Session) Session {
	if info, err := os.Stat(session.Path); err == nil {
		session.ModTime = info.ModTime()
		session.Size = info.Size()
	}
	return session
}

func candidateSessions(sessions []Session, opts SearchOptions) ([]Session, error) {
	if len(sessions) == 0 {
		return nil, nil
	}
	paths, err := ripgrepCandidates(sessions, opts)
	if err == nil {
		out := make([]Session, 0, len(paths))
		for _, session := range sessions {
			if paths[session.Path] {
				out = append(out, session)
			}
		}
		return out, nil
	}
	return goCandidates(sessions, opts)
}

func indexedCandidateSessions(sessions []Session, opts SearchOptions) ([]Session, error) {
	if len(sessions) == 0 {
		return nil, nil
	}
	paths, err := ripgrepIndexedCandidates(sessions, opts)
	if err == nil {
		out := make([]Session, 0, len(paths))
		for _, session := range sessions {
			if paths[searchCacheTextPath(session.Path)] {
				out = append(out, session)
			}
		}
		return out, nil
	}
	return goIndexedCandidates(sessions, opts), nil
}

func ripgrepIndexedCandidates(sessions []Session, opts SearchOptions) (map[string]bool, error) {
	rg, err := exec.LookPath("rg")
	if err != nil {
		return nil, err
	}
	var targets []string
	if len(sessions) <= 160 {
		for _, session := range sessions {
			targets = append(targets, searchCacheTextPath(session.Path))
		}
	} else {
		targets = []string{searchCacheDir()}
	}
	result := map[string]bool{}
	for qi, query := range opts.Queries {
		args := []string{"-l", "--no-messages", "-g", "*.txt"}
		if !opts.CaseSensitive {
			args = append(args, "-i")
		}
		if !opts.Regex {
			args = append(args, "-F")
		}
		if opts.Regex || strings.Contains(query, "\n") {
			args = append(args, "-U")
		}
		args = append(args, "--", query)
		args = append(args, targets...)
		out, runErr := exec.Command(rg, args...).Output()
		if runErr != nil {
			var exitErr *exec.ExitError
			if !errors.As(runErr, &exitErr) || exitErr.ExitCode() != 1 {
				return nil, runErr
			}
		}
		set := map[string]bool{}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if line == "" {
				continue
			}
			abs, _ := filepath.Abs(line)
			set[abs] = true
		}
		if qi == 0 || opts.Mode == SearchAny {
			for path := range set {
				result[path] = true
			}
		} else {
			for path := range result {
				if !set[path] {
					delete(result, path)
				}
			}
		}
	}
	return result, nil
}

func goIndexedCandidates(sessions []Session, opts SearchOptions) []Session {
	out := make([]Session, 0, len(sessions))
	for _, session := range sessions {
		if rawFileMatches(searchCacheTextPath(session.Path), opts) {
			out = append(out, session)
		}
	}
	return out
}

func ripgrepCandidates(sessions []Session, opts SearchOptions) (map[string]bool, error) {
	rg, err := exec.LookPath("rg")
	if err != nil {
		return nil, err
	}
	targets := searchTargets(sessions)
	if len(targets) == 0 {
		return map[string]bool{}, nil
	}
	var result map[string]bool
	for qi, query := range opts.Queries {
		args := []string{"-l", "--no-messages", "-g", "*.jsonl"}
		if !opts.CaseSensitive {
			args = append(args, "-i")
		}
		if !opts.Regex {
			args = append(args, "-F")
		}
		if opts.Regex || strings.Contains(query, "\n") {
			args = append(args, "-U")
		}
		args = append(args, "--", query)
		args = append(args, targets...)
		cmd := exec.Command(rg, args...)
		out, runErr := cmd.Output()
		if runErr != nil {
			var exitErr *exec.ExitError
			if !errors.As(runErr, &exitErr) || exitErr.ExitCode() != 1 {
				return nil, runErr
			}
		}
		set := map[string]bool{}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if line == "" {
				continue
			}
			abs, _ := filepath.Abs(line)
			set[abs] = true
		}
		if qi == 0 || opts.Mode == SearchAny {
			if result == nil {
				result = map[string]bool{}
			}
			for path := range set {
				result[path] = true
			}
		} else {
			for path := range result {
				if !set[path] {
					delete(result, path)
				}
			}
		}
	}
	if result == nil {
		result = map[string]bool{}
	}
	return result, nil
}

func searchTargets(sessions []Session) []string {
	if len(sessions) <= 160 {
		out := make([]string, 0, len(sessions))
		for _, session := range sessions {
			out = append(out, session.Path)
		}
		return out
	}
	// Passing thousands of filenames can exceed ARG_MAX. Parent directories are
	// much fewer for date-partitioned Codex stores; results are filtered back to
	// the supplied session set by candidateSessions.
	seen := map[string]bool{}
	var dirs []string
	for _, session := range sessions {
		dir := filepath.Dir(session.Path)
		if !seen[dir] {
			seen[dir] = true
			dirs = append(dirs, dir)
		}
	}
	sort.Strings(dirs)
	return dirs
}

func goCandidates(sessions []Session, opts SearchOptions) ([]Session, error) {
	type result struct {
		session Session
		match   bool
	}
	jobs := make(chan Session)
	results := make(chan result)
	workers := 8
	if len(sessions) < workers {
		workers = len(sessions)
	}
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for session := range jobs {
				results <- result{session: session, match: rawFileMatches(session.Path, opts)}
			}
		}()
	}
	go func() {
		wg.Wait()
		close(results)
	}()
	go func() {
		for _, session := range sessions {
			jobs <- session
		}
		close(jobs)
	}()
	var out []Session
	for r := range results {
		if r.match {
			out = append(out, r.session)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ModTime.After(out[j].ModTime) })
	return out, nil
}

func rawFileMatches(path string, opts SearchOptions) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	var compiled []*regexp.Regexp
	if opts.Regex {
		compiled, err = compileQueries(opts)
		if err != nil {
			return false
		}
	}
	found := make([]bool, len(opts.Queries))
	r := bufio.NewReaderSize(f, 256*1024)
	for {
		line, readErr := r.ReadString('\n')
		hay := line
		if !opts.CaseSensitive && !opts.Regex {
			hay = strings.ToLower(hay)
		}
		for i, query := range opts.Queries {
			if found[i] {
				continue
			}
			if opts.Regex {
				found[i] = compiled[i].MatchString(line)
			} else {
				needle := query
				if !opts.CaseSensitive {
					needle = strings.ToLower(needle)
				}
				found[i] = strings.Contains(hay, needle)
			}
		}
		if opts.Mode == SearchAny {
			for _, ok := range found {
				if ok {
					return true
				}
			}
		} else if allTrue(found) {
			return true
		}
		if readErr != nil {
			return false
		}
	}
}

func searchCacheDir() string {
	return filepath.Join(cacheBaseDir(), "search-v2")
}

func searchCachePath(path string) string {
	return filepath.Join(searchCacheDir(), cacheKey(path)+".json.gz")
}

func searchCacheTextPath(path string) string {
	return filepath.Join(searchCacheDir(), cacheKey(path)+".txt")
}

func searchCacheMetaPath(path string) string {
	return filepath.Join(searchCacheDir(), cacheKey(path)+".meta.json")
}

func loadSearchCacheMeta(session Session) (searchCacheMeta, bool) {
	var meta searchCacheMeta
	session = refreshSessionStat(session)
	data, err := os.ReadFile(searchCacheMetaPath(session.Path))
	if err != nil || json.Unmarshal(data, &meta) != nil {
		return meta, false
	}
	if meta.Version != searchCacheVersion || meta.Path != session.Path || meta.Size != session.Size || meta.ModUnixNano != session.ModTime.UnixNano() {
		return meta, false
	}
	if _, err := os.Stat(searchCachePath(session.Path)); err != nil {
		return meta, false
	}
	if _, err := os.Stat(searchCacheTextPath(session.Path)); err != nil {
		return meta, false
	}
	return meta, true
}

func loadSearchCache(session Session) (searchCacheEntry, bool) {
	var entry searchCacheEntry
	if _, ok := loadSearchCacheMeta(session); !ok {
		return entry, false
	}
	session = refreshSessionStat(session)
	f, err := os.Open(searchCachePath(session.Path))
	if err != nil {
		return entry, false
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return entry, false
	}
	defer gz.Close()
	if json.NewDecoder(gz).Decode(&entry) != nil {
		return entry, false
	}
	if entry.Version != searchCacheVersion || entry.Path != session.Path || entry.Size != session.Size || entry.ModUnixNano != session.ModTime.UnixNano() {
		return entry, false
	}
	return entry, true
}

func writeSearchCache(session Session, blocks []Block) error {
	dir := searchCacheDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := searchCachePath(session.Path)
	f, err := os.CreateTemp(dir, ".agentscript-search-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	gz := gzip.NewWriter(f)
	entry := searchCacheEntry{
		Version: searchCacheVersion, Path: session.Path, Size: session.Size,
		ModUnixNano: session.ModTime.UnixNano(), Session: session, Blocks: stripRawBlocks(blocks),
	}
	err = json.NewEncoder(gz).Encode(entry)
	if closeErr := gz.Close(); err == nil {
		err = closeErr
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := replaceFile(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := atomicWriteFile(searchCacheTextPath(session.Path), []byte(normalizedSearchText(blocks))); err != nil {
		return err
	}
	meta := searchCacheMeta{
		Version: searchCacheVersion, Path: session.Path, Size: session.Size,
		ModUnixNano: session.ModTime.UnixNano(), Session: session,
	}
	metaData, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	// Meta is written last and acts as the commit marker for the structured and
	// text sidecars above.
	return atomicWriteFile(searchCacheMetaPath(session.Path), metaData)
}

func normalizedSearchText(blocks []Block) string {
	var b strings.Builder
	for _, block := range blocks {
		b.WriteString(string(block.Kind))
		b.WriteByte('\n')
		b.WriteString(block.ToolName)
		b.WriteByte('\n')
		b.WriteString(block.Text)
		b.WriteByte('\n')
		b.WriteString(toolInputText(block))
		b.WriteString("\n\x1e\n")
	}
	return b.String()
}

func atomicWriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".agentscript-cache-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := replaceFile(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func replaceFile(tmp, path string) error {
	if runtime.GOOS == "windows" {
		_ = os.Remove(path)
	}
	return os.Rename(tmp, path)
}

func stripRawBlocks(blocks []Block) []Block {
	out := make([]Block, len(blocks))
	copy(out, blocks)
	for i := range out {
		out[i].Raw = nil
	}
	return out
}

func GetIndexStatus(sessions []Session) IndexStatus {
	status := IndexStatus{Sessions: len(sessions), CacheDir: searchCacheDir()}
	for _, session := range sessions {
		metaPath := searchCacheMetaPath(session.Path)
		if _, err := os.Stat(metaPath); err != nil {
			status.Missing++
			continue
		}
		if _, ok := loadSearchCacheMeta(session); ok {
			status.Cached++
		} else {
			status.Stale++
		}
		for _, path := range []string{metaPath, searchCachePath(session.Path), searchCacheTextPath(session.Path)} {
			if info, err := os.Stat(path); err == nil {
				status.CacheBytes += info.Size()
			}
		}
	}
	return status
}

func RebuildIndex(sessions []Session, progress func(done, total int, session Session)) (IndexStatus, error) {
	var firstErr error
	for i, session := range sessions {
		session = refreshSessionStat(session)
		tr, err := ParseFile(session.Path)
		if err == nil {
			session.Provider = tr.Provider
			session = HydrateSessionFromTranscript(session, tr)
			err = writeSearchCache(session, tr.Blocks)
		}
		if err != nil && firstErr == nil {
			firstErr = err
		}
		if progress != nil {
			progress(i+1, len(sessions), session)
		}
	}
	return GetIndexStatus(sessions), firstErr
}

func ClearIndex() error {
	for _, name := range []string{"search-v1", "search-v2"} {
		if err := os.RemoveAll(filepath.Join(cacheBaseDir(), name)); err != nil {
			return err
		}
	}
	return nil
}

func RenderSearchJSON(w io.Writer, groups []SearchGroup, stats SearchStats) error {
	type jsonHit struct {
		Index     int    `json:"index"`
		EndIndex  int    `json:"end_index,omitempty"`
		Turn      int    `json:"turn,omitempty"`
		Kind      Kind   `json:"kind"`
		ToolName  string `json:"tool_name,omitempty"`
		Timestamp string `json:"timestamp,omitempty"`
		IsError   bool   `json:"is_error,omitempty"`
		Snippet   string `json:"snippet,omitempty"`
	}
	type jsonGroup struct {
		Session Session   `json:"session"`
		Count   int       `json:"count"`
		Hits    []jsonHit `json:"hits"`
	}
	outGroups := make([]jsonGroup, 0, len(groups))
	for _, group := range groups {
		out := jsonGroup{Session: group.Session, Count: group.Count, Hits: make([]jsonHit, 0, len(group.Hits))}
		for _, hit := range group.Hits {
			out.Hits = append(out.Hits, jsonHit{
				Index: hit.Block.Index, EndIndex: hit.EndIndex, Turn: hit.Block.Turn,
				Kind: hit.Block.Kind, ToolName: hit.Block.ToolName, Timestamp: hit.Block.Timestamp,
				IsError: hit.Block.IsError, Snippet: hit.Snippet,
			})
		}
		outGroups = append(outGroups, out)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(struct {
		Stats  SearchStats `json:"stats"`
		Groups []jsonGroup `json:"groups"`
	}{Stats: stats, Groups: outGroups})
}
