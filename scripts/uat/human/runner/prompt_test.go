// prompt_test.go — Proves the person's answer is taken literally and that no
// keystroke pattern produces a pass nobody meant.

package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/scripts/uat/human/inventory"
)

func sampleCase() inventory.Case {
	return inventory.Case{
		ID:       "observe/screenshot",
		Kind:     inventory.KindMCPMode,
		Tool:     "observe",
		Mode:     "screenshot",
		Setup:    "Scroll a long page halfway down.",
		Question: "Does the image show the part of the page that is on screen right now?",
	}
}

func present(t *testing.T, typed string) (humanAnswer, string) {
	t.Helper()
	var out bytes.Buffer
	prompter := newPrompter(strings.NewReader(typed), &out)
	answer, err := prompter.present(1, 1, sampleCase(), callOutcome{Response: []byte(`{"ok":true}`)})
	if err != nil {
		t.Fatal(err)
	}
	return answer, out.String()
}

func TestBlankAnswersNeverBecomeAPass(t *testing.T) {
	t.Parallel()
	// Holding return through the run is the cheapest way to fake a green UAT.
	// Empty input must end the sitting with the case unanswered, not pass it.
	answer, _ := present(t, "\n\n\n")
	if answer.Verdict == verdictPass {
		t.Fatal("an empty line passed a case nobody judged")
	}
	if !answer.Quit {
		t.Errorf("input ran out and the run did not stop: %+v", answer)
	}
}

func TestEachAnswerIsTakenLiterally(t *testing.T) {
	t.Parallel()
	for typed, want := range map[string]verdict{
		"p\n":             verdictPass,
		"pass\n":          verdictPass,
		"F\nbroke\n":      verdictFail,
		"b\nno browser\n": verdictBlocked,
		"s\n":             verdictSkipped,
	} {
		answer, _ := present(t, typed)
		if answer.Verdict != want {
			t.Errorf("%q => %q, want %q", typed, answer.Verdict, want)
		}
	}
	// Control: an answer outside the vocabulary is not silently mapped to one.
	if verdict, ok := parseVerdict("probably fine"); ok {
		t.Errorf("%q was accepted as %q", "probably fine", verdict)
	}
}

func TestAFailWithoutANoteIsRefused(t *testing.T) {
	t.Parallel()
	// The note is what a regression test gets written from. Accepting a bare FAIL
	// leaves a red case nobody can act on.
	answer, transcript := present(t, "f\n\n\nit captured the wrong tab\n")
	if answer.Verdict != verdictFail {
		t.Fatalf("verdict = %q", answer.Verdict)
	}
	if answer.Note != "it captured the wrong tab" {
		t.Errorf("note = %q, want what the tester typed after the empty lines", answer.Note)
	}
	if !strings.Contains(transcript, "required") {
		t.Error("the tester was not told the note is required")
	}
}

func TestAPassMayCarryNoNote(t *testing.T) {
	t.Parallel()
	answer, _ := present(t, "p\n\n")
	if answer.Verdict != verdictPass || answer.Note != "" {
		t.Errorf("answer = %+v, want a pass with no note", answer)
	}
}

func TestTheQuestionAndTheResponseAreBothShown(t *testing.T) {
	t.Parallel()
	_, transcript := present(t, "p\n\n")

	// The case is judged from the question and what came back. A prompt missing
	// either turns the rig back into a reachability sweep.
	if !strings.Contains(transcript, sampleCase().Question) {
		t.Error("the question was not shown")
	}
	if !strings.Contains(transcript, `{"ok":true}`) {
		t.Error("the tool response was not shown")
	}
	if !strings.Contains(transcript, sampleCase().Setup) {
		t.Error("the setup step was not shown, so the tester has nothing to look at")
	}
}

func TestAHugeResponseIsTruncatedSoTheQuestionStaysVisible(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	prompter := newPrompter(strings.NewReader("p\n\n"), &out)
	huge := append([]byte(`{"data":"`), bytes.Repeat([]byte("x"), 20000)...)
	if _, err := prompter.present(1, 1, sampleCase(), callOutcome{Response: huge}); err != nil {
		t.Fatal(err)
	}
	if out.Len() > 12000 {
		t.Errorf("the prompt printed %d bytes; the question scrolls off the screen", out.Len())
	}
	if !strings.Contains(out.String(), "full response is in the run log") {
		t.Error("the truncation does not say where the rest went")
	}
}

func TestACallErrorIsShownAndStillJudged(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	prompter := newPrompter(strings.NewReader("f\nthe tool refused\n"), &out)
	answer, err := prompter.present(1, 1, sampleCase(), callOutcome{Err: "server error -32603: no tracked tab"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "no tracked tab") {
		t.Error("the error was hidden from the person asked to judge it")
	}
	// The runner does not decide: some cases are about what happens when the
	// tool cannot do the thing.
	if answer.Verdict != verdictFail {
		t.Errorf("verdict = %q, want the tester's own answer", answer.Verdict)
	}
}
