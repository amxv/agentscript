package app

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/amxv/agentscript/internal/transcript"
)

func runIndex(args []string, stdout io.Writer) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		printIndexHelp(stdout)
		return nil
	}
	action := strings.ToLower(args[0])
	if action == "clear" {
		if err := transcript.ClearIndex(); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(stdout, "cleared agentscript search index")
		return nil
	}

	var provider, roots, project, since, format string
	var latest int
	var refresh bool
	fs := flag.NewFlagSet("index "+action, flag.ContinueOnError)
	fs.StringVar(&provider, "provider", "", "provider filter: claude or codex")
	fs.StringVar(&roots, "roots", "", "comma-separated roots for discovery")
	fs.StringVar(&project, "project", "", "only include sessions whose project/cwd matches this value")
	fs.StringVar(&since, "since", "", "only include sessions since a date or duration such as 30d")
	fs.IntVar(&latest, "latest", 0, "only include the N latest sessions")
	fs.StringVar(&format, "format", "text", "output format: text or json")
	fs.BoolVar(&refresh, "refresh", action == "rebuild", "refresh the cached session catalog")
	if err := fs.Parse(interspersed(args[1:], map[string]bool{
		"provider": true, "roots": true, "project": true, "since": true, "latest": true, "format": true,
	})); err != nil {
		return err
	}

	sessions, err := transcript.DiscoverCached(0, parseProvider(provider), splitCSV(roots), refresh)
	if err != nil {
		return err
	}
	sessions, err = filterSessions(sessions, project, since)
	if err != nil {
		return err
	}
	if latest > 0 && len(sessions) > latest {
		sessions = sessions[:latest]
	}

	switch action {
	case "status":
		status := transcript.GetIndexStatus(sessions)
		if strings.EqualFold(format, "json") {
			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(status)
		}
		_, _ = fmt.Fprintf(stdout, "sessions: %d\nindexed:  %d\nstale:    %d\nmissing:  %d\nsize:     %s\ncache:    %s\n",
			status.Sessions, status.Cached, status.Stale, status.Missing, humanBytes(status.CacheBytes), status.CacheDir)
		return nil
	case "rebuild":
		status, err := transcript.RebuildIndex(sessions, func(done, total int, _ transcript.Session) {
			if done == total || done%100 == 0 {
				_, _ = fmt.Fprintf(stdout, "indexed %d/%d\n", done, total)
			}
		})
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(stdout, "index ready: %d/%d sessions, %s\n", status.Cached, status.Sessions, humanBytes(status.CacheBytes))
		return nil
	default:
		return fmt.Errorf("unknown index command %q", action)
	}
}

func filterSessions(sessions []transcript.Session, project, since string) ([]transcript.Session, error) {
	cutoff, err := parseSince(since)
	if err != nil {
		return nil, err
	}
	projectNeedle := strings.ToLower(strings.TrimSpace(project))
	out := make([]transcript.Session, 0, len(sessions))
	for _, session := range sessions {
		if !cutoff.IsZero() && session.ModTime.Before(cutoff) {
			continue
		}
		if projectNeedle != "" {
			session = transcript.HydrateSessionCached(session)
			hay := strings.ToLower(session.Project + "\n" + session.CWD)
			if !strings.Contains(hay, projectNeedle) {
				continue
			}
		}
		out = append(out, session)
	}
	return out, nil
}

func parseSince(value string) (time.Time, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return time.Time{}, nil
	}
	if strings.HasSuffix(value, "d") || strings.HasSuffix(value, "w") {
		unit := value[len(value)-1]
		var n int
		if _, err := fmt.Sscanf(value[:len(value)-1], "%d", &n); err != nil || n < 0 {
			return time.Time{}, fmt.Errorf("invalid --since %q", value)
		}
		d := time.Duration(n) * 24 * time.Hour
		if unit == 'w' {
			d *= 7
		}
		return time.Now().Add(-d), nil
	}
	if d, err := time.ParseDuration(value); err == nil {
		return time.Now().Add(-d), nil
	}
	if t, err := time.ParseInLocation("2006-01-02", value, time.Local); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid --since %q; use a date, RFC3339 timestamp, duration, or values like 30d/2w", value)
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for value := n / unit; value >= unit; value /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

func fallbackDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func printIndexHelp(w io.Writer) {
	writeLines(w,
		"agentscript index - inspect or build the normalized transcript search cache",
		"",
		"Usage:",
		"  agentscript index status [flags]",
		"  agentscript index rebuild [flags]",
		"  agentscript index clear",
		"",
		"Examples:",
		"  agentscript index status",
		"  agentscript index rebuild --since 30d",
		"  agentscript index rebuild --project agentscript",
	)
}
