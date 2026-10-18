# DeepSeek Harness Web conversations

Gtmux discovers saved user conversations under `$DSH_HOME/sessions` (default
`~/.dsh/sessions`). Open **External conversations** on the menu bar, phone, iPad
or Web to read prompts, public assistant text and tool steps. CLI users can run
`gtmux agents --json`, then `gtmux transcript SESSION_ID --home-id HOME_ID --json`
using the row's `session_id` and `home_id`. Continue interacting in DSH Web.
Reading history does not grant HQ follow, notifications or knowledge retention.
Guests cannot read or change these external conversations.

The adapter targets the current **v4** JSONL format shipped by DSH **0.2.0-rc.2**,
including concatenated checksummed Zstandard frames. It chooses the highest saved
generation and refuses unknown generations, corrupt records, duplicate identities
or two encodings of the same generation. Earlier generations are not migrated or
silently selected. A partially written final frame waits for the next poll.
Discovery lists the 128 most recently changed root conversations; an exact ID read
can still open an older one. Child agents and conversations without user messages
are excluded. Views keep a bounded recent window and report omitted turns.

This is a public append history, including earlier user dialogue before model
compaction; it is not DSH's reconstructed model context. Injected instructions,
compaction checkpoints, hidden reasoning, replay metadata and raw tool results
are excluded. No credentials file, writer lease or DSH HTTP endpoint is needed.
Saved activity is not proof of a currently connected Web page. The controlling
client remains unconfirmed; gtmux does not resume, adopt or send input to DSH.
TUI control and usage/context figures are not supported in this integration.

## Native lifecycle plugin

For future activity, install gtmux's dedicated **Cordis plugin**:

```sh
gtmux install-hooks --agent dsh
```

The command prints `dsh web --patch '…/gtmux/gtmux.cordis.yml'`. Add that overlay
to your next intentional Web launch. The currently running server is not restarted
or reconfigured. Existing conversation reading works before installing the plugin.
The installer owns only `$DSH_HOME/gtmux/gtmux.mjs` and `gtmux.cordis.yml`; it
preserves profile patches and other plugins. `gtmux uninstall-hooks --agent dsh`
removes those two owned files. Omit the overlay on later launches after removal.

The plugin uses DSH's native `session/event` and `user-questions/request` APIs,
not its Codex/Claude compatibility hooks. It observes root prompts, turn start,
success, interruption and failure. It delegates every question unchanged and
clears a pending question when that request settles. It never answers a question
or approves a tool. `approval/asked` is deliberately not a human-wait signal:
automatic policy/plugins may handle it. Human approval-card detection is not yet
available through a confirmed native Host event; approvals stay in DSH Web.
Subprocesses use piped stdin, a bounded queue and a timeout, so the plugin cannot
steal terminal input or block DSH on an unavailable CLI.

Enable **HQ follow** per conversation before HQ may observe new activity. Enable
**Conversation notifications** separately for completion/question notices and
**Save to knowledge** separately for future reusable lessons. Disabling HQ clears
both dependent grants. Re-enabling knowledge starts a new time boundary and does
not mine the old conversation. Grants belong to the agent, canonical data home
and session together; a copied ID in another home cannot inherit them. Settings
use revision checks so another device's edit is not overwritten silently.
Lifecycle feed updates require the native plugin; saved-history polling alone
reads metadata and does not replay old events into HQ.

The whale mark comes from [DeepSeek Harness's official favicon](https://github.com/deepseek-ai/deepseek-harness/blob/dsh-v0.2.0-rc.2/apps/web/public/favicon.svg).
Source and checksum are recorded in [icon provenance](../../assets/agent-icons/SOURCES.md).
Native contracts: [session types](https://github.com/deepseek-ai/deepseek-harness/blob/dsh-v0.2.0-rc.2/packages/core/session/src/types.ts),
[session observer](https://github.com/deepseek-ai/deepseek-harness/blob/dsh-v0.2.0-rc.2/packages/session/session-telemetry/src/coordinator.ts),
[approval service](https://github.com/deepseek-ai/deepseek-harness/blob/dsh-v0.2.0-rc.2/packages/interaction/user-approval/src/index.ts).

The first discovery builds metadata by walking selected artifacts once; subsequent unchanged polls reuse it, and append reads decode only new complete frames.
