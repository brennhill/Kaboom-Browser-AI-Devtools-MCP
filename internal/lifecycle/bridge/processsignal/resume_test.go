// resume_test.go — Verifies safe bridge process-resume signaling.

package processsignal

import "testing"

func TestResumeIgnoresMissingProcess(t *testing.T) {
	Resume(nil)
}
