package transcript

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The CLI stores its profile relative to the OS user's home on all platforms.
// os.UserHomeDir resolves HOME on Unix and USERPROFILE on Windows.
func antigravityRoot() string {
	return expandHome("~/.gemini/antigravity-cli/brain")
}

func agyLogDir(sessionDir string) string {
	return filepath.Join(sessionDir, ".system_generated", "logs")
}

func agySessionDir(path string) string {
	dir := filepath.Dir(path)
	if filepath.Base(dir) != "logs" || filepath.Base(filepath.Dir(dir)) != ".system_generated" {
		return ""
	}
	session := filepath.Dir(filepath.Dir(dir))
	if !IsSessionID(filepath.Base(session)) {
		return ""
	}
	return session
}

// Truncated, full and chunked copies can coexist. Keep only one in the catalog.
func preferredAgyLog(session string) string {
	dir := agyLogDir(session)
	for _, name := range []string{"transcript_full.jsonl", "transcript.jsonl"} {
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() && info.Size() > 0 {
			return path
		}
	}
	return ""
}

type agyMetadata struct {
	Summary struct {
		WorkspaceURIs []string
		Title         string
		Preview       string
	}
}

func agyMetadataForBrain(brain string) map[string]agyMetadata {
	var cache struct{ Conversations map[string]agyMetadata }
	f, err := os.Open(filepath.Join(filepath.Dir(brain), "cache", "conversation_metadata.json"))
	if err == nil {
		defer f.Close()
		_ = json.NewDecoder(f).Decode(&cache)
	}
	if cache.Conversations == nil {
		cache.Conversations = make(map[string]agyMetadata)
	}
	if data, err := os.ReadFile(filepath.Join(filepath.Dir(brain), "jetbox_summaries_proto.pb")); err == nil {
		loadAgySummaries(data, cache.Conversations)
	}
	return cache.Conversations
}

func applyAgyMetadata(session *Session, meta agyMetadata) {
	if uris := meta.Summary.WorkspaceURIs; len(uris) > 0 && strings.HasPrefix(uris[0], "file://") {
		session.CWD = NormalizeTranscriptPath(uris[0])
	}
	if meta.Summary.Title != "" {
		session.Title = meta.Summary.Title
	} else if meta.Summary.Preview != "" {
		session.Title = meta.Summary.Preview
	}
}

func hydrateAgyLine(s *Session, obj map[string]any) {
	if s.StartedAt.IsZero() {
		s.StartedAt, _ = time.Parse(time.RFC3339Nano, stringValue(obj["created_at"]))
	}
	if s.FirstPrompt == "" && obj["type"] == "USER_INPUT" {
		s.FirstPrompt = cleanPrompt(agyUserText(stringValue(obj["content"])))
	}
	msg := mapValue(obj["msg"])
	params := mapValue(msg["params"])
	if s.ID == "" {
		for _, candidate := range []any{obj["conversation_id"], mapValue(obj["result"])["conversation_id"], mapValue(msg["result"])["sessionId"], params["sessionId"]} {
			if id := stringValue(candidate); IsSessionID(id) {
				s.ID = id
				break
			}
		}
	}
	if s.CWD == "" {
		s.CWD = stringValue(mapValue(obj["init"])["cwd"])
		if s.CWD == "" {
			s.CWD = stringValue(params["cwd"])
		}
	}
	if s.FirstPrompt == "" && msg["method"] == "session/prompt" {
		s.FirstPrompt = cleanPrompt(agyContent(params["prompt"]))
	}
}

func readAgyExport(path string) map[string]any {
	if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var obj map[string]any
	if json.NewDecoder(io.LimitReader(f, maxJSONLineSize)).Decode(&obj) != nil || !isAntigravityCapture(obj) {
		return nil
	}
	return obj
}

func ResolveAntigravitySessionID(id string, roots []string) (string, error) {
	return resolveUUID(id, roots, ProviderAntigravity)
}
