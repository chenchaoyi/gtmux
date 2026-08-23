# Tasks — tmux-server-own-identity

- [x] 1.1 An explicit opt-in on `gtmux restore` for the caller that knows it is an app; the menu-bar app passes it
- [x] 1.2 Start the boot server via launchd on that path only, passing LANG/LC_CTYPE/PATH explicitly
- [x] 1.3 Fall back to today's path on any failure, and on any platform without launchd
- [x] 1.4 Tests: the opt-in is honoured, the fallback is taken when launchd fails, the server is started exactly once either way
- [x] 1.5 Verify on an isolated server: the responsible process changes, AND a CJK pane title still reads back correctly through the radar (the v0.11.3 regression)
- [x] 1.6 TROUBLESHOOTING: the prompt that names the wrong app — symptom, why the name is wrong, how to check (the tccd log query)
- [x] 1.7 Sync specs + archive
