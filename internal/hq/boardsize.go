package hq

// How long a situation-board cell may run.
//
// The board is read by a person, on a phone, to answer "what is happening now". A cell
// that has become an investigation log does not answer that: measured on the machine this
// was written for, one cell held ~1,180 characters of semicolon-joined findings inside a
// 555-line board. What the operator saw on the phone was a wall of text (2026-09-08).
//
// The charter now says a cell carries a CONCLUSION and a pointer — the working is what the
// knowledge base and `gtmux events` are for. This is how that rule becomes visible: a rule
// nothing measures is a rule nobody keeps, which this repo has paid for before.

import (
	"os"
	"strings"
)

// BoardCellBudget is the length past which a board cell has stopped being a board cell.
//
// Roughly a phone screen of Chinese text. Generous on purpose: the point is to catch an
// essay, never to nag about a full sentence.
const BoardCellBudget = 400

// OversizeBoardCells reports how many board lines run past the budget, and the longest.
//
// Counted in RUNES, not bytes: the board is usually Chinese, where a byte count would
// flag an ordinary sentence. Fenced code blocks are skipped — a pasted capture is not
// prose and shortening it would lose the evidence.
func OversizeBoardCells() (count, longest int) {
	b, err := os.ReadFile(BoardPath())
	if err != nil {
		return 0, 0
	}
	inFence := false
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if n := len([]rune(line)); n > BoardCellBudget {
			count++
			if n > longest {
				longest = n
			}
		}
	}
	return count, longest
}
