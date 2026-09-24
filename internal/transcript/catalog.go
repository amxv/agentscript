package transcript

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const catalogCacheVersion = 1

var catalogTTL = 30 * time.Second

type catalogSnapshot struct {
	Version   int             `json:"version"`
	CreatedAt time.Time       `json:"created_at"`
	Sources   []SessionSource `json:"sources"`
	Sessions  []Session       `json:"sessions"`
}

func DiscoverCached(limit int, provider Provider, roots []string, refresh bool) ([]Session, error) {
	sources := SourcesFromRoots(roots, provider)
	path := catalogCachePath(sources)
	if !refresh {
		if sessions, ok := loadCatalog(path, sources); ok {
			return filterAndLimitSessions(sessions, provider, limit), nil
		}
	}
	sessions, err := DiscoverSources(0, provider, sources)
	if err != nil {
		return nil, err
	}
	_ = writeCatalog(path, sources, sessions)
	return filterAndLimitSessions(sessions, provider, limit), nil
}

func HydrateSessionCached(session Session) Session {
	if meta, ok := loadSearchCacheMeta(session); ok {
		merged := mergeSession(session, meta.Session)
		if merged.FirstPrompt == "" {
			return HydrateSession(merged)
		}
		return merged
	}
	return HydrateSession(session)
}

func HydrateSessions(sessions []Session, limit int) []Session {
	if limit <= 0 || limit > len(sessions) {
		limit = len(sessions)
	}
	out := append([]Session(nil), sessions...)
	for i := 0; i < limit; i++ {
		out[i] = HydrateSessionCached(out[i])
	}
	return out
}

func catalogCachePath(sources []SessionSource) string {
	parts := make([]string, 0, len(sources))
	for _, source := range sources {
		parts = append(parts, string(source.Provider)+"="+filepath.Clean(expandHome(source.Path)))
	}
	sort.Strings(parts)
	return filepath.Join(cacheBaseDir(), "catalog-v1", cacheKey(strings.Join(parts, "\n"))+".json")
}

func loadCatalog(path string, sources []SessionSource) ([]Session, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var snapshot catalogSnapshot
	if json.Unmarshal(data, &snapshot) != nil || snapshot.Version != catalogCacheVersion {
		return nil, false
	}
	if time.Since(snapshot.CreatedAt) > catalogTTL || !sameSources(snapshot.Sources, sources) {
		return nil, false
	}
	return snapshot.Sessions, true
}

func writeCatalog(path string, sources []SessionSource, sessions []Session) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	snapshot := catalogSnapshot{Version: catalogCacheVersion, CreatedAt: time.Now(), Sources: sources, Sessions: sessions}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	return atomicWriteFile(path, data)
}

func sameSources(a, b []SessionSource) bool {
	if len(a) != len(b) {
		return false
	}
	normalize := func(values []SessionSource) []string {
		out := make([]string, 0, len(values))
		for _, source := range values {
			out = append(out, string(source.Provider)+"="+filepath.Clean(expandHome(source.Path)))
		}
		sort.Strings(out)
		return out
	}
	aa, bb := normalize(a), normalize(b)
	for i := range aa {
		if aa[i] != bb[i] {
			return false
		}
	}
	return true
}

func filterAndLimitSessions(sessions []Session, provider Provider, limit int) []Session {
	out := make([]Session, 0, len(sessions))
	for _, session := range sessions {
		if provider != "" && provider != ProviderUnknown && session.Provider != provider {
			continue
		}
		out = append(out, session)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ModTime.After(out[j].ModTime) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
