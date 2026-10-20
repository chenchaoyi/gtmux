# Clio and HQ topic discovery

Research note, 2026-10-20. Topic discovery below is a proposed direction, not shipped behavior.

## What transfers

Anthropic’s [Clio introduction](https://www.anthropic.com/news/clio) describes extracting conversation attributes, clustering related material, naming groups and building a hierarchy. The [paper, §2.2 and Appendix B](https://arxiv.org/html/2412.13678v1) describes embeddings and k-means for similarity grouping. These are useful discovery tools for an unorganized conversation history.

## Proposed fit for gtmux

A conversation can contain many goals, and one goal can span several conversations. Use a goal episode as the candidate unit: retain original message IDs/time intervals, the requested outcome, affected artifact, constraints and explicit references. Prefer existing matter IDs, task references, PR/issue links and recorded participant associations before semantic similarity.

For material without an explicit link, produce a small set of candidate existing matters or a proposed new matter, with evidence snippets and a reason. Keep uncertain candidates unassigned. A broad topic such as “gtmux UI” can group distinct tasks for navigation; it is not sufficient evidence to merge their execution histories. User/HQ corrections must keep stable IDs and recorded merge/split edges.

Use an incremental, separately tracked read cursor and cache unchanged excerpts. Never reuse or advance HQ’s event-consumption watermark for discovery. Start with bounded excerpts processed by the existing HQ workflow; a new vector database or always-running classifier is not a prerequisite. Any external model processing needs an explicit data boundary and should receive only the necessary source material.

## Limits and evaluation

The paper’s privacy design includes size thresholds and auditing; it is intended for aggregate analysis and does not provide a formal differential-privacy guarantee (§2.3). Personal task management needs exact artifacts and small, unique goals, so suppressing rare topics would discard useful work. Preserve provenance locally and minimize model input rather than claiming that the original aggregate protections transfer automatically.

Evaluate task discovery separately from broad-topic clustering: unrelated tasks on one project must stay separate; a task across agent/session changes must remain linkable; corrections, multilingual goal changes and ambiguous evidence need coverage. Measure false merges, missed links, attribution evidence and label stability on synthetic/redacted examples. A cluster is not authorization to send, close, archive or complete a task. Current explicit matter records remain authoritative.

## Current scope

The native Overview brief reads bounded excerpts from explicit workspace records. It does not run this discovery pipeline, regroup conversations or change the HQ playbook. The brief’s report/state/source boundaries are documented in [HQ workspace](hq-workspace.md).
