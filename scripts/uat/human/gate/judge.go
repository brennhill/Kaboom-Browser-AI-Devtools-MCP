// judge.go — Decides whether a run clears a release, and says exactly what does not.

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/scripts/uat/human/evidence"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/scripts/uat/human/inventory"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/scripts/uat/human/runlog"
)

// minimumWaiverReason is how much text a waiver must carry.
//
// A waiver is somebody accepting a known risk on the record. "n/a" or "later"
// accepts nothing and tells the next reader nothing, so the gate refuses it and
// the case counts as unanswered.
const minimumWaiverReason = 20

// waiver is one accepted risk.
type waiver struct {
	CaseID string `json:"case_id"`
	Reason string `json:"reason"`
	// Owner is who accepted it. A waiver with nobody's name on it is a way for
	// everyone to assume somebody else looked.
	Owner string `json:"owner"`
}

// waiverFile is the whole file.
type waiverFile struct {
	Version int      `json:"version"`
	Waivers []waiver `json:"waivers"`
}

// loadWaivers reads the waiver file. A missing file means no waivers, which is
// the normal state and not an error.
func loadWaivers(path string) (map[string]waiver, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]waiver{}, nil
		}
		return nil, err
	}
	var file waiverFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	byCase := map[string]waiver{}
	for _, accepted := range file.Waivers {
		byCase[accepted.CaseID] = accepted
	}
	return byCase, nil
}

// usable reports whether a waiver actually accepts something.
func (w waiver) usable() bool {
	return strings.TrimSpace(w.Owner) != "" && len(strings.TrimSpace(w.Reason)) >= minimumWaiverReason
}

// gateVerdict is the gate's answer.
type gateVerdict struct {
	Passed bool
	Build  string
	Tally  runlog.Tally
	Total  int
	// Failed cases were judged FAIL against this build.
	Failed []string
	// Unanswered cases have no verdict for this build at all.
	Unanswered []string
	// StaleBuild cases were answered, but against a different binary.
	StaleBuild []string
	// Blocked cases could not be judged and were not waived.
	Blocked []string
	// Waived cases are accepted risks, listed so a reader sees what shipped.
	Waived []string
	// UnreproducibleFailures are FAILs whose evidence is not on disk. A red case
	// nobody can reopen has to be fixed from the tester's memory, which is the
	// difference between a defect and an anecdote.
	UnreproducibleFailures []string
	// UnusableWaivers name a case but accept nothing.
	UnusableWaivers []string
	// StaleWaivers cover a case that passed, so they are accepting nothing.
	StaleWaivers []string
}

// judge applies the release rule to one run.
func judge(cases []inventory.Case, log *runlog.Log, waivers map[string]waiver, build string) gateVerdict {
	verdict := gateVerdict{Build: build, Total: len(cases), Tally: runlog.Summarize(cases, log)}
	covered := map[string]bool{}

	for _, c := range cases {
		record, answered := log.Answered(c.ID)
		waiver, waived := waivers[c.ID]
		switch {
		case answered && record.BuildSHA != build && build != "":
			// Answered, but not for this binary. Counting it would let a release
			// inherit last week's verdicts.
			verdict.StaleBuild = append(verdict.StaleBuild, fmt.Sprintf("%s (judged on %s)", c.ID, record.BuildSHA))
		case answered && record.Verdict == runlog.VerdictFail:
			verdict.Failed = append(verdict.Failed, fmt.Sprintf("%s — %s", c.ID, record.Note))
			if reason, ok := bundleMissing(record); ok {
				verdict.UnreproducibleFailures = append(verdict.UnreproducibleFailures,
					fmt.Sprintf("%s — %s", c.ID, reason))
			}
			covered[c.ID] = true
		case answered && record.Verdict == runlog.VerdictBlocked:
			if waived && waiver.usable() {
				verdict.Waived = append(verdict.Waived, waiverLine(waiver))
				covered[c.ID] = true
				continue
			}
			verdict.Blocked = append(verdict.Blocked, fmt.Sprintf("%s — %s", c.ID, record.Note))
		case answered:
			covered[c.ID] = true
		case waived && waiver.usable():
			verdict.Waived = append(verdict.Waived, waiverLine(waiver))
			covered[c.ID] = true
		case waived:
			verdict.UnusableWaivers = append(verdict.UnusableWaivers,
				fmt.Sprintf("%s — a waiver needs an owner and at least %d characters saying what risk is accepted", c.ID, minimumWaiverReason))
			verdict.Unanswered = append(verdict.Unanswered, c.ID)
		default:
			verdict.Unanswered = append(verdict.Unanswered, c.ID)
		}
	}

	verdict.StaleWaivers = staleWaivers(waivers, log, build)
	sortAll(&verdict)
	verdict.Passed = len(verdict.Failed) == 0 && len(verdict.Unanswered) == 0 &&
		len(verdict.StaleBuild) == 0 && len(verdict.Blocked) == 0 && len(verdict.StaleWaivers) == 0
	return verdict
}

// bundleMissing reports why a FAIL cannot be reopened, if it cannot.
//
// A verdict is only useful if someone else can act on it. Checked here rather
// than at capture time because a bundle can be captured and then deleted, and
// the release is the moment that matters.
func bundleMissing(record runlog.Result) (string, bool) {
	if len(record.Evidence) == 0 {
		return "no evidence was captured, so this failure cannot be reopened without the person who found it", true
	}
	var manifests int
	for _, path := range record.Evidence {
		dir := filepath.Dir(path)
		if _, err := os.Stat(filepath.Join(dir, evidence.ManifestName)); err == nil {
			manifests++
			continue
		}
		if _, err := os.Stat(path); err != nil {
			return fmt.Sprintf("the evidence it names is gone (%s)", path), true
		}
	}
	if manifests == 0 {
		return "its evidence has no " + evidence.ManifestName + ", so nothing says which build or question produced it", true
	}
	return "", false
}

// staleWaivers are waivers for cases that passed on this build.
//
// Left in place they accumulate, and a waiver file nobody prunes is one that
// silently covers a case that later starts failing.
func staleWaivers(waivers map[string]waiver, log *runlog.Log, build string) []string {
	var stale []string
	for id, accepted := range waivers {
		record, answered := log.Answered(id)
		if answered && record.Verdict == runlog.VerdictPass && (build == "" || record.BuildSHA == build) {
			stale = append(stale, fmt.Sprintf("%s — passed on this build; delete the waiver (%s)", id, accepted.Owner))
		}
	}
	return stale
}

func waiverLine(w waiver) string {
	return fmt.Sprintf("%s — %s (accepted by %s)", w.CaseID, w.Reason, w.Owner)
}

func sortAll(v *gateVerdict) {
	for _, list := range [][]string{v.Failed, v.Unanswered, v.StaleBuild, v.Blocked, v.Waived,
		v.UnusableWaivers, v.StaleWaivers, v.UnreproducibleFailures} {
		sort.Strings(list)
	}
}

// Report renders the verdict for a human reading a failed release.
func (v gateVerdict) report() string {
	var out strings.Builder
	fmt.Fprintf(&out, "Human UAT gate for build %s\n%s\n", v.Build, runlog.DescribeTally(v.Tally, v.Total))
	section(&out, "FAILED", v.Failed)
	section(&out, "FAILURES NOBODY ELSE CAN REPRODUCE", v.UnreproducibleFailures)
	section(&out, "BLOCKED and not waived", v.Blocked)
	section(&out, "ANSWERED ON A DIFFERENT BUILD", v.StaleBuild)
	section(&out, "NEVER ANSWERED", v.Unanswered)
	section(&out, "WAIVERS THAT ACCEPT NOTHING", v.UnusableWaivers)
	section(&out, "WAIVERS THAT ARE NO LONGER NEEDED", v.StaleWaivers)
	section(&out, "SHIPPING WITH AN ACCEPTED RISK", v.Waived)
	if v.Passed {
		fmt.Fprintf(&out, "\nPASS: every case was judged on this build.\n")
	} else {
		fmt.Fprintf(&out, "\nFAIL: this build has not been judged. Run `make uat-human` against it, or record a waiver naming the accepted risk.\n")
	}
	return out.String()
}

// section prints a list, capped so a first run of 194 unanswered cases does not
// bury the reason underneath it.
func section(out *strings.Builder, title string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(out, "\n%s (%d):\n", title, len(items))
	const shown = 15
	for i, item := range items {
		if i == shown {
			fmt.Fprintf(out, "  … and %d more\n", len(items)-shown)
			break
		}
		fmt.Fprintf(out, "  %s\n", item)
	}
}
