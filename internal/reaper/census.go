// census.go — Renders the machine census for operators.
// Why: before this existed there was no way to SEE how many Kaboom processes were
// running, which is why twelve daemons could survive twenty hours of green test
// runs. A leak nobody can observe is a leak nobody fixes.

package reaper

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/instancegov"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/instancereg"
)

// FormatCensus renders one line per instance, plus a summary. Ages are relative
// because "up 61h0m" is actionable in a way that an absolute timestamp is not.
func FormatCensus(records []instancereg.Record, now time.Time) string {
	if len(records) == 0 {
		return "No Kaboom instances are registered on this machine.\n"
	}
	ordered := make([]instancereg.Record, len(records))
	copy(ordered, records)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Role != ordered[j].Role {
			return ordered[i].Role < ordered[j].Role
		}
		return ordered[i].PID < ordered[j].PID
	})

	var out strings.Builder
	fmt.Fprintf(&out, "%-8s %-8s %-9s %-13s %-10s %s\n",
		"PID", "ROLE", "PORTS", "UPTIME", "HEARTBEAT", "STATE DIR")
	daemons, bridges := 0, 0
	for _, rec := range ordered {
		switch rec.Role {
		case instancereg.RoleDaemon:
			daemons++
		case instancereg.RoleBridge:
			bridges++
		}
		fmt.Fprintf(&out, "%-8d %-8s %-9s %-13s %-10s %s\n",
			rec.PID, roleLabel(rec), portsLabel(rec.Ports),
			uptimeLabel(rec, now), heartbeatLabel(rec, now), stateDirLabel(rec))
	}
	fmt.Fprintf(&out, "\n%d daemon(s), %d bridge(s). Version(s): %s\n",
		daemons, bridges, versionsLabel(ordered))
	return out.String()
}

func roleLabel(rec instancereg.Record) string {
	if rec.Parallel {
		return string(rec.Role) + "*"
	}
	return string(rec.Role)
}

func portsLabel(ports []int) string {
	if len(ports) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(ports))
	for _, port := range ports {
		parts = append(parts, fmt.Sprint(port))
	}
	return strings.Join(parts, ",")
}

func uptimeLabel(rec instancereg.Record, now time.Time) string {
	started, ok := rec.Started()
	if !ok {
		return "unknown"
	}
	return now.Sub(started).Round(time.Minute).String()
}

// heartbeatLabel flags an instance the reaper would consider wedged, so the census
// and the reaper never tell an operator two different stories.
func heartbeatLabel(rec instancereg.Record, now time.Time) string {
	age, ok := rec.HeartbeatAge(now)
	if !ok {
		return "unknown"
	}
	if age > instancegov.DefaultHeartbeatTTL {
		return "STALE"
	}
	return age.Round(time.Second).String()
}

func stateDirLabel(rec instancereg.Record) string {
	if rec.StateDir == "" {
		return "-"
	}
	return rec.StateDir
}

func versionsLabel(records []instancereg.Record) string {
	seen := map[string]bool{}
	var versions []string
	for _, rec := range records {
		if rec.Version != "" && !seen[rec.Version] {
			seen[rec.Version] = true
			versions = append(versions, rec.Version)
		}
	}
	if len(versions) == 0 {
		return "unknown"
	}
	sort.Strings(versions)
	return strings.Join(versions, ", ")
}
