package transcript

import (
	"strings"
)

// IDs correlate actual records, not commands with similar text. Private IDs are
// retained with the cached steps so incremental refresh keeps the same behavior.
func (st *parseState) addRecordedStep(provider string, step Step, id string) {
	st.ensure()
	if len(id) > 256 {
		id = ""
	}
	if id != "" {
		name := step.Title
		if i := strings.LastIndex(name, "__"); i >= 0 {
			name = name[i+2:]
		}
		if i := strings.LastIndex(name, "."); i >= 0 {
			name = name[i+1:]
		}
		id = provider + "\x00" + id + "\x00" + codexToolName(name)
		if step.Operation == "code" {
			id += "\x00code"
		}
		if st.recordedSeen == nil {
			st.recordedSeen = make(map[string]bool)
			for _, segment := range st.cur.Segments {
				for _, old := range segment.Steps {
					if old.recordedCallID != "" {
						st.recordedSeen[old.recordedCallID] = true
					}
				}
			}
		}
		if st.recordedSeen[id] {
			if step.Call != nil && st.sourceID != "" {
				step.Call.Ref = toolInputRef(st.sourceID, st.off, 0, step)
				for si := range st.cur.Segments {
					for ti := range st.cur.Segments[si].Steps {
						old := &st.cur.Segments[si].Steps[ti]
						if old.recordedCallID == id && (old.Call == nil || !old.Call.HasResult || step.Call.HasResult) {
							old.Call = step.Call
						}
					}
				}
			}
			// A native command may add a recorded read/search classification to
			// an earlier generic shell call. Keep its original full-input ref.
			if step.Operation == "read" || step.Operation == "list" || step.Operation == "search" {
				for si := range st.cur.Segments {
					for ti := range st.cur.Segments[si].Steps {
						old := &st.cur.Segments[si].Steps[ti]
						if old.recordedCallID == id && old.Operation == "command" {
							old.Operation, old.Detail = step.Operation, step.Detail
						}
					}
				}
			}
			return
		}
		st.recordedSeen[id] = true
		step.recordedCallID = id
	}
	st.addSteps([]Step{step})
}
