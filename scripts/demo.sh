#!/usr/bin/env bash
# Demo: event-sourced bank account with CQRS split and read-model replay.
set -u
cd "$(dirname "$0")/.."
PORT=18080

echo "=== 1. Build & start ==="
go build -o bin/server ./cmd/server
./bin/server > /tmp/poc2-server.log 2>&1 &
SRV_PID=$!
trap 'kill $SRV_PID 2>/dev/null' EXIT
sleep 1

echo "=== 2. Write side: open account, deposit, withdraw ==="
ACCT=$(curl -s -X POST localhost:$PORT/commands/open-account -d '{"owner":"Ada"}' | grep -o '"account_id":"[^"]*"' | cut -d'"' -f4)
echo "account: $ACCT"
curl -s -X POST localhost:$PORT/commands/deposit -d "{\"account_id\":\"$ACCT\",\"amount_cents\":5000}"; echo
curl -s -X POST localhost:$PORT/commands/withdraw -d "{\"account_id\":\"$ACCT\",\"amount_cents\":1200}"; echo

echo "=== 3. Write side validation: overdraft is rejected, never appended ==="
curl -s -X POST localhost:$PORT/commands/withdraw -d "{\"account_id\":\"$ACCT\",\"amount_cents\":99999}"; echo

echo "=== 4. Read side: balance comes from the projection, not the log ==="
curl -s localhost:$PORT/balance/$ACCT; echo

echo "=== 5. The log itself: every fact that ever happened, nothing more ==="
curl -s localhost:$PORT/admin/events | python3 -c "
import json,sys
for e in json.load(sys.stdin):
    print(' seq=%d %-16s amount=%s owner=%s' % (e['seq'], e['type'], e.get('amount',''), e.get('owner','')))
"

echo "=== 6. Replay: wipe the read model and rebuild it from the log ==="
curl -s -X POST localhost:$PORT/admin/replay; echo
echo "balance after replay (identical):"
curl -s localhost:$PORT/balance/$ACCT; echo

echo
echo "=== Done. Log: /tmp/poc2-server.log ==="
