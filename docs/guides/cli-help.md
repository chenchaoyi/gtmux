# Command help

[中文](cli-help.zh.md)

`gtmux --help` groups common commands by purpose: status and usage, sessions and terminals, HQ, remote access, setup, and screenshots. Less frequently used commands remain under **ALSO**.

- `gtmux <command> --help` shows that command's complete options.
- `gtmux --help --json` returns the structured command catalogue for scripts and agents.
- `gtmux --lang=zh --help` selects Chinese; use `--lang=en` for English.

Long command syntax appears above its description so flags are neither truncated nor joined to prose. Narrow terminals use stacked rows.

Screenshot CLI support shipped in 1.0.175:

```sh
gtmux screenshot --help
gtmux screenshot capabilities
```

The first command explains the workflow in the selected language. The second checks the running Mac App without requesting screen-recording permission. Capture, annotation and delivery are separate operations; capture never sends implicitly. Resolve the conversation with `target` before explicitly delivering with `send`.

See the [agent screenshot guide](agent-screenshots.md) for complete examples. These help changes do not alter accepted command arguments, access permissions or delivery behavior.
