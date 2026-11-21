# Recover a managed Codex frontend after a misplaced resume index

## Why
A short heartbeat interruption can let a legacy writer replace the location's index even while the managed bridge remains alive. That bridge subsequently rejects the mismatched index, leaving status and completion detached from the pane.

## What Changes
Keep a managed index reserved for its owner until the receipt is retired or its process incarnation has ended. Let the running, strictly verified publisher repair a legacy index when independent binding receipts place the two threads in distinct panes. Preserve the misplaced index as private history. Unproven conflicts remain refused and are identified in heartbeat diagnostics.

## Surfaces
- terminal: the original bridge restores the exact pane association without restarting or replaying input.
- menubar: the shared radar data regains the managed pane's identity and lifecycle tracking.
- phone: existing server association and input checks use the recovered identity.
- iPad: uses the same shared server association as phone.
- Web: uses the shared server association with the existing access checks.
