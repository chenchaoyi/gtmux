## ADDED Requirements

### Requirement: Bound the owned desktop delivery queue

The hook writer SHALL keep at most 256 timestamped notification requests and expire owned requests and orphan temporary files older than the consumer's 30-second stale threshold. Maintenance SHALL stream directory entries with bounded memory, serialize cooperating writers with a kernel lease, and preserve foreign files and symlinks. Lease contention SHALL be a bounded best-effort delivery failure rather than waiting for the app. Notification requests SHALL remain private atomic files; queueing SHALL NOT prove banner delivery.

#### Scenario: The menu-bar app is absent

- **WHEN** many hooks enqueue requests without a running consumer
- **THEN** new writes retain only the newest bounded owned backlog without touching logs or other state
