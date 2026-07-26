// Purpose: Provides stack/message normalization and similarity helpers used by error clustering logic.
// Why: Keeps clustering matches stable by centralizing fuzzy-match signal extraction rules.
// Docs: docs/features/feature/error-clustering/index.md

package clustering

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// StackFrame represents a parsed stack trace frame.
type StackFrame struct {
	Function    string
	File        string
	Line        int
	Column      int
	IsFramework bool
}

var frameworkPatterns = []string{
	"node_modules/react",
	"node_modules/vue",
	"node_modules/@angular",
	"node_modules/svelte",
	"webpack/bootstrap",
	"webpack/runtime",
	"zone.js",
	"node_modules/rxjs",
	"node_modules/core-js",
}

var (
	stackFrameWithFunc = regexp.MustCompile(`^\s*at\s+(.+?)\s+\((.+?):(\d+):(\d+)\)`)
	stackFrameAnon     = regexp.MustCompile(`^\s*at\s+(.+?):(\d+):(\d+)\s*$`)
)

func parseStackFrame(line string) StackFrame {
	line = strings.TrimSpace(line)

	if m := stackFrameWithFunc.FindStringSubmatch(line); m != nil {
		lineNum, _ := strconv.Atoi(m[3])
		colNum, _ := strconv.Atoi(m[4])
		frame := StackFrame{
			Function: m[1],
			File:     m[2],
			Line:     lineNum,
			Column:   colNum,
		}
		frame.IsFramework = isFrameworkPath(frame.File)
		return frame
	}

	if m := stackFrameAnon.FindStringSubmatch(line); m != nil {
		lineNum, _ := strconv.Atoi(m[2])
		colNum, _ := strconv.Atoi(m[3])
		frame := StackFrame{
			Function: "<anonymous>",
			File:     m[1],
			Line:     lineNum,
			Column:   colNum,
		}
		frame.IsFramework = isFrameworkPath(frame.File)
		return frame
	}

	return StackFrame{Function: "<unknown>", File: line}
}

func isFrameworkPath(path string) bool {
	for _, pattern := range frameworkPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
}

func parseStack(stack string) []StackFrame {
	if stack == "" {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(stack), "\n")
	frames := make([]StackFrame, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		frames = append(frames, parseStackFrame(line))
	}
	return frames
}

func appFrames(frames []StackFrame) []StackFrame {
	result := make([]StackFrame, 0)
	for _, f := range frames {
		if !f.IsFramework {
			result = append(result, f)
		}
	}
	return result
}

func countSharedFrames(a, b []StackFrame) int {
	bSet := make(map[string]bool)
	for _, f := range b {
		bSet[fmt.Sprintf("%s:%d", f.File, f.Line)] = true
	}
	count := 0
	for _, f := range a {
		if bSet[fmt.Sprintf("%s:%d", f.File, f.Line)] {
			count++
		}
	}
	return count
}

func findCommonFrames(a, b []StackFrame) []StackFrame {
	bSet := make(map[string]StackFrame)
	for _, f := range b {
		key := fmt.Sprintf("%s:%d", f.File, f.Line)
		bSet[key] = f
	}
	var common []StackFrame
	for _, f := range a {
		key := fmt.Sprintf("%s:%d", f.File, f.Line)
		if _, ok := bSet[key]; ok {
			common = append(common, f)
		}
	}
	return common
}

func collectAffectedFiles(a, b []StackFrame) []string {
	seen := make(map[string]bool)
	var files []string
	for _, frames := range [][]StackFrame{a, b} {
		for _, f := range frames {
			if f.File != "" && !f.IsFramework && !seen[f.File] {
				seen[f.File] = true
				files = append(files, f.File)
			}
		}
	}
	return files
}
