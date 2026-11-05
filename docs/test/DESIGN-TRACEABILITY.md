# Design traceability

[中文](DESIGN-TRACEABILITY.zh.md) · [Current layers](TEST-PLAN.md) · [Case catalogue](CASES.md)

This is a map of design requirements to implementation and test entry points, not
an acceptance report. The detailed historical section/known-gap matrix is retained
in the Chinese counterpart; this English overview preserves its statuses and scope.
The new catalogue and reviewed design records are the current test-maintenance entry.

| Design requirement | Implementation / test entry | Coverage and remaining acceptance |
| --- | --- | --- |
| Status model, color/rank and tokens (§1/9) | AgentStore, Theme, StatusBadge; status-color/rank tests and design guard | Logic covered; visual triple encoding still requires acceptance |
| Status item (§2) | StatusItemGlyph pane grid, AppDelegate count/display modes | Color-only grid remains a known deviation from non-color encoding; light/dark/tinted and notch acceptance separate |
| Popover / quick switcher (§3/4) | MenuView, CommandPalette, fuzzy/group/age/navigation tests | Logic present; real layout/material/keyboard/focus still requires Mac acceptance |
| Empty state / first-run (§5) | States views | Views exist; first-jump permission detection/card timing remains incomplete |
| Agent identity / logo (§6/12) | AgentIcons, avatars, Go openable icon paths, decoding/monogram tests | Loading implemented; source-specific fallback and visuals separately accepted |
| tmux / desktop-native (§7) | Hook/native radar contract, app decoding | tmux panes support view/focus/input; pane-less desktop-native rows cannot be sent to or focused directly; transfer-to-tmux is a separate path |
| Preferences (§8) | Preferences/AppSettings | Immediate language/display/notification behavior needs actual UI acceptance; configurable global shortcut remains unimplemented (fixed shortcut shown) |
| Motion (§10) | StatusBadge/MenuVisibility, teardown tests | Working ring has visibility/Reduce Motion branches, 2s period; replacement/CPU not proven by flag tests. Idle-to-waiting pulse remains unimplemented |
| Accessibility / language (§11) | L10n and per-control role/label tests | Some HQ roles exist; whole agent-row VoiceOver labels/hints remain incomplete. CJK, spoken output and non-color encoding require real acceptance |
| State/boundary matrix (§13) | Group/count tests, surface scenarios | Partial; 0/1/5/15+, long/CJK/native/switching require explicit evidence |
| JSON contract (not current §14) | agentJSON / Agent decode tests | Logic coverage does not resolve repeated design section numbers |
| HQ navigation, copy and stale history | HQWorkspaceTests, HQChatPresentationTests, HQHeader/HQScreen components | Injected clients/clipboard tests; not actual desktop focus or physical clipboard proof |
| Knowledge search and iPad HQ layout | KnowledgeSheet, knowledge-search Appium, layout/HQScreen tests | First tap with keyboard, sidebar/rotation, narrow inspector, preserved drafts require actual phone/iPad operation |
| HQ keyboard and send recovery | useKeyboardChrome, HQScreen.poll, hq-keyboard, isolated send API | Actual landscape/large type/hardware keyboard and Android behavior separately recorded; never send tests into real work sessions |
| Phone footer spacing / preview and copy | footerSpacing, UsageSheet, KnowledgeSheet and document/Trace suites | Source/component coverage is not installed-device visual acceptance |

Section mapping remains incomplete for dated §9 rules (unfocusable rows and slow
operations), HQ/Preferences-sharing additions, duplicate §14/16 headings, server
mode §17 and screenshot-to-agent §18. Do not mark the design fully followed from
this table. The Chinese matrix retains the original detailed paths and exceptions;
update both languages when changing these requirements.

For a new requirement: link the relevant OpenSpec scenario, write/review stable test
cases in `designs/` before implementation, then map executable selectors and actual
results. Separate implementation completeness, automated coverage and device/visual
acceptance; do not turn an unimplemented requirement into a green label.
