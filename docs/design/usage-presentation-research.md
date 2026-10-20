# Usage presentation: capacity and recorded output

**English** · [中文](usage-presentation-research.zh.md)

Research checked on 2026-10-20. The trigger was a phone Usage screenshot: a PLAN
section repeated agent names later listed below the token heatmap, and a missing
DeepSeek balance appeared as a technical note between the two. The numbers have
different meanings, but the presentation made them look interchangeable.

## Comparable products

| Primary source | Relevant approach | Application here |
| --- | --- | --- |
| [CodexBar provider guide](https://github.com/steipete/CodexBar/blob/main/docs/providers.md#usage--spend-settings) | Separates provider quota from local usage/spend history; omits unsupported spend sources and keeps native currencies separate. | Keep capacity and output history distinct. Do not synthesize an empty subscription or add balances across currencies. |
| [OpenRouter Activity](https://openrouter.ai/blog/announcements/activity-dashboard/) | Starts with aggregate metrics, followed by charts and grouped breakdowns of the same data. | Put the per-agent output table inside the token-history block, with shared Today / Last 7 days columns. |
| [Langfuse usage and cost tracking](https://langfuse.com/docs/observability/features/token-and-cost-tracking) | Distinguishes usage units from cost and identifies ingested versus inferred values. | Label output tokens explicitly and state their local-record coverage; add no price estimate or billing claim. |

This is a judgment about fit for gtmux, not a universal ranking or a hands-on test
of private dashboards. The comparison uses the products' public documentation and
published UI descriptions. No account, cookie or conversation data was shared.

## Decision

The phone and Mac use the same information order:

1. **Limits & balances**: provider-reported quota percentages/reset windows and
   exact account balances. Preserve each provider's units and reset period.
2. **Output tokens**: aggregate local recorded output, the existing activity
   charts, then a compact By agent table with shared Today / Last 7 days headers.
3. **Sessions**: the existing trouble-ranked session detail. Machine resources
   retain their existing surface placement.

The History & sources disclosure lives directly below the capacity it explains.
It starts closed. It holds historical quota bars, unreadable-provider reasons,
account-balance limitations, recorded plan identifiers and observation times.
Historical values remain neutral and inspectable, including when an agent has
both current and historical windows. The core's stale flag remains authoritative;
an older report without that flag retains its prior meaning.

If there is no readable capacity, a short unavailable summary replaces the
current rows. Missing balance data is never a zero. A supported provider with
only local output contributes to output history independently of its account
source. Current balance strings and currencies stay exact and are not summed.

Source timestamps remain available in the disclosure; current quota reset times
stay beside their bars. The main explanation says provider-reported capacity;
the output block says conversations recorded by the connected machine, by local day. Local
output is neither a bill nor a conversion to provider quota percentage. It does
not imply visibility into every device on an account.

## Scope and verification

Installation, historical conversations and a saved plan label do not verify an
active subscription. Claude Code's official [`auth status`](https://code.claude.com/docs/en/cli-reference)
reports authentication, not a billing cancellation; Claude Code also supports
[API and cloud-provider authentication](https://code.claude.com/docs/en/authentication).
Even cancellation can retain paid access until the
[current billing period ends](https://support.claude.com/en/articles/8325617-cancel-your-pro-or-max-subscription).
This patch therefore hides stale quota readings by default without labelling an
account unsubscribed, deleting conversations or starting a new account probe.
Current entitlement would require fresh account evidence from that provider.

This is a presentation change in the existing React Native and SwiftUI readers,
with small view/data helpers. It adds no endpoint, provider, login, credential
reader, estimated price, polling loop or background scan. Current warnings and tightest-window selection retain their existing rules.
A lone ordinary quota does not repeat as a lead summary; warned/full windows and
machine warnings remain prominent. Exact-money rendering,
mixed fresh/stale groups and disclosure interaction have focused regressions.
Native previews use synthetic fixtures. Simulator UI captures, if supplied,
exercise current JavaScript in a reused Simulator host; they are not a new
signed release or physical-device/VoiceOver acceptance. CI and author tests are
not an independent review.

The core `internal/usage/daily.go` sums today and the preceding six host-local
dates into the legacy `week_out` field. This is a rolling seven-date window, not
a calendar week or a provider billing/reset period. Both readers and their Usage
entry summaries label it Last 7 days / 7d; the wire field and values are unchanged.
The chart's weekly mode continues to mean calendar-week buckets.
