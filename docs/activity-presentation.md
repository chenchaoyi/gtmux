# Reading tool activity

[中文](activity-presentation.zh.md)

Activity answers “what did the agent do?” The primary label follows the normalized
operation: Read file, Search, Run command, Run code, Edit file, Browse web, Wait or
Delegate. The original method and provider stay in the inspector alongside the
recorded input, result and status. Unknown tools keep their original name.

A command can be a file read or search even when its transport method is `exec`.
Codex `parsed_cmd` and each agent adapter supply those facts. A mixed or unknown
script remains Run command; seeing a word such as `read` in script text is not
proof of a file operation. A label never proves successful execution.

App, browser Activity and Trace share the typed presentation code. Browser assets
are generated with `node mobileapp/scripts/gen-trace-web.cjs`; edit the canonical
files under `mobileapp/src/ui/` and `mobileapp/src/trace/`, not the generated assets.

Previews can unwrap complete command argv arrays for reading. Parsing is limited
to 8 KiB and 64 string arguments; incomplete records retain a bounded raw summary.
The actual input is unchanged. An Exec reading several files is still one recorded
call. Run code and inner calls remain separate records when that is what the source
contains. The view does not invent parent links or additional per-file calls.

Mouse-first browser rows have a 30px minimum height. Touch browser rows and App
buttons retain a 44px/44pt target. Long summaries ellipsize; full recorded content
remains available by expanding a row, including with the keyboard.

Reviewed cases AOP-001 through AOP-003 cover semantic/raw separation, bounded argv
previews, cross-renderer consistency and density. Synthetic browser and RN tests
are not real-device accessibility certification. Changes to this contract follow
the [test design workflow](test/DESIGN-WORKFLOW.md).

In a maximized browser pane, Restore layout occupies its own header column beside the title and window controls. Narrow panes retain a named return icon. The maximized pane follows container resizing; restoring through the button or existing Escape path returns its previous dimensions. AOP-004 records the layout and restore checks.

AOP-005 updates existing rendered Activity journeys to assert density against actual mouse/touch capabilities while preserving settings, disclosure, content, count and bounds checks. A narrow viewport alone does not imply a touch device.
