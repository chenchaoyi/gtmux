package mine

import (
	"regexp"
	"strings"
)

// The correction lexicon. RECALL-FIRST by design: a hit means "a human reacted to what
// the agent just did", not "here is a lesson" — HQ reads every survivor and decides.
// Measured on a week of real logs when this was written, a hit on a typed line that
// follows an assistant reply read as a genuine correction about four times in five once
// the machine's own injections had been subtracted; the misses were relays and pasted
// output, which the subtractions now remove upstream. Keep it bounded; a phrase earns
// its place by catching corrections the others miss, not by sounding negative.
//
// Two shapes count:
//   - a phrase from the lists below (zh or en), anywhere in the line;
//   - emphatic punctuation: a doubled ！/? at the end of a clause.
var (
	zhCorrection = []string{
		"不对", "不是这", "不是那", "应该是", "别这", "不要这", "你没", "怎么又", "还是不",
		"错了", "我说的是", "明明", "测过没", "没测", "为什么还", "太浓", "太重", "不够",
		"不专业", "重新设计", "重新做", "不行", "又复发", "又出现", "不是已经", "我不是要",
	}
	enCorrection = []string{
		"that's wrong", "that is wrong", "not what i", "you didn't", "you did not",
		"you should have", "you broke", "again?", "still broken", "still not", "revert that",
		"i said", "i asked for", "did you even", "why is it still", "that's not it", "wrong file",
	}
	emphaticRe = regexp.MustCompile(`(！！|!!|？？|\?\?)`)
)

// isCorrectionShaped reports whether a typed human line looks like a reaction to the
// agent's previous reply. It does not look at the reply — the caller guarantees one
// preceded this line; a line that opens a task cannot be a correction of nothing.
func isCorrectionShaped(line string) bool {
	if emphaticRe.MatchString(line) {
		return true
	}
	for _, p := range zhCorrection {
		if strings.Contains(line, p) {
			return true
		}
	}
	l := strings.ToLower(line)
	for _, p := range enCorrection {
		if strings.Contains(l, p) {
			return true
		}
	}
	return false
}
