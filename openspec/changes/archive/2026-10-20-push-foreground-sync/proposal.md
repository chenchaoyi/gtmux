## Why
Push registration survives app suspension as a foreground timer. A four-second deadline can then be observed many minutes later and reported as a network timeout, while queued alternate routes start after the app has already left the foreground.

## What Changes
Cancel an in-flight push sync when the app becomes inactive/backgrounded, stop alternate attempts, and leave sync pending for the existing foreground retry. Keep real foreground deadlines and HTTP failures observable.

## Surfaces
Terminal, menu bar and Web are unchanged: this is phone lifecycle handling. Phone and iPad share the same push queue, cancellation and foreground retry. No push permission, endpoint, pairing or access-policy changes.
