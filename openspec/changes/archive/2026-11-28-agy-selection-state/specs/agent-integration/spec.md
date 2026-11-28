## ADDED Requirements

### Requirement: Screen-rendered approval menus are recognized for Antigravity sessions
For hook-equipped agents such as Antigravity (`agy`) that emit no permission-wait hook, gtmux SHALL detect live interactive numbered menus (`> 1. …` accompanied by navigation chrome) from the pane screen, join soft-wrapped option continuations, report `status: "waiting"` with replyable options immediately across radar, digest, HQ, and server interaction endpoints without false-positiving on Markdown blockquotes, and restore `working` or `idle` once the menu clears.

#### Scenario: Antigravity pauses on a wrapped command approval prompt
- **WHEN** an active `agy` pane renders `Run this command?` with `> 1. Yes, run command` and soft-wrapped command-rule options above `↑/↓ Navigate`
- **THEN** `GatherAgents`, `GatherDigest`, `/api/interaction`, and `/api/options` report `waiting` (`permission`) and return all complete numbered options for tap-to-reply

#### Scenario: Markdown blockquote in prose does not trigger waiting
- **WHEN** an `agy` pane renders `> 1. First` and `> 2. Second` in prose without interactive navigation chrome or above an active composer
- **THEN** `WaitingOptions` returns `nil` and the pane does not enter `waiting`
