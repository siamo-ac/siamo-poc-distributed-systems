# siamo-poc-distributed-systems

**Concept, in plain language.** Most apps store *current state*: a `balance`
column that gets overwritten on every transaction, destroying history.
**Event sourcing** flips that: the database is an append-only log of *things
that happened* (`AccountOpened`, `MoneyDeposited`, `MoneyWithdrawn`). Current
state is never stored — it is *derived* by replaying the log. **CQRS**
(Command Query Responsibility Segregation) splits the system in two: the
**write side** validates commands and appends events; the **read side** folds
events into a queryable view (a projection). The two sides can evolve and
scale independently, which is why the pattern shows up in distributed
systems.

**CAP / ACID vs BASE, as it applies here.** On this single node, CAP is
trivially satisfied (no network partitions to survive). The interesting
lesson is ACID vs BASE: the write side is ACID-ish — one mutex-guarded log,
every command validated against a consistent rebuild of its aggregate. The
read side is BASE — *Basically Available, Soft state, Eventually consistent*:
the balance projection is a disposable cache that lags the log and can be
thrown away and rebuilt at any time. Real distributed systems accept that
trade everywhere: the log is the truth (consistent), the views are
convenient approximations (eventually consistent). `POST /admin/replay` makes
that concrete — delete the whole read model, rebuild it from the log, get
identical answers.

Layout is hexagonal-light: `cmd/server` is the main; `internal/` holds the
event store (`events`), the aggregate with business rules (`domain`), the
write side (`write`), the read projection (`read`), and the HTTP adapter
(`api`).

## Run the demo

```bash
cd ~/workspace/services/pocs/siamo-poc-distributed-systems
chmod +x scripts/demo.sh
./scripts/demo.sh          # port :18080 (chosen to avoid the live engines on :8080/:8090)
```

Or by hand:

```bash
go build -o bin/server ./cmd/server
./bin/server &                       # :18080

# write side: commands append events
ACCT=$(curl -s -X POST localhost:18080/commands/open-account -d '{"owner":"Ada"}' | grep -o '"account_id":"[^"]*"' | cut -d'"' -f4)
curl -X POST localhost:18080/commands/deposit  -d "{\"account_id\":\"$ACCT\",\"amount_cents\":5000}"
curl -X POST localhost:18080/commands/withdraw -d "{\"account_id\":\"$ACCT\",\"amount_cents\":1200}"
curl -X POST localhost:18080/commands/withdraw -d "{\"account_id\":\"$ACCT\",\"amount_cents\":99999}"  # rejected: overdraft

# read side: balance from the projection
curl localhost:18080/balance/$ACCT

# the log itself
curl localhost:18080/admin/events

# replay: rebuild the read model from scratch
curl -X POST localhost:18080/admin/replay
curl localhost:18080/balance/$ACCT   # identical
```

## What to observe

1. **The write/read split is structural, not cosmetic.** `POST /commands/*`
   only ever appends to the log; `GET /balance/*` only ever reads the
   projection. The projection has no business rules and accepts no commands.
2. **The log is the truth; state is derived.** `/admin/events` shows exactly
   3 events for the demo account (opened, deposited 5000, withdrew 1200).
   The rejected overdraft produced an error but **no event** — invalid
   commands never touch the log.
3. **Replay is a superpower.** `POST /admin/replay` wipes the projection and
   rebuilds it from the log; the balance is byte-identical afterward
   (`events_applied: 3`). That is how you recover a corrupted read model, add
   a brand-new projection (e.g. monthly statements), or fix a projection bug:
   replay history through corrected code.
4. **Validation replays too.** Each command rebuilds its aggregate from that
   account's events before checking the rules — the write side never trusts a
   stored balance column, because there isn't one.

## Honest limits

- **POC — not production hardening.** The event store is **in-memory**: stop
  the server and all history is gone. A real system would persist the log
  (and snapshot aggregates so commands don't replay unbounded history).
- The read side updates **synchronously** via callback after each append, so
  the demo is deterministic. A real distributed design would publish events
  over a bus and let projections update **asynchronously** — genuinely
  eventually consistent, with lag you can observe.
- No concurrency control: two simultaneous withdrawals could both pass
  validation against the same rebuild (no optimistic locking / expected
  sequence numbers).
- No event schema versioning or upcasting — old events and new code are
  assumed compatible.
- Amounts are integer cents; no multi-currency, no idempotency keys on
  commands (retrying a deposit would double-apply it).
