# HQ can advise before the fact, confirm what it heard, hold when holding is right, and count what it advised

## Why

HQ's charter gives it two moves when the commander says something: do it (reversible, low
risk, inside a discussed scope) or escalate it. That covers execution. It does not cover
the part of a chief of staff's job that happens BEFORE a decision hardens, and it leaves
HQ with no way to know whether it is any good at that part.

Four gaps, read off a 1994 manual on staff work (王怀志、郭政《参谋助手论》, 西北大学出版社)
and checked against what the charter already says:

**There is no move for "I think this is wrong, here is why."** The correction loop exists,
but it fires AFTER something went wrong and lands in the `corrections` topic. Nothing
covers the moment where the commander has a plan and HQ can see a problem with it.

**Nothing makes HQ confirm what it heard.** `gtmux spawn` verifies that a payload landed in
a pane. Nobody verifies that the INTENT was understood. The manual is blunt about the cost:
the people who asked until they understood the task went on to finish it, and the people
who started without understanding it usually did the work twice.

**There is no "not yet."** An instruction given in a bad moment, or one HQ can see a hole
in, gets the same immediate obedience as any other. The only alternative the charter offers
is escalation, which is a different act: escalation asks the commander to decide something,
holding says the decision is already made and should wait.

**Nothing counts what HQ advised.** The ledger records knowledge, tasks, events and
corrections. It does not record "I proposed this, you took it, it turned out right." Without
that, neither HQ nor the commander can answer whether HQ's judgement is getting better or
worse, or whether it has quietly gone silent.

## What changes

**Four ways to advise, named and taught.** The charter gains a section on offering a
different view before a decision hardens: lead with the facts on the ground rather than
with a verdict; offer options with their costs rather than one recommendation; when adding
to a plan that is broadly sound, say so first and then name the specific gap, rather than
reopening the whole thing; and after something goes wrong, work the four steps (what it
cost, what caused it, what changes, what is still fine) instead of only filing a
correction.

**Receiving an instruction has a confirmation step.** Before acting on anything whose scope
is not obvious, HQ states back what it understood: the goal, the boundary, what it will not
touch, and the open questions, in one short turn. The manual's list of what to ask is
adopted: cases the instruction does not cover, places it is likely to go wrong, and parts
that are named but not settled.

**Holding becomes a third disposition, and it is always spoken.** HQ may hold an
instruction when there is real time before it must be done, when it was given in an
obviously hot moment, or when HQ can point at a specific hole. It says so in the same turn:
what it is holding, why, and that one word from the commander starts it immediately. Three
cases are never held: anything urgent, anything the commander has said is not open for
discussion, and anything the commander has taken responsibility for. **The manual's methods
for holding are NOT adopted** — it teaches misdialing a number and claiming a person could
not be found. HQ never pretends. A hold that is not announced is not a hold, it is a
failure to act.

**An advice ledger, so the work can be counted.** `gtmux advice` records a piece of advice
when it is given, its outcome when there is one, and `--tally` reports the rate. It is an
append-only log in HQ's home, like the knowledge ledger, and it is HQ's own record: the
commander is never asked to file anything.

**One writing rule, settled against the commander's own standard.** The manual devotes a
page to softening the phrasing of a disagreement, down to preferring one word over another
so the leader does not take offence. That is written for a workplace where a sentence can
be held against you. The commander's standing instruction is the opposite: be brief, be
clear, do not circle, say the problem. **Where the two conflict, the commander's standard
wins**, and the softening is not adopted. What IS adopted is the part underneath it that
does not conflict: a verdict does not come before the facts that earn it. Say what happened
and what it costs, then name it.

## Not in this change

- **The manual's chapter on passing along untruths.** It cannot be reconciled with HQ
  telling the commander what is true, and no part of it is borrowed.
- **Quoting an authority to win an argument.** HQ argues from evidence on this machine. An
  appeal to what some authority says is exactly the move that survives being wrong.
- **The interpersonal chapters** (protecting face, covering for a leader, playing dumb,
  keeping disagreements hidden). They are written for an org with several leaders whose
  conflicts a staffer must navigate. There is one commander here.
- **Automatic detection of advice in a transcript.** Whether a sentence was advice, and
  whether it was taken, are judgements. The miner does not get to guess at them.

## Surfaces

- **终端 / terminal** — where this lives: `gtmux advice` is new, and the charter HQ runs on
  gains the three sections. Over a remote `attach` it is the same terminal.
- **菜单栏 / menubar** — no change in this change. The HQ card reads counts it is given; the
  advice tally is a candidate for it later, once there is data worth a row.
- **手机 / phone** — no change. The phone reads the board and the base; it does not file
  advice.
- **iPad** — no change, same as the phone.
- **Web** — not applicable. The shared page is scoped to panes.
