# Give recent managed HQ reports a protected delivery opportunity
## Why
A full decision queue can indefinitely exclude a durable worker report that HQ still needs to read. The rate-limited queue alert lacks the report reference.
## What changes
Use a single guarded native delivery slot for recent, prospective managed-task reports addressed to HQ. Prefer a report with exact source-turn end evidence. Coalesce the slot with eligible native wakes without displacing decision entries. Preserve pending reports and report references independently if HQ does not claim them.
## Surfaces
- terminal: the verified native HQ can receive an exact report reference using the existing input guards.
- menubar: independent attention uses the existing desktop notification channel.
- phone: no direct input or access changes.
- iPad: same server behavior as phone.
- Web: current authorized workspace readers can pull the durable reference.
