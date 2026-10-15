## ADDED Requirements

### Requirement: Mobile request diagnostics distinguish expected outcomes
The phone and iPad SHALL treat GET/HEAD HTTP 304 as successful cache validation, without
adding a failure warning, and SHALL retain the consumer's cached-content behavior.
An explicitly aborted lifecycle request SHALL NOT be recorded as a network failure.
App-imposed deadlines SHALL be marked before aborting and remain warning records with
a timeout reason. Other transport and HTTP failures SHALL remain observable. Elapsed
time alone SHALL NOT imply timeout, refusal or that the whole Mac is offline.
Previously stored GET/HEAD 304 failure records SHALL display as unchanged content and
be excluded from problem counts/filters; raw JSON exports SHALL remain unchanged.

#### Scenario: Cached conversation validation
- **WHEN** a transcript read returns HTTP 304
- **THEN** its existing content is retained and no failure is recorded
- **AND** a previously failing route can record recovery

#### Scenario: Leaving a page versus a deadline
- **WHEN** navigation cancels a read
- **THEN** no network warning is added
- **WHEN** a marked deadline expires
- **THEN** its timeout warning remains visible without claiming the Mac is offline

#### Scenario: Legacy false warning
- **WHEN** an older app stored GET transcript HTTP 304 as api.failed
- **THEN** the diagnostic page explains unchanged content without a problem marker
- **AND** copying the record still exports the original entry

## MODIFIED Requirements

### Requirement: Each surface shows what it recorded, in words

A person reading a diagnostic record is already having a bad day, so each surface SHALL
show its own record rather than only offer to hand it over, and SHALL show each entry as a
sentence with its raw attributes underneath it.

The menu bar SHALL carry a Diagnostics section: how much the store holds, how long it is
kept and how many of today's entries are warnings or errors; a window showing the last
three days, newest first, filtered to everything or to problems only; a one-click bug
report (`gtmux doctor --bundle`) that names where the file landed; and a switch for
recording extra detail, under a plain note saying what it is for and to turn it off when
done. When it takes effect (each process as it next starts) is for `gtmux config debug` and
the CLI docs to say; the note is read by someone who cannot act on process lifetimes.

The phone and iPad app SHALL carry a diagnostic record page reached from one settings row,
that row stating the number of problems when there are any and how much is kept when there
are none. The page SHALL group entries by day with today and yesterday named, mark actionable
warnings and errors, filter to problems only, and offer Copy, Share and Clear. Before
anything is recorded it SHALL say what will show up there and that credentials are
replaced before anything is written down.

Entry text SHALL be written for the reader of that surface: the app translates its own
entries, and the Mac's store stays English because its entries are what a bug report
quotes.

#### Scenario: A phone that stopped updating

- **WHEN** the Mac stops answering and the person opens the diagnostic record
- **THEN** the page names the incomplete request, route, elapsed time and repeat count,
  without inferring that the entire Mac is offline or requiring the reader to decode
  `api.failed` or `status=0`

#### Scenario: Nothing has gone wrong yet

- **WHEN** the record is empty
- **THEN** the page says what shows up there and that nothing is uploaded, instead of
  showing an empty list
