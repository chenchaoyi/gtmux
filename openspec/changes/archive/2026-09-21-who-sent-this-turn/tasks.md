# Tasks — who-sent-this-turn

## 1. The journal says who sent
chat-transcript

- [x] `gtmux:audit:send` records the sender from `diag.Caller()`
- [x] `gtmux events` prints it, so the trail reads the same as the action log

## 2. The transcript attributes a turn
chat-transcript

- [x] A turn carries `from` when gtmux delivered its prompt on someone else's behalf
- [x] The join is bounded by the oldest turn served, and cached against the journal's seq
- [x] An unmatched turn carries nothing, and renders as it does today
- [x] `api/contract.md` documents the field

## 3. The chat draws it
chat-transcript

- [x] Phone: the sender's avatar replaces the person-battery on a marked prompt
- [x] Web: the same, in the chat and in a tile's chat
- [x] `MOBILE.md` + its Chinese half carry the rule
