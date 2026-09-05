// prompt.go — Presents one case and takes the person's answer.
//
// The answer is the result. Nothing here inspects the tool response to form an
// opinion: the rig exists because "the response is not an MCP error" is not an
// assertion, and a runner that guessed would reintroduce exactly that.

package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/scripts/uat/human/inventory"
)

// humanAnswer is what the person said.
type humanAnswer struct {
	Verdict verdict
	Note    string
	// Quit ends the run. Everything already answered is on disk.
	Quit bool
}

// parseVerdict maps what the person typed to a verdict.
//
// Only the four listed answers and their initials are accepted. A blank line is
// NOT a pass: defaulting to PASS on an empty return is how a tester holding the
// enter key would sign off on 194 cases without looking at one.
func parseVerdict(typed string) (verdict, bool) {
	switch strings.ToLower(strings.TrimSpace(typed)) {
	case "p", "pass":
		return verdictPass, true
	case "f", "fail":
		return verdictFail, true
	case "b", "blocked":
		return verdictBlocked, true
	case "s", "skip", "skipped":
		return verdictSkipped, true
	}
	return "", false
}

// isQuit reports whether the person asked to stop.
func isQuit(typed string) bool {
	switch strings.ToLower(strings.TrimSpace(typed)) {
	case "q", "quit", "exit":
		return true
	}
	return false
}

// prompter reads answers from a terminal.
type prompter struct {
	in  *bufio.Reader
	out io.Writer
}

// newPrompter builds a prompter over any streams, so the interaction is testable
// without a terminal.
func newPrompter(in io.Reader, out io.Writer) *prompter {
	return &prompter{in: bufio.NewReader(in), out: out}
}

// present prints the case and returns the person's answer.
func (p *prompter) present(index, total int, c inventory.Case, outcome callOutcome) (humanAnswer, error) {
	fmt.Fprintf(p.out, "\n────────────────────────────────────────────────────────\n")
	fmt.Fprintf(p.out, "[%d/%d] %s\n\n", index, total, c.ID)
	fmt.Fprintf(p.out, "SET UP:   %s\n", c.Setup)
	fmt.Fprintf(p.out, "QUESTION: %s\n\n", c.Question)
	p.printOutcome(outcome)
	return p.ask(c)
}

// printOutcome shows what the tool actually returned, including the failure.
//
// A call that errored is still presented for judgement rather than auto-failed:
// several cases are about what the tool does when it cannot do the thing, and
// the person is the one who knows which.
func (p *prompter) printOutcome(outcome callOutcome) {
	if outcome.Request != nil {
		fmt.Fprintf(p.out, "SENT:     %s\n", string(outcome.Request))
	}
	if outcome.Err != "" {
		fmt.Fprintf(p.out, "CALL ERROR: %s\n", outcome.Err)
	}
	if len(outcome.Response) > 0 {
		fmt.Fprintf(p.out, "RESPONSE: %s\n", truncate(string(outcome.Response), 4000))
	}
	for _, path := range outcome.Evidence {
		fmt.Fprintf(p.out, "EVIDENCE: %s\n", path)
	}
}

// ask loops until the person gives one of the accepted answers.
func (p *prompter) ask(c inventory.Case) (humanAnswer, error) {
	for {
		fmt.Fprintf(p.out, "\n%s? [p]ass / [f]ail / [b]locked / [s]kip / [q]uit: ", c.ID)
		typed, err := p.in.ReadString('\n')
		if err != nil && strings.TrimSpace(typed) == "" {
			// End of input with nothing typed: the session is over, and an
			// unanswered case must stay unanswered.
			return humanAnswer{Quit: true}, nil
		}
		if isQuit(typed) {
			return humanAnswer{Quit: true}, nil
		}
		verdict, ok := parseVerdict(typed)
		if !ok {
			fmt.Fprintf(p.out, "  Answer one of: pass, fail, blocked, skip, quit.\n")
			continue
		}
		note, err := p.readNote(verdict)
		if err != nil {
			return humanAnswer{}, err
		}
		return humanAnswer{Verdict: verdict, Note: note}, nil
	}
}

// readNote takes the free-text note, required for anything that is not a pass.
//
// A FAIL with no note cannot be turned into a regression test later, and a
// BLOCKED with no note cannot be unblocked by anyone but the person who hit it.
func (p *prompter) readNote(verdict verdict) (string, error) {
	required := verdict == verdictFail || verdict == verdictBlocked
	for {
		if required {
			fmt.Fprintf(p.out, "  What did you see? (required): ")
		} else {
			fmt.Fprintf(p.out, "  Note (optional, enter to skip): ")
		}
		typed, err := p.in.ReadString('\n')
		note := strings.TrimSpace(typed)
		if note != "" {
			return note, nil
		}
		if !required {
			return "", nil
		}
		if err != nil {
			return "", fmt.Errorf("a %s needs a note and input ended", verdict)
		}
	}
}

// truncate keeps a huge response from burying the question above it.
func truncate(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	return text[:limit] + fmt.Sprintf("\n… [%d more bytes; full response is in the run log]", len(text)-limit)
}
