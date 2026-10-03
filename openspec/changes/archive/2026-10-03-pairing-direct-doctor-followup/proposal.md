# Direct pairing and doctor follow-up

## Why

A Direct user ran `gtmux tunnel --status`, which checked only the Standard
LaunchAgent and said always-on was off. Following its suggested `--service`
command then replaced Direct with Standard and changed the pairing address.
Separately, a phone could successfully redeem a single-use pairing code but
discard the issued device token when its subsequent, expensive radar request
timed out. The doctor progress indicator identified the agent section but not
the probe inside it that was slow.

## What changes

- Report the installed Direct or Standard service and preserve that backend when
  repeating `tunnel --service` without an explicit `--backend`.
- Reuse either loaded service for a foreground `tunnel` request.
- Save a v2 device token as soon as enrollment succeeds; the radar fetch is a
  connection concern, not a prerequisite for keeping a consumed pairing code.
- Announce the individual agent/notification doctor probes on stderr.

## Surfaces

| Surface | Change |
| --- | --- |
| CLI / terminal | Tunnel status, implicit backend selection, and doctor progress. |
| phone / iPad | Save a v2 pairing token immediately after enrollment. |
| menubar and web | Existing pairing address reads the selected backend; no view change. |

## Non-goals and limits

This does not prove why the colleague's phone could not reach the Shanghai
route over 5G. A Mac-side health probe only proves the route from that Mac.
The new doctor progress line will identify which agent probe is slow on the
colleague's machine; it does not claim to speed up an unmeasured operation.
