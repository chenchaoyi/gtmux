# Private storage writers

The two managed roots are `state.Dir()` and `state.ConfigDir()` (the app uses
`Paths.dataDir` and `Paths.configDir`). A file's creation mode alone is insufficient:
`WriteFile` keeps an existing inode's mode, chmod ignores umask, and rename keeps
the source mode. Each gtmux writer must apply the policy locally.

| Entry / destination | Expected policy | Evidence / disposition |
|---|---|---|
| CLI attachments, Mac screenshot sends, HTTP uploads → `uploads/` | Private staging and atomic 0600 publication | Fixed by `WritePrivateFile`; `TestUploadWritersKeepPrivateModes`, including 000/022/077 and wide retries |
| `usage.saveDaily` → daily ledger; `radar.BuiltinIconPath` → icon cache | Private atomic publication; private cache hits without churn | Fixed stale fixed-`.tmp` publication and wide identical cache hits; `TestDailyWriterIsLocallyPrivate`, `TestIconWriterIsLocallyPrivateIncludingCacheHits` fail before the fix |
| State markers, CPU/frame samples, tab order, run stamps → data root | Create privately; narrow an owned wide inode before overwrite | `state.WriteFile` / `OpenFile` / `MkdirAll`; `TestMarkerWritersAreLocallyPrivate` fails before the fix |
| Config, device/share/push rosters, remote/tunnel credentials and status → config/data roots | 0600 data, private directory chain; preserve serialization | Routed through policy IO; `TestConfigurationAndCredentialsArePrivateOnRewrite` checks new/wide destinations and JSON |
| Events, sequence counter, advice, mine history, OpenCode mirror, wake tallies → managed journals | Private append; retain inode, O_APPEND, O_EXCL and flock semantics | Policy `OpenFile`; `TestJournalAndSequenceArePrivateBeforeAppend`, `TestPrivateIOKeepsInodesFlagsAndForeignModes`, existing concurrent event tests |
| HQ seeds/backups/rendered knowledge, notification queues, dispatch records → managed roots | Private writes and staging; preserve owner execution on tools | Policy IO; original subsystem tests plus raw-writer inventory guard. Copying bytes does not import source modes; HQ archive restore retains owner execution only |
| Native/resume/session-policy/client stores and their locks → data root | Private staging/overwrite/lock inodes, unchanged records and concurrency | Mechanical policy-IO substitutions; original storage, delivery, scope and lifecycle suites retained |
| Go status/relay/profile/account stores, migration/export staging → managed roots | Existing 0600 exclusive temporaries plus private parents | Their atomic algorithms retained; `state.CreateTemp` / `MkdirTemp` now validate the managed parent chain |
| Swift diagnostic append → data root `logs/` | Private directory chain and inode before append; refuse links | `PrivateLogFile`; `DiagLogTests.testAppendNarrowsExistingModesAndRefusesLinks` fails before the fix (seven assertions) |
| gtmux DSH terminal plugin → terminal clients/input | Exclusive 0600 JSON/claims, owned 0700 directories; refuse unsafe existing roots | Existing atomic/exclusive leaf writes retained; root validation added; synthetic Node contract covers permissive umask and a wide root. Hosted in an agent, but still gtmux code |
| launchd/systemd output capture → managed `logs/` | Prepare 0600 before the service manager opens stdout/stderr | Added bounded preparation when generating a service descriptor; `TestServiceDescriptorsPrepareManagedCapturePrivately` preserves existing output and never starts a service |
| Foreign agent/shell/tmux/Warp configuration; caller-selected exports; executable install | Preserve the external file's existing policy; conventional new modes / executable 0755 where intended | `WriteForeign` and policy IO retain OS semantics outside managed roots. `TestWriteForeignKeepsConventionalModes` and `TestPrivateIOKeepsInodesFlagsAndForeignModes`; installer and service suites |
| Doctor probes / generated source / privileged system guard | Read-only probes must not repair; codegen and root-owned system assets have their own modes | Exact raw-OS call exceptions in `TestRawWritersHaveAnExplicitBoundary`; system guard script/plist intentionally 0755/0644 outside the private roots |
| Agent-written HQ notes/reports; OS preferences/keychain; App image exports and personal system temporaries | Different writer or storage authority | Agent documents are not attributed to gtmux without evidence. Screenshot exports retain their selected target behavior; importing via `--attach` drops the source mode. No blanket home migration |

Policy IO is deliberately small: managed `MkdirAll`, `OpenFile`, `WriteFile`,
`CreateTemp` and `MkdirTemp` share the held-directory/owner checks. Append and
in-place overwrite keep their inode semantics; atomic publishers retain theirs.
New ordinary data files have 0600, new directories 0700, and old owned objects lose
group/other bits before new data is written. In-place narrowing never adds owner
bits; newly published data inodes use 0600.
Outside these roots, the wrappers preserve the previous OS behavior. Managed
symlinks, special files and foreign owners fail explicitly. The source inventory
guard rejects unclassified raw Go writer calls and broad literal chmod additions;
it complements runtime tests, not a semantic proof of every possible program.

On the local APFS filesystem, concurrent raw `O_CREAT|O_NOFOLLOW` opens produced
`ENOENT` despite a held, live parent. Policy open retries only creation's `ENOENT`,
at most eight times against the same fd; other errors are returned immediately.
`TestPrivateCreateFailureBoundaries` confirms deleted-parent and permission failures
remain failures, and existing concurrent sequence tests exercise successful creation.

The guarantee concerns POSIX modes. Inherited ACLs are not attested or removed,
and agent sandbox directory authorization is a separate mechanism. Existing files
not touched by a writer are not bulk migrated. Symlinked managed stores may now
fail instead of following links. No installation, service restart or release is
part of this change.
