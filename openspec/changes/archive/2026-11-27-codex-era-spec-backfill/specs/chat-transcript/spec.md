## MODIFIED Requirements

### Requirement: Recorded Activity action facts

Optional action metadata SHALL distinguish a recorded skill read, web search, page open and page find across all supported agent adapters and shared App/browser/Trace presenters. Distinct file target counts SHALL describe one invocation without increasing real call counts. Explicit methods, complete known web argument structures and canonical recorded skill-read paths MAY supply these facts; shell text, results, arbitrary filenames and incomplete or mixed operations SHALL NOT invent them. For native `dsh` and `opencode` `skill` tool invocations only, the parser MAY also recognize `name` or `id` alongside `skill` when all present non-empty values agree, and SHALL reject empty or conflicting values. For native `dsh` `web_search` invocations lacking a scalar `query`, the parser MAY format a comma-separated preview of up to 3 items from a non-empty `queries` list bounded to at most 64 string entries. Original input, scoped references, recorded status and bounded retention SHALL remain intact; unknown or old payloads retain neutral fallback.

#### Scenario: One native invocation reads three files
- **WHEN** a native record with the same exact call identity adds three parsed read targets to an existing generic call
- **THEN** the row shows one call with three file targets and retains its original input and scoped reference
- **AND** App, browser and Trace use the same factual action label without claiming success

#### Scenario: Native DSH or OpenCode skill tool uses `name` or `id`
- **WHEN** a `dsh` or `opencode` `skill` tool call supplies a non-empty `name` or `id` (and no conflicting `skill`/`name`/`id` values)
- **THEN** the activity step records action `skill` with that skill identity and subject count 1
- **AND** non-`dsh`/non-`opencode` adapters do not interpret `name` or `id` as skill aliases, and conflicting values produce no skill action

#### Scenario: Native DSH web search supplies a bounded `queries` array
- **WHEN** a `dsh` `web_search` call omits `query` and supplies between 1 and 64 non-empty string entries in `queries`
- **THEN** the activity step records action `web_search` with a preview of up to the first 3 queries
- **AND** an empty list, a list exceeding 64 entries, or any empty/non-string entry leaves the search detail unset
