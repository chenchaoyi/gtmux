package hqwake

import "testing"

// Claude Code 2.1.277 records a pasted wake batch wrapped in pasted_content tags. The
// batch id is the driver receipt the delivery ack matches first; with the wrapper it
// stopped being found and every ack fell back to reading the screen.
func TestBatchIDSurvivesClaudesPastedContentWrapper(t *testing.T) {
	wrapped := "\n\n<pasted_content id=\"676c\">\n» ◆ gtmux·goal-changed  web:0.0 (%20) │ goal:\"fix it\" · #81a11d\n</pasted_content id=\"676c\">\n"
	if got := BatchID(wrapped); got != "#81a11d" {
		t.Errorf("BatchID of a wrapped batch = %q, want #81a11d", got)
	}
}
