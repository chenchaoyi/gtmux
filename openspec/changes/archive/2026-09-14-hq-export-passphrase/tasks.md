# Tasks — hq-export-passphrase

- [x] 1.1 `ExportMemoryEncrypted` / `ImportMemoryEncrypted` (age, scrypt passphrase), `IsEncryptedArchive`, strength ladder, last-export record; tests (round trip, wrong passphrase changes nothing, short refused, plain still plain, stdin line)
- [x] 1.2 `gtmux hq --export` / `--import`: passphrase from stdin flag, env, or an unechoed terminal prompt with confirmation; `--plain`; usage en/zh; the memory line's last-export tail
- [x] 2.1 Menu bar: export sheet (passphrase, confirm, show, hint, keychain remember, save panel after), confirmation page, failure page with the CLI's words; passphrase over stdin
- [x] 2.2 Reader memory line shows the last export and whether it was locked
- [x] 3.1 docs/cli pair; DESIGN pair §12; specs synced; change archived
- [x] 3.2 Gates green; design canvas of the sheet states
