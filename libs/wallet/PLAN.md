# Wallet — Implementation Design

## 1. Goals

- **Multi-currency, multi-wallet ledger.** Multiple wallets per tenant; each wallet contains accounts in arbitrary currencies; transactions can move value between accounts (within or across wallets) within a single currency.
- **Append-only source of truth.** Transactions and entries are strictly immutable. State changes append new rows; nothing is ever updated or deleted.
- **Double-entry bookkeeping.** Every transaction balances per currency; every value movement is reconcilable.
- **Two-phase commit semantics.** Transactions can open as a lien (hold) and later execute or reverse. Single-phase (skip-lien direct execution) is supported.
- **Reversibility with audit.** Any LIEN or EXECUTED transaction can be reversed; full history is preserved.
- **Tag-driven configuration.** Fees, taxes, ledger postings, and FX targets are looked up from a config table keyed by tag predicates.
- **Idempotent mutations.** Every Layer 1 mutating call accepts a client-supplied idempotency key.

## 2. Non-goals

- Cross-currency atomicity within a single transaction row. FX is modeled at Layer 1 as two coupled single-currency groups.
- Replacing analytics and reporting. Those go through materialized views, not the live cache.
- Backwards-incompatible schema migrations. Schema changes are additive only; new fields are nullable or have defaults.
- General-purpose accounting features outside this scope (depreciation, accruals, multi-period closing).

## 3. Principles

- minor denomination based (kobo, cents, satoshi)
- multi-currency
- multi-wallet
- append-only, log-based
- double-entry

## 4. Glossary

- **Transaction Row (tx row)** — a single, immutable row in the `transactions` table. Represents one event in the lifecycle of a logical transaction. Carries its own status, type, currency, primary account, tags, and entries.
- **Group** — the set of transaction rows sharing a `group_id`. Represents one logical transaction across its full state history. Reading a group ordered by `created_at` gives the trail of state transitions.
- **Logical Transaction** — synonymous with Group. The end-user-visible "transaction" is a group; its current state is the status on the latest row.
- **Entry** — a single per-account leg of a transaction row. Has a type (DEBIT or CREDIT), an amount, and an account reference. Entries are append-only and tied to a specific transaction row.
- **Primary Account** — the account that initiated the logical transaction. Identified by `transactions.primary_account_id`. Exactly one primary entry per row exists on this account.
- **Secondary Account** — every other account participating in the row (counterparty, fee account, ledger postings, etc.).
- **Primary Transaction** — the main logical transaction (e.g. a transfer between two user accounts).
- **Secondary Transaction** — auxiliary movements processed alongside the primary (e.g. fees, taxes, ledger postings). In this design, secondary entries are co-located on the same row as the primary.

## 5. Money & Currency

- **Minor units everywhere.** All amounts are stored as integer minor units. Examples: NGN 100 = ₦1; USD 100 = $1; BTC 100,000,000 = ₿1; JPY 1 = ¥1.
- **No floating point in money math.** APIs accept and return integer minor units. Display formatting is a presentation concern handled by clients.
- **`currencies` reference table** holds `(code, exponent, name)`. Transactions and accounts FK on `currency`.
- **Bounds.** Amounts fit in `BIGINT` (i64). Per-account `balance` and `available_balance` also fit in i64. Values approaching i64 max should fire an alarm.
- **Single currency per row.** All entries on a row reference accounts whose `currency` matches the row's `currency`. Cross-currency movement uses the FX flow (two coupled rows in two groups).

## 6. Type (per Entry and per Row)

- **Entry type:** `DEBIT` (withdrawal direction on its account) or `CREDIT` (deposit direction).
- **Row type:** `DEBIT` or `CREDIT`. Mirrors the type of the row's primary entry (the entry on `primary_account_id`). Stored on the row for query convenience; must be consistent with entries.

## 7. Status & State Machine

Frozen on each row: `LIEN`, `EXECUTED`, `REVERSED`.

The current status of a group is the status of the latest row in the group. Status is never mutated — every state change appends a new row.

### Allowed events

| Event              | New row.status | New row.prior_status | Group precondition                      |
| ------------------ | -------------- | -------------------- | --------------------------------------- |
| open lien          | LIEN           | NULL                 | new group                               |
| execute direct     | EXECUTED       | NULL                 | new group (skip-lien)                   |
| execute lien       | EXECUTED       | LIEN                 | latest row in group is LIEN             |
| reverse lien       | REVERSED       | LIEN                 | latest row in group is LIEN             |
| reverse executed   | REVERSED       | EXECUTED             | latest row in group is EXECUTED         |

Once a group has a `REVERSED` row, no further events are accepted.

### State machine

```
              (new group)
                 │
        ┌────────┴────────┐
        │                 │
       open            execute
       lien             direct
        │                 │
        ▼                 ▼
      LIEN  ─ execute ─► EXECUTED
        │                 │
     reverse           reverse
       lien            executed
        │                 │
        └─────► REVERSED ◄┘
                   │
                (terminal)
```

## 8. Tags

Tags are key/value pairs (both strings) used to categorize accounts and transactions. Examples:

- `category`: `main`, `earn`, `savings`
- `type`: `personal`, `business`
- `product`: `transfer`, `bill_payment`, `airtime`

Tag keys are registered in `tag_definitions`. A registered tag may declare an enum of `allowed_values` (NULL = free-form). Inserts that violate the vocabulary fail with `INVALID_TAG`.

## 9. Config

The `config_rules` table holds rules keyed by tag predicates. Each rule has:

- `kind` — `lookup` (single-outcome: ledger account, FX target) or `accumulating` (multi-outcome: fees, taxes).
- `category` — `fee`, `tax`, `ledger_posting`, `fx_target`, etc.
- `match` — JSONB tag predicate (set of key/value pairs that must all be present on the row's tags ∪ primary account's tags).
- `priority` — integer; higher wins.
- `payload` — JSONB outcome (account ref, fee schedule, etc.).
- `active` — boolean; inactive rules are ignored.

### Conflict resolution

- **Lookup rules:** of all rules whose `match` is satisfied, the one with the highest `priority` wins. Ties are broken by earliest `created_at`. If none match, the call fails with `CONFIG_RULE_MISSING`.
- **Accumulating rules:** every rule whose `match` is satisfied applies. They are ordered by `priority` descending for deterministic application order (relevant when fees compound).

This applies to fees/taxes (accumulating), ledger postings (lookup), and FX target accounts (lookup).

## 10. Source of Truth & Cached Balances

- The `transactions` and `entries` tables are the **source of truth** and are **strictly append-only**. No field on any row is ever mutated.
- Every state transition appends a new transactions row to the same group, plus its entries. This applies uniformly to opening a lien, executing it, executing direct, and reversing.
- The full history of a logical transaction is `SELECT … FROM transactions WHERE group_id = ? ORDER BY created_at, id`. This is the audit trail.
- The `accounts` table caches `balance` and `available_balance`. The cache is updated atomically with each appended row so reads are O(1).
- The cache is a projection: it can always be rebuilt by replaying the log. The reconcile job verifies cache vs projection.

## 11. Balance Computation Model

The cache update produced by each appended row is determined by `(row.type, row.status, row.prior_status)` and is applied per entry on the row.

For each entry `e`:
- It is the **primary entry** if `e.account_id == row.primary_account_id`; otherwise **secondary**.
- Look up `(row.type, role, row.status, row.prior_status)` in the table below to get the (balance Δ, available Δ).
- Apply `delta * e.amount` to `e.account.balance` and `e.account.available_balance` respectively. (Delta of `---` means no update to that field.)

| row.type | row.status, row.prior_status        | Primary balance | Primary available | Secondary balance | Secondary available |
| -------- | ----------------------------------- | --------------- | ----------------- | ----------------- | ------------------- |
| DEBIT    | LIEN, NULL                          | ---             | -e.amount         | +e.amount         | ---                 |
| DEBIT    | EXECUTED, LIEN                      | -e.amount       | ---               | ---               | +e.amount           |
| DEBIT    | EXECUTED, NULL (skip-lien)          | -e.amount       | -e.amount         | +e.amount         | +e.amount           |
| DEBIT    | REVERSED, LIEN                      | ---             | +e.amount         | -e.amount         | ---                 |
| DEBIT    | REVERSED, EXECUTED                  | +e.amount       | +e.amount         | -e.amount         | -e.amount           |
| CREDIT   | LIEN, NULL                          | +e.amount       | ---               | ---               | -e.amount           |
| CREDIT   | EXECUTED, LIEN                      | ---             | +e.amount         | -e.amount         | ---                 |
| CREDIT   | EXECUTED, NULL (skip-lien)          | +e.amount       | +e.amount         | -e.amount         | -e.amount           |
| CREDIT   | REVERSED, LIEN                      | -e.amount       | ---               | ---               | +e.amount           |
| CREDIT   | REVERSED, EXECUTED                  | -e.amount       | -e.amount         | +e.amount         | +e.amount           |

Multi-leg rows: each secondary entry independently receives the secondary delta scaled by its own amount.

EXECUTED-from-LIEN and REVERSED rows replicate the entry list of the row they are transitioning from (with type-flipped for REVERSED) so each row carries a complete, auditable entry set. The cache delta is computed from the table above, not from summing entries.

## 12. Atomicity

- All entries within a single transaction row commit atomically.
- `append_transaction_rows` accepts an array; the entire array (rows + entries + accounts cache update + idempotency row) is one atomic DB transaction. Partial commits are not possible.
- Layer 1 multi-row operations (FX, lien-with-fees) are all-or-nothing across their constituent rows and groups.

## 13. Idempotency

### Request hash

- SHA-256 of canonical-JSON request body. Canonical form: object keys sorted lexically, no whitespace, integers (not strings) for amounts, ISO-8601 for timestamps.
- Stored as `BYTEA(32)` in `idempotency.request_hash`.

### Conflict semantics

- Same `(key, request_hash)` → return cached `response` (HTTP 200, idempotent replay).
- Same `key`, different `request_hash` → `IDEMPOTENCY_CONFLICT` (HTTP 409) with the original request's `created_at`.
- Different `wallet_id` for an existing key → `IDEMPOTENCY_CONFLICT`.

### TTL

- Default 24h. Configurable per-call up to 7 days.
- Daily cron deletes rows where `expires_at < NOW()`. After deletion, the key is reusable.

### Layer 0

`append_transaction_rows` may pass an idempotency key through. If absent, the server generates one for internal bookkeeping (so a retry from the same Layer 1 call doesn't re-append rows).

## 14. Concurrency & Locking

Per Layer 1 mutating call:

1. Validate request shape and authorization.
2. **Outside the DB transaction:** look up idempotency, config rules, FX rate, and group's current state.
3. `BEGIN` (READ COMMITTED is sufficient given explicit row locks).
4. `SELECT … FOR UPDATE` on every account involved (primary, secondaries, fee accounts, ledger accounts, FX intermediates), **ordered by `account.id` ASC** to avoid deadlocks.
5. Re-read each locked account's `balance`, `available_balance`, `status`.
6. For state transitions on existing groups: `SELECT … FROM transactions WHERE group_id = ? ORDER BY created_at DESC, id DESC LIMIT 1 FOR UPDATE`. Verify the latest row's status matches expected `prior_status`.
7. Validate post-lock invariants (sufficient funds; account active; group not REVERSED; etc.).
8. `INSERT` transactions row(s); `INSERT` entries; `UPDATE` accounts (cache); `INSERT` idempotency row.
9. `COMMIT`.

Notes:
- Lock all accounts in a single sorted batch *before* any insert, even across two FX legs.
- `FOR UPDATE` on the latest row of a group is required to serialize concurrent `execute`/`reverse` calls on the same group.
- App-level `LOCK_TIMEOUT` should bound how long a call waits for locks (default 5s).

## 15. Invariants

Each invariant lists where it's enforced.

1. **Append-only on transactions and entries.** DB rules turn UPDATE/DELETE into noop. Non-admin roles lack UPDATE/DELETE permission. Periodic audit confirms.
2. **Balanced rows.** `SUM(entries.amount WHERE type=DEBIT) = SUM(entries.amount WHERE type=CREDIT)` per row. App + reconcile.
3. **Single currency per row.** All entries reference accounts of `transactions.currency`. App + reconcile.
4. **Status-transition validity.** `(status, prior_status)` matches an allowed event. DB CHECK constraint + app match against group's actual latest.
5. **Terminal REVERSED.** No event accepted on a group whose latest row is REVERSED. App.
6. **Cache consistency.** For every account, `balance` and `available_balance` equal the projection over the entry log per the Balance Computation Model. Reconcile.
7. **Non-negative for prepaid.** Accounts with `class='prepaid'` cannot have `balance < 0` or `available_balance < 0` after any event. App pre-commit check.
8. **Idempotency uniqueness.** No two `(key, request_hash)` pairs with different responses. App returns 409.
9. **Type consistency.** `transactions.type` equals the type of the entry on `transactions.primary_account_id`. App + reconcile.
10. **Primary entry exists.** Every row has exactly one entry whose `account_id = primary_account_id`. App + reconcile.
11. **FX coupling.** Both legs of a `convert` share `fx_pair_group_id`; both commit or neither. App.

## 16. Database Schema

### DDL

```sql
-- Currencies
CREATE TABLE currencies (
  code TEXT PRIMARY KEY,
  exponent INT NOT NULL CHECK (exponent >= 0),
  name TEXT NOT NULL
);

-- Wallets
CREATE TABLE wallets (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL,
  name TEXT NOT NULL,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_wallets_tenant ON wallets(tenant_id);

-- Accounts (with cached balances)
CREATE TABLE accounts (
  id BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
  wallet_id TEXT NOT NULL REFERENCES wallets(id),
  currency TEXT NOT NULL REFERENCES currencies(code),
  class TEXT NOT NULL CHECK (class IN ('prepaid','liability','asset','revenue','expense')),
  tags JSONB NOT NULL DEFAULT '{}'::jsonb,
  status TEXT NOT NULL CHECK (status IN ('active','frozen','closed')) DEFAULT 'active',
  balance BIGINT NOT NULL DEFAULT 0,
  available_balance BIGINT NOT NULL DEFAULT 0,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_accounts_wallet ON accounts(wallet_id);
CREATE INDEX idx_accounts_tags ON accounts USING GIN (tags);

-- Transactions (append-only)
CREATE TABLE transactions (
  id BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
  group_id UUID NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('LIEN','EXECUTED','REVERSED')),
  prior_status TEXT CHECK (prior_status IN ('LIEN','EXECUTED')),
  CONSTRAINT valid_transition CHECK (
    (status = 'LIEN' AND prior_status IS NULL) OR
    (status = 'EXECUTED' AND (prior_status IS NULL OR prior_status = 'LIEN')) OR
    (status = 'REVERSED' AND prior_status IN ('LIEN','EXECUTED'))
  ),
  type TEXT NOT NULL CHECK (type IN ('DEBIT','CREDIT')),
  primary_account_id BIGINT NOT NULL REFERENCES accounts(id),
  currency TEXT NOT NULL REFERENCES currencies(code),
  tags JSONB NOT NULL DEFAULT '{}'::jsonb,
  idempotency_key UUID,
  fx_pair_group_id UUID,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_tx_group_created ON transactions(group_id, created_at DESC, id DESC);
CREATE INDEX idx_tx_status ON transactions(status);
CREATE INDEX idx_tx_idempotency ON transactions(idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX idx_tx_tags ON transactions USING GIN (tags);
CREATE INDEX idx_tx_primary_account ON transactions(primary_account_id);
CREATE INDEX idx_tx_fx_pair ON transactions(fx_pair_group_id) WHERE fx_pair_group_id IS NOT NULL;

CREATE OR REPLACE RULE transactions_no_update AS ON UPDATE TO transactions DO INSTEAD NOTHING;
CREATE OR REPLACE RULE transactions_no_delete AS ON DELETE TO transactions DO INSTEAD NOTHING;

-- Entries (append-only)
CREATE TABLE entries (
  id BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
  transaction_id BIGINT NOT NULL REFERENCES transactions(id),
  account_id BIGINT NOT NULL REFERENCES accounts(id),
  type TEXT NOT NULL CHECK (type IN ('DEBIT','CREDIT')),
  amount BIGINT NOT NULL CHECK (amount > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_entries_tx ON entries(transaction_id);
CREATE INDEX idx_entries_account ON entries(account_id);

CREATE OR REPLACE RULE entries_no_update AS ON UPDATE TO entries DO INSTEAD NOTHING;
CREATE OR REPLACE RULE entries_no_delete AS ON DELETE TO entries DO INSTEAD NOTHING;

-- Tag vocabulary
CREATE TABLE tag_definitions (
  key TEXT PRIMARY KEY,
  description TEXT,
  applies_to TEXT NOT NULL CHECK (applies_to IN ('account','transaction','both')),
  allowed_values TEXT[]
);

-- Config rules
CREATE TABLE config_rules (
  id BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
  kind TEXT NOT NULL CHECK (kind IN ('lookup','accumulating')),
  category TEXT NOT NULL,
  match JSONB NOT NULL,
  priority INT NOT NULL DEFAULT 0,
  payload JSONB NOT NULL,
  active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_config_kind_category ON config_rules(kind, category) WHERE active;
CREATE INDEX idx_config_match ON config_rules USING GIN (match);

-- FX rates
CREATE TABLE rates (
  id BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
  from_currency TEXT NOT NULL REFERENCES currencies(code),
  to_currency TEXT NOT NULL REFERENCES currencies(code),
  rate NUMERIC(30, 12) NOT NULL CHECK (rate > 0),
  valid_from TIMESTAMPTZ NOT NULL,
  valid_to TIMESTAMPTZ,
  source TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_rates_lookup ON rates(from_currency, to_currency, valid_from DESC);

-- Idempotency
CREATE TABLE idempotency (
  key UUID PRIMARY KEY,
  wallet_id TEXT NOT NULL REFERENCES wallets(id),
  request_hash BYTEA NOT NULL,
  response JSONB NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_idempotency_expires ON idempotency(expires_at);

-- Helper view: latest row per group
CREATE VIEW current_transaction_state AS
SELECT DISTINCT ON (group_id) *
FROM transactions
ORDER BY group_id, created_at DESC, id DESC;
```

## 17. API Interface

All requests and responses are JSON. All amounts are integer minor units. All UUIDs are RFC 4122 v4.

Common response envelope:

```json
{ "data": { ... } }            // success
{ "error": { "code": "...", "message": "...", "details": {...} } }   // failure
```

### 17.1 Layer 0 (low level)

#### `create_accounts`

Request:
```json
{ "accounts": [
  { "wallet_id": "wlt_123", "currency": "NGN", "class": "prepaid",
    "tags": {"category": "main"}, "metadata": {} }
]}
```

Response:
```json
{ "data": { "accounts": [
  { "id": 42, "wallet_id": "wlt_123", "currency": "NGN", "class": "prepaid",
    "balance": 0, "available_balance": 0, "status": "active",
    "tags": {"category":"main"}, "metadata": {}, "created_at": "..." }
]}}
```

Errors: `INVALID_TAG`, `CURRENCY_NOT_FOUND`, `WALLET_NOT_FOUND`.

#### `get_account(id)` → `Account`
#### `list_accounts(wallet_id, tags?, currency?, class?, status?, limit, cursor)` → `Account[]`

#### `append_transaction_rows`

Atomically appends one or more transaction rows with their entries. Validates each row and the global precondition.

Request:
```json
{ "rows": [
  {
    "group_id": "uuid (existing or new)",
    "status": "LIEN",
    "prior_status": null,
    "type": "DEBIT",
    "primary_account_id": 42,
    "currency": "NGN",
    "tags": {"product":"transfer"},
    "idempotency_key": "uuid (optional)",
    "fx_pair_group_id": null,
    "entries": [
      { "account_id": 42, "type": "DEBIT", "amount": 1000 },
      { "account_id": 99, "type": "CREDIT", "amount": 1000 }
    ]
  }
]}
```

Per-row validation:
- `entries` non-empty; exactly one entry on `primary_account_id`; that entry's `type` matches `row.type`.
- `SUM(entries WHERE type=DEBIT) == SUM(entries WHERE type=CREDIT)`.
- All `entries.account_id` reference accounts of `currency` matching `row.currency`.
- `(status, prior_status)` is an allowed combination.

Per-group precondition (queried under lock):
- If `prior_status IS NOT NULL`: latest row of `group_id` has `status = prior_status`.
- If `prior_status IS NULL`: no rows exist for `group_id` (it must be new).
- Group is not in REVERSED state.

Cache update: per the Balance Computation Model, applied to each entry's account.

Errors: `UNBALANCED_ROW`, `CURRENCY_MISMATCH`, `INVALID_STATUS_TRANSITION`, `GROUP_ALREADY_REVERSED`, `GROUP_PRECONDITION_FAILED`, `INSUFFICIENT_FUNDS`, `ACCOUNT_FROZEN`, `ACCOUNT_NOT_FOUND`, `IDEMPOTENCY_CONFLICT`, `LOCK_TIMEOUT`.

#### `get_transaction_row(id)` → row + its entries
#### `get_group(group_id)` → all rows in the group ordered by `created_at` + the latest status
#### `list_groups(wallet_id, tags?, currency?, latest_status?, limit, cursor)`
#### `list_transaction_rows(filter)` — for low-level introspection

### 17.2 Layer 1 (high level)

All Layer 1 calls accept an `idempotency_key`. They consult the config table to materialize secondary entries (fees, taxes, ledger postings). Each call appends one or more rows via Layer 0; nothing is ever updated.

#### `lien`

Request:
```json
{ "wallet_id": "wlt_123",
  "from_account_id": 42, "to_account_id": 99,
  "amount": 1000, "currency": "NGN",
  "tags": {"product":"transfer","category":"main"},
  "idempotency_key": "550e8400-e29b-41d4-a716-446655440000",
  "metadata": {} }
```

Behavior:
1. Validate: accounts exist, are `active`, currency matches `currency`.
2. Resolve config rules from `tags ∪ from_account.tags ∪ to_account.tags`:
   - Accumulating (fees/taxes) → list of `{account_id, amount}` deductions.
   - Lookup (ledger postings) → list of `{account_id, amount}` postings.
3. Compose entries: primary `DEBIT from_account` for `amount + sum(fees+taxes)`; `CREDIT to_account` for `amount`; `CREDIT fee_account` per fee; `CREDIT tax_account` per tax; ledger postings as configured. Verify balanced.
4. Lock all accounts in `id` order. Re-read.
5. Verify `from_account.available_balance >= total debited`.
6. Generate `group_id`. Append a single LIEN row with all entries. Update caches.
7. Insert idempotency row. Commit. Return `group_id`, `transaction_row_id`, `status`, `entries`, `accounts_after`.

Errors: as Layer 0 + `CONFIG_RULE_MISSING` (if a required `lookup` rule has no match).

#### `execute(group_id, idempotency_key)`

1. Look up group's latest row. Must be LIEN.
2. Lock all accounts referenced by the LIEN row's entries.
3. Re-verify latest row's status is LIEN.
4. Append EXECUTED row with `prior_status=LIEN`, replicating the LIEN row's entry list. Update caches per the LIEN→EXECUTED deltas.
5. Insert idempotency. Commit. Return updated group.

Errors: `GROUP_NOT_FOUND`, `INVALID_STATUS_TRANSITION` (group not LIEN), `IDEMPOTENCY_CONFLICT`.

#### `execute_direct`

Same shape as `lien` but appends an EXECUTED row directly with `prior_status=NULL`. Cache updates use the skip-lien deltas. Used when a transaction has no holding period (e.g., bulk imports, instant settlement).

#### `reverse(group_id, idempotency_key)`

1. Look up group's latest row. Must be LIEN or EXECUTED. Reject if REVERSED.
2. Lock accounts referenced by the latest row's entries.
3. Append REVERSED row with `prior_status` = current latest's status. Entries list mirrors the latest row's entries with `DEBIT`/`CREDIT` flipped (so the row's net effect is the inverse). Apply cache deltas per the table.
4. Insert idempotency. Commit. Return updated group.

Errors: `GROUP_NOT_FOUND`, `GROUP_ALREADY_REVERSED`, `IDEMPOTENCY_CONFLICT`.

#### `convert`

Cross-currency value movement modeled as two coupled groups, one per currency, joined through an FX intermediate account that exists in both currencies (the system maintains an `fx_intermediate` account per currency).

Request:
```json
{ "wallet_id": "wlt_123",
  "from_account_id": 42,            // NGN
  "to_account_id": 99,              // USD
  "amount": 100000,                 // source minor units
  "tags": {"product":"convert"},
  "idempotency_key": "..." }
```

Behavior:
1. Validate accounts; `from_account.currency = source`, `to_account.currency = target`. Reject if same currency.
2. Look up rate from `rates` (latest row where `from_currency=source AND to_currency=target AND valid_from <= NOW() AND (valid_to IS NULL OR valid_to > NOW())`). Pin its `rate` and `id` onto both groups' tags for audit.
3. Compute `target_amount = floor(source_amount * rate)`. Reject if `target_amount = 0`.
4. Resolve `fx_intermediate_account_id_source` and `fx_intermediate_account_id_target` from a `fx_target` lookup config rule (or static table).
5. Generate `group_id_source`, `group_id_target`; both share `fx_pair_group_id = group_id_source`.
6. Compose:
   - **Source-leg row** (currency=source, type=DEBIT, primary=from_account): DEBIT from_account `source_amount`, CREDIT fx_intermediate_source `source_amount`. Status = EXECUTED, prior = NULL.
   - **Target-leg row** (currency=target, type=CREDIT, primary=to_account): DEBIT fx_intermediate_target `target_amount`, CREDIT to_account `target_amount`. Status = EXECUTED, prior = NULL.
7. Lock all four accounts in id order.
8. Verify `from_account.available_balance >= source_amount` and `fx_intermediate_target.available_balance >= target_amount`.
9. `append_transaction_rows([source_leg, target_leg])` atomically.
10. Insert idempotency. Commit.

Errors: `RATE_NOT_FOUND`, `SAME_CURRENCY_CONVERT`, `FX_INTERMEDIATE_INSUFFICIENT_FUNDS`, plus the standard set.

## 18. Worked Examples

### 18.1 Lien-then-execute, single currency

Setup: `from = #42 NGN prepaid`, balance/avail = 1000/1000. `to = #99 NGN liability`, balance/avail = 0/0.

`lien(from=42, to=99, amount=1000, currency=NGN)`:
- New group `G1`. Append row R1: status=LIEN, prior=NULL, type=DEBIT, primary=42. Entries: DEBIT 42 1000, CREDIT 99 1000.
- Cache: `42.avail = 0, 99.balance = 1000` (others unchanged).

`execute(G1)`:
- Latest of G1 is R1 (LIEN). Append R2: status=EXECUTED, prior=LIEN, primary=42, type=DEBIT. Entries replicate R1.
- Cache: `42.balance = 0, 99.avail = 1000`.

Final: 42 = 0/0; 99 = 1000/1000.

### 18.2 Skip-lien transfer with fee

Setup: `from=42 (1000/1000)`, `to=99 (0/0)`, `fee_acct=7 (0/0)`. Config: 1% fee on `product=transfer`.

`execute_direct(from=42, to=99, amount=1000, tags={product:"transfer"})`:
- Fee = 10. Group `G2`. Append R1: status=EXECUTED, prior=NULL, type=DEBIT, primary=42. Entries: DEBIT 42 1010, CREDIT 99 1000, CREDIT 7 10.
- Skip-lien deltas (DEBIT row): primary balance + avail both -e.amount; secondary balance + avail both +e.amount.
- Cache: `42 = -10/-10 → 990 → wait, 1000 - 1010 = -10` — INSUFFICIENT_FUNDS (rejected).

Re-run with `from` balance 1500/1500: `42 = 490/490`, `99 = 1000/1000`, `7 = 10/10`.

### 18.3 Reversal of an executed transfer

Continuing 18.2, with the second example (where it succeeded).

`reverse(G2)`:
- Latest of G2 is R1 (EXECUTED, prior=NULL). Append R2: status=REVERSED, prior=EXECUTED, primary=42, type=DEBIT. Entries: CREDIT 42 1010, DEBIT 99 1000, DEBIT 7 10.
- REVERSED-from-EXECUTED deltas (DEBIT row): primary balance + avail both +e.amount; secondary balance + avail both -e.amount.
- Cache: `42 = 1500/1500`, `99 = 0/0`, `7 = 0/0`. Group is terminal.

### 18.4 FX conversion

Setup: `from=42 NGN (100000/100000)`, `to=99 USD (0/0)`, `fx_int_NGN=10 (10000000/10000000)`, `fx_int_USD=11 (10000/10000)`. Rate `NGN→USD = 0.0006666` (i.e., 1500 NGN = 1 USD).

`convert(from=42, to=99, amount=100000)`:
- target_amount = floor(100000 * 0.0006666) = 66.
- Source leg group `Gs`, target leg group `Gt`, both share `fx_pair_group_id = Gs`.
- Source row: type=DEBIT, primary=42. Entries: DEBIT 42 100000, CREDIT 10 100000. Status=EXECUTED, prior=NULL.
- Target row: type=CREDIT, primary=99. Entries: DEBIT 11 66, CREDIT 99 66. Status=EXECUTED, prior=NULL.
- After: `42 = 0/0`, `10 = 10100000/10100000`, `11 = 9934/9934`, `99 = 66/66`.

## 19. Errors

| Code | HTTP | When |
| --- | --- | --- |
| `INVALID_REQUEST` | 400 | Malformed request body |
| `INVALID_AMOUNT` | 400 | Amount ≤ 0 or > i64 max |
| `INVALID_TAG` | 400 | Tag key not in vocabulary or value not in `allowed_values` |
| `CURRENCY_NOT_FOUND` | 400 | Unknown currency code |
| `CURRENCY_MISMATCH` | 400 | Entries reference different currencies on the same row |
| `UNBALANCED_ROW` | 400 | Per-row DEBIT total ≠ CREDIT total |
| `INVALID_STATUS_TRANSITION` | 400 | (status, prior_status) not allowed |
| `SAME_CURRENCY_CONVERT` | 400 | `convert` called with from/to in same currency |
| `WALLET_NOT_FOUND` | 404 | |
| `ACCOUNT_NOT_FOUND` | 404 | |
| `GROUP_NOT_FOUND` | 404 | |
| `RATE_NOT_FOUND` | 404 | No active rate for the pair |
| `CONFIG_RULE_MISSING` | 404 | Required lookup rule has no match |
| `ACCOUNT_FROZEN` | 409 | Account `status` is `frozen` or `closed` |
| `INSUFFICIENT_FUNDS` | 409 | Debit would overdraw a prepaid account |
| `FX_INTERMEDIATE_INSUFFICIENT_FUNDS` | 409 | FX intermediate has no liquidity for the pair |
| `GROUP_ALREADY_REVERSED` | 409 | Cannot transition a REVERSED group |
| `GROUP_PRECONDITION_FAILED` | 409 | Group's latest row doesn't match expected `prior_status` |
| `IDEMPOTENCY_CONFLICT` | 409 | Same key, different request or wallet |
| `LOCK_TIMEOUT` | 503 | Could not acquire row locks within timeout |
| `INTERNAL_ERROR` | 500 | Unexpected |

Error body shape:
```json
{ "error": {
  "code": "INSUFFICIENT_FUNDS",
  "message": "Account 42 has 500 NGN available; needs 1000",
  "details": { "account_id": 42, "available": 500, "requested": 1000, "currency": "NGN" }
}}
```

## 20. Observability

### Metrics (per Layer 1 call)

- `wallet_layer1_call_total{op,outcome}` — counter, op∈{lien,execute,execute_direct,reverse,convert}, outcome∈{ok,error_code}.
- `wallet_layer1_call_duration_seconds{op}` — histogram.
- `wallet_idempotency_replay_total{op}` — counter for cache hits.
- `wallet_lock_wait_seconds{op}` — histogram.
- `wallet_balance_total{currency,wallet_id,class}` — gauge of summed balances (sampled).

### Structured logs

Every Layer 1 call emits one structured log line on success/failure with:
- `op`, `wallet_id`, `idempotency_key`, `group_id` (or both for FX), `outcome`, `duration_ms`, `error_code` (if any).

### Traces

Span per Layer 1 call. Sub-spans: `validate`, `config_lookup`, `db_txn` (with sub-spans `lock`, `insert_rows`, `update_cache`, `idempotency`).

### Reconcile alarms

- `wallet_reconcile_drift_total{account_id,currency}` — counter incremented when cache disagrees with projection.
- Page on any drift; investigate before allowing further writes against the affected account.

## 21. Operational Notes

### Current-state queries

- Index `transactions(group_id, created_at DESC, id DESC)`. Reduces "latest row per group" to an index lookup.
- SQL view `current_transaction_state` uses `DISTINCT ON (group_id)`; callers query the view.
- Refresh-based MV of "latest row per group" for reporting workloads where minute-level staleness is acceptable.
- A denormalized `group_current_state` table is reserved for if the index stops scaling. It carries no value data and is rebuildable from the log.

### Live balance

The cache stays. Materialized views and on-the-fly computation were rejected for the live-balance path:
- Refresh-based MVs introduce a staleness window that breaks overdraft prevention.
- Streaming/incremental MVs (pg_ivm, Materialize) are functionally equivalent to the cache plus extension/service overhead.
- On-the-fly degrades linearly with history depth and still requires a serialization point for overdraft.

MVs remain appropriate for derived/aggregate views (statements, daily snapshots, fee revenue, lien summaries).

### Write transaction shape

- All reads outside the DB transaction (idempotency, config, rates, group state).
- Batch all inserts (`INSERT … VALUES (…),(…),…`).
- Lock accounts in `id` order; `LOCK_TIMEOUT` bound (default 5s).
- Idempotency in the same statement as writes (`INSERT … ON CONFLICT DO NOTHING RETURNING …`).
- Shard by `wallet_id` once a single primary outgrows capacity.
- Read replicas for non-authorization reads. Write-time overdraft checks stay on the primary.

Avoid: async cache updates (break read-after-write and create overdraft windows); skipping idempotency on calls that "obviously" can't be retried.

### Reconciliation

- Incremental against a per-account `reconcile_watermark` (highest `transactions.id` reconciled). O(new entries).
- Run on a read replica.
- Streaming shadow-balance via CDC for high-volume wallets; alert on drift in near real-time.
- Sampling backstop: full reconcile of a random 1% daily, full reconcile weekly.

### GDPR / data retention

- Anonymize, don't delete. Replace user-identifying metadata on `accounts` (and the upstream user record) with anonymized placeholders. Keep `account_id` and the entire transaction history.
- Reason: deleting an account's entries breaks the counterparty's matching debit/credit; financial-record retention obligations (typically 5–7 years) override GDPR via "compliance with a legal obligation."
- Delete freely from systems where retention does not apply (CRM, support, analytics).

### Importing from external systems

- Use `execute_direct` to synthesize each imported transaction as a single skip-lien row.
- Establish starting balances by appending a single opening-balance transaction per account against an `import_source` ledger account, so the log is self-contained from day one.

### Cognitive overhead

Document the core invariants in onboarding. Enforce via:
- DB-level RULES rejecting UPDATE/DELETE on `transactions` and `entries`.
- CHECK constraint on `(status, prior_status)` combinations.
- Code-review attention on any path proposing mutation of these tables.

## 22. Test Plan

### Unit

- DDL: every CHECK constraint and RULE behaves as specified.
- Validation: each error condition in `append_transaction_rows` returns the right code.
- Balance Computation Model: for every (type, status, prior_status) row in §11, verify the cache delta against a hand-computed expectation.
- Config rule resolution: lookup priority + tie-break; accumulating ordering; missing-rule error.
- Idempotency: hash determinism over canonical JSON; replay returns cached response; conflict detection.

### Integration (real Postgres)

- Lien → execute → assert cache and entries.
- Lien → reverse → assert cache and entries.
- Skip-lien execute → reverse → assert cache and entries.
- Execute on an already-EXECUTED group → `INVALID_STATUS_TRANSITION`.
- Reverse on an already-REVERSED group → `GROUP_ALREADY_REVERSED`.
- Reverse on a group whose latest row was just appended by another tx (race) → exactly one wins, the other returns `GROUP_PRECONDITION_FAILED`.
- Two concurrent debits draining one prepaid account → exactly one fails with `INSUFFICIENT_FUNDS`.
- FX `convert` → both legs commit; rolling back one (forced) rolls back both.
- Idempotency replay across process restarts.
- Reconcile job: replay log into a scratch DB and assert `accounts.balance` matches.

### Property tests

- For any randomly generated valid event sequence on a group: cache update applied per-row equals replaying entries.
- For any random row: `SUM(DEBIT entries) == SUM(CREDIT entries)`.
- For any random group ending in REVERSED: net cache delta across all rows equals zero.

### Load

- 1k concurrent `lien` against distinct accounts: throughput, p99 latency.
- 100 concurrent `lien` against the same account: contention, no overdrafts, no deadlocks.

## 23. Implementation Order

1. **Skeleton.** Migrations, currencies + wallets + accounts + tag_definitions tables. Bare DDL with CHECK and RULES for transactions/entries. Reference-data seed.
2. **Layer 0 reads.** `get_account`, `list_accounts`, `get_transaction_row`, `get_group`, `list_*`. `current_transaction_state` view. Basic auth.
3. **Layer 0 writes.** `create_accounts`. `append_transaction_rows` with full validation, locking, cache update. Property tests for the Balance Computation Model.
4. **Idempotency middleware.** Shared by all Layer 1 calls.
5. **Layer 1: `execute_direct`.** Simplest — no group lookup, no state machine across rows. End-to-end test.
6. **Layer 1: `lien` + `execute`.** Introduces group state machine. Concurrency tests.
7. **Layer 1: `reverse`.** Extends state machine. Reversal property tests.
8. **Config + tag-driven entries.** Materialize fees/taxes/ledger postings inside `lien`/`execute_direct`.
9. **Rates + Layer 1 `convert`.** FX flow with two coupled groups.
10. **Reconcile job.** Incremental + sampling. Drift alarms.
11. **Materialized views.** Daily balance snapshots, fee revenue, lien summaries.
12. **Operational hardening.** Sharding readiness, structured logs/traces/metrics, runbooks for: stuck lien, idempotency conflict storm, drift detected, FX intermediate dry.
