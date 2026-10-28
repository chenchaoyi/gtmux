# Open conversation file links directly and render Markdown

## Why
A report that opens locally returns 404 on a paired phone until a separate publication command runs. Markdown previews show raw syntax and unresolved image paths. The user expects clicking the same conversation link to open the file across devices without publication work.

## What Changes
- Resolve supported local links in verified public conversation text on click, creating the private snapshot internally. Keep fixed file IDs immutable and preserve removal.
- Require exact session/home and Markdown-link evidence before any unregistered source read; reject guests and protected/unsafe files.
- Render Markdown with semantic structure by default, a source toggle and original-file sharing. Resolve bounded same-directory Markdown images/links through the authenticated parent document.

## Surfaces
- terminal: artifact add remains an optional immutable-share command; no input changes.
- menubar: no new native surface; shared server document behavior applies to paired clients.
- phone: local conversation links open directly; native Markdown preview gains formatting and local images.
- iPad: same preview with responsive reading width, tables, source toggle and existing anchored sharing.
- Web: backend preserves scope checks; this change does not add a browser file viewer or OS file URL navigation.
