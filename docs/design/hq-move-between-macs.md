# Moving HQ to another Mac

## Two different jobs

**Restore this Mac** puts a complete HQ archive back after loss. It can restore the old
situation board and other machine-specific files. The existing `gtmux hq --import` does
this and moves the destination home aside first.

**Move to a new Mac** carries selected durable content into a different HQ. It must never
restore the old Mac's live situation, generated instructions, connections or permissions.
The current full import is therefore not the move command. Its archive can still be the
source. The Mac menu bar makes a passphrase-encrypted export; the phone's existing copy
is a plain `.tar.gz`. Both contain the files needed for a selective import, but the plain
copy must be labelled as unencrypted wherever it is shared or chosen.

## What moves

| Content | New Mac default | Reason |
|---|---|---|
| Knowledge ledger (`knowledge/.ledger.jsonl`) | Offered for selection | Durable lessons and their provenance are the main thing worth carrying. Machine-specific claims need review before use. |
| Your instructions (`LOCAL.md`) | Offered for selection | Your preferences and authority limits survive a computer change. Show the text and any destination differences before applying it. |
| Knowledge tools (`knowledge/tools/`) | Stage for review only | Scripts may name old paths, credentials or installed tools. Do not execute or distribute them on arrival. |
| Situation board (`notes/board.md`) | Excluded | It describes sessions, panes and open decisions on the old Mac. The new HQ builds its own board. |
| Built-in instructions (`AGENTS.md`, `CLAUDE.md`) | Excluded | gtmux seeds the edition for the installed version and chosen language. |
| Other notes, pending distillation, promotion outputs | Excluded | Their state and destinations belong to the old machine or workflow. The ledger retains the lesson and provenance. |
| Session/event logs, snapshots, machine instruction blocks, tunnel accounts, device tokens | Excluded | They are host state or credentials, not HQ knowledge. Reconnect and authorize the new Mac separately. |

No metadata field currently proves that an entry is safe on another machine. `kind`,
`topic`, `audience`, and a nonempty source are useful review hints, not an automatic
portability verdict. In particular, a `machine` audience is about the *source* Mac.
Sensitive entries stay in the encrypted source and are shown as a count in preview;
moving them requires a separate choice and confirmation. Previews must not print their
bodies or secrets into logs.

## User flow

1. On the old Mac, preferably export HQ records with a passphrase. The phone can export
   its existing plain copy to Files when the old Mac is unavailable; the UI warns that
   this file is unencrypted. Either archive can also serve same-Mac recovery.
2. On the new Mac, choose **HQ records → Move from another Mac** (CLI equivalent:
   `gtmux hq migrate --from <archive>`). Enter the passphrase locally. A plain phone copy
   needs no passphrase but requires an explicit acknowledgement that it is unencrypted.
   A read-only preview shows source date, counts, available choices, sensitive count and
   destination conflicts; it changes nothing.
3. Select **Knowledge base** and/or **Your instructions**; both start unchecked. The
   latter opens a text diff against the destination's `LOCAL.md`; keeping the destination
   is always an option.
   The board and built-in instructions are shown as *staying on the old Mac*, not as
   disabled checkboxes that suggest they might be appropriate.
4. Import into a private staging area. If the new HQ is active, stop before apply with a
   clear request to finish that turn. After validation and a destination snapshot, apply
   the selected data. A failed apply restores the destination snapshot.
5. Show a receipt: imported, skipped duplicates, conflicts and entries awaiting review.
   Open the review list. The new HQ creates its own board from current sessions.

## Knowledge merge and review

The ledger is append-only history, not a folder of independent Markdown pages. The
importer validates every operation and preserves source IDs, lineage and provenance in
a staging ledger. It does not copy rendered topic pages; those are regenerated.

- An exact entry/history already present at the destination is skipped. An ID with
  different content is a conflict for review, never last-writer-wins.
- Imported entries are marked **from another Mac / review needed** and cannot be echoed
  to workers, promoted or used as machine facts until reviewed. Review can accept one
  entry, accept a selected group after inspection, or leave it archived. A blanket
  machine-safe judgement from topic or audience is forbidden.
- A sensitive entry needs its own opt-in and remains under the existing sensitive-entry
  rules after acceptance. Tool files remain inert attachments until individually checked.
- If no destination knowledge exists, the same validation and review status still apply;
  an empty home is not proof that old machine facts now hold.

The apply step uses a versioned migration manifest, bounded archive extraction, path
validation and a private staging directory. It writes a structured receipt with counts
and a migration ID, not content or passphrases. Repeating the same import is idempotent.

## Surfaces and wording

- **Mac menu bar** owns the file picker, preview, selection, conflict review and final
  receipt. It labels the existing full action **Export HQ records for backup** and the
  new action **Move from another Mac**.
- **CLI** exposes preview and apply for people setting up a headless Mac. It shares the
  same validation and receipt as the menu bar.
- **Phone and iPad** keep their existing full copy and export action. The export says
  plainly that its `.tar.gz` is unencrypted. They can pass that file to the new Mac;
  no remote import into a Mac is offered from a phone.
- **Web** has no records export or import: a shared page must never gain that access.

This is a design for the move flow. The current `--import` remains a full restore until
the separate migration command and review path are implemented.
