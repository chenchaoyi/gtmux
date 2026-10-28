# Align browser composition and identity with the app

## Why
The browser composer lacks app input controls, staged attachments, long-form editing and recall. Quote only copies Markdown punctuation and adds an unnecessary action. Writable badges repeat what the composer already makes visible, while avatar identities cannot be inspected.

## What Changes
- Align Send (up arrow), terminal keys, newline/send preference, mobile keyboard controls, expanded editor, attachment staging, paste/drop, image annotation, history and saved phrases.
- Keep drafts and attachment state scoped to panes and credentials, with explicit failures and no submission during attachment selection or history recall.
- Remove Quote and writable badges; retain Copy and read-only explanations.
- Replace the appearance label with a settings icon and add responsive speaker/agent identity sheets.

## Surfaces
- terminal: browser control keys use the existing allowlisted input API; input/upload contracts remain unchanged.
- menubar: no additional native changes; local workspace opens the refined browser.
- phone: mobile browser uses app-style controls and sheets; native code remains unchanged.
- iPad: browser controls and identity presentation follow the app's information hierarchy; native code remains unchanged.
- Web: shared composer, identity sheets and simplified controls across focused conversations and tiles.
