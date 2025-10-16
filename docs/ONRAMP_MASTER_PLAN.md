# ONRAMP Master Plan for Stablenode Aggregator + Provision Node

This plan adds fiat-to-crypto onramp flows alongside the existing crypto-to-fiat offramp, without breaking current behavior. It introduces new data models, routes, services, and a staged rollout guarded by feature flags.

- Codebase: `rails/aggregator/`
- Key existing components referenced:
  - Controllers: `controllers/sender/sender.go`, `controllers/provider/provider.go`, `controllers/index.go`
  - Routers: `routers/index.go`, `routers/router.go`
  - Services: `services/engine.go`, `services/indexer/evm.go`, `services/common/order.go`
  - Ent: `ent/schema/...`
  - Docs: `docs/WEBHOOK_IMPLEMENTATION_SUMMARY.md`, `docs/TOKEN_FLOW_EXPLAINED.md`, `docs/PROVISION_NODE_SPEC.md`
- Provision node project: `provision-node/` with docs in `docs/PROVISION_NODE_SPEC.md`, `docs/PROVISION_NODE_GO_GUIDE.md`

---

## Goals

- Add onramp (user pays fiat → receives crypto).
- Keep offramp intact; avoid regression.
- Leverage provider nodes for PSP integration and on-chain sending.
- Minimize schema churn in existing offramps (add new, isolated Ent schemas).
- Strong auth and idempotency across aggregator ↔ provider node.

---

## High-Level Onramp Flow

```mermaid
sequenceDiagram
  participant U as User
  participant A as Aggregator (API)
  participant DB as Aggregator DB
  participant PN as Provider Node (PSP+Wallet)
  participant PSP as PSP/Bank
  participant CH as Chain (EVM)

  U->>A: POST /v1/sender/onramps (token, network, fiatCurrency, amountFiat|amountToken, wallet, Idempotency-Key)
  A->>DB: Create OnrampOrder (status: initiated, fiat_currency_id set)
  A->>A: Select provider (supports fiatCurrency, token/network)
  A->>A: Fetch/compute rate (fiatCurrency per token)
  A->>A: Compute amountToken|amountFiat using rate
  A->>A: Validate slippage vs provider terms
  A->>CH: RPC balanceOf(providerWallet) for liquidity
  A->>DB: Create reservation (providerId, amountToken, TTL)
  alt No provider meets liquidity/rules
    A-->>U: 409 No provider liquidity for requested amount
  else Provider found
    A->>PN: POST /onramps (prepare: token, network, fiatCurrency, amountFiat|amountToken, wallet, quotedRate=rate) [HMAC + Idempotency-Key]
    PN->>PSP: Create virtual account/reference (currency=fiatCurrency)
    PN-->>A: deposit details (account_no, bank_code, account_name, psp_ref, currency, expires_at)
    A->>DB: Save rate, deposit details; status: awaiting_deposit
    A-->>U: Deposit instructions (account_no, bank_code, ref, expires_at, currency, rate)
  end

  Note over U,PSP: User deposits fiat -> PSP posts webhook to Provider Node
  PSP-->>PN: Webhook: deposit confirmed (amountFiat, currency, pspReference)
  PN->>A: POST /v1/provider/onramps/:id/fiat-confirm (pspReference, amountFiat, currency) [HMAC + Idempotency-Key]
  A->>DB: Mark fiat_confirmed; extend reservation TTL

  A->>PN: POST /onramps/:id/fulfill (toAddress, token, amountToken) [HMAC + Idempotency-Key]
  PN->>CH: Send ERC20 from provider wallet to user address
  PN-->>A: POST /v1/provider/onramps/:id/complete (txHash, blockNumber) [HMAC + Idempotency-Key]
  A->>CH: Optional verify transfer (indexer/poll/rpc)
  A->>DB: Mark settled; persist OnrampTransfer; write TransactionLog
  A-->>U: GET /v1/sender/onramps/:id returns status, tx, fiatCurrency, rate, fee

  opt Expiry/Cancel
    A->>DB: On deposit TTL expiry -> cancel order; release reservation
    A-->>U: 408 Deposit window expired
  end

  Note over A,PN: All POSTs signed with HMAC and Idempotency-Key for idempotency
```


## Data Model (New Ent Schemas)

Create new ent entities instead of modifying `PaymentOrder` to keep offramp code stable.

  - `ent/schema/onramporder.go`
  - id (UUID, PK)
  - status (enum: initiated, awaiting_deposit, fiat_confirmed, transfer_pending, settled, cancelled, expired, failed, refunded)
  - sender_profile_id (FK → `SenderProfile`)
  - provider_profile_id (nullable FK → `ProviderProfile`) // chosen provider
  - token_id (FK → `Token`)
  - network_id (FK → `Network`)
  - fiat_currency_id (FK → `FiatCurrency`)
  - amount_fiat (decimal)
  - amount_token (decimal)
  - rate (decimal) // normalized semantics; document below
  - fee_percent, fee_amount (decimal)
  - user_wallet_address (string)
  - psp_name (string)
  - deposit_reference (string)
  - deposit_details (json) // account_no, bank_code, webhook hints
  - deposit_expires_at (time)
  - tx_hash (string)
  - block_number (int64)
  - metadata (json)
  - created_at, updated_at

- `ent/schema/fiatdeposit.go` (optional granular audit; can also use `transactionlog`)
  - id (UUID)
  - onramp_order_id (FK)
  - psp_reference (string)
  - amount (decimal)
  - currency (string)
  - status (enum: pending, success, failed)
  - raw_payload (json)
  - created_at

- `ent/schema/onramptransfer.go`
  - id (UUID)
  - onramp_order_id (FK)
  - from_address (string) // provider wallet
  - to_address (string) // user wallet
  - amount_token (decimal)
  - tx_hash (string)
  - status (enum: initiated, confirmed, failed)
  - block_number (int64)
  - created_at

- `ent/schema/onrampreservation.go` (optional)
  - id (UUID)
  - provider_profile_id (FK)
  - token_id (FK)
  - amount_token (decimal)
  - status (enum: active, released, expired)
  - reserved_until (time)
  - order_id (FK → OnrampOrder)
  - created_at

Notes:
- Continue to use `transactionlog` for global audit (status: onramp_initiated, onramp_deposit_ready, onramp_fiat_confirmed, onramp_transfer_initiated, onramp_settled, onramp_failed with `Network`, `TxHash`, and `Metadata` context).
- No changes to offramps entities like `PaymentOrder`, `LockPaymentOrder`.

### Rate Semantics

- Persist a single canonical rate field interpreted as "fiat per token" for the requested fiat currency (e.g., NGN/USDC). For an input amount in fiat, compute `amount_token = amount_fiat / rate`. For an input in token, compute `amount_fiat = amount_token * rate`.
- Reuse slippage logic similar to `providerordertoken.RateSlippage`.

---

## Aggregator API Design

### Sender APIs

- POST `/v1/sender/onramps`
  - Request: token, network, fiatCurrency (ISO code, e.g., NGN), amountFiat or amountToken, userWalletAddress, optional reference
  - Behavior:
    - Validate sender profile, token/network, KYC gating if needed.
    - Create `OnrampOrder(status=initiated)`.
    - Call provider node to prepare deposit and receive account details.
    - Update order with `status=awaiting_deposit`, deposit details.
  - Response: onrampOrderId, deposit details, rate, fee, expiresAt.

  - GET `/v1/sender/onramps/:id`
  - Returns full order details: status, deposit info, tx hash, timestamps.

- GET `/v1/sender/onramps`
  - Pagination and filters (status, token, network, fiatCurrency, date).

  Implementation: add new controller file e.g. `controllers/onramp/sender_onramp.go` to keep separation from `controllers/sender/sender.go`.

### Provider APIs (HMAC; reuse provider auth middleware)

- GET `/v1/provider/onramps`
  - List onramp orders relevant to provider (status filtering).

- POST `/v1/provider/onramps/:id/fiat-confirm`
  - Body: pspReference, amount, currency, timestamp
  - Marks order `fiat_confirmed`, writes `FiatDeposit`, confirms or extends reservation.

- POST `/v1/provider/onramps/:id/fulfill`
  - Body: toAddress (must match user wallet), amountToken, timestamp
  - Signals provider node should send tokens. Aggregator marks `transfer_pending`.

- POST `/v1/provider/onramps/:id/complete`
  - Body: txHash, blockNumber, timestamp
  - Aggregator verifies (optional indexer), marks `settled` and logs.

- POST `/v1/provider/onramps/:id/cancel`
  - Body: reason
  - Cancels reservation, order clean-up, status updates.

Implementation: add `controllers/onramp/provider_onramp.go`. Do not overload existing offramp `FulfillOrder` in `controllers/provider/provider.go`.

### Router

Update `routers/index.go` to register (guarded by `ENABLE_ONRAMP`):

- Sender group:
  - `v1/sender/onramps` [POST, GET, GET:id]
- Provider group:
  - `v1/provider/onramps` [GET]
  - `v1/provider/onramps/:id/fiat-confirm` [POST]
  - `v1/provider/onramps/:id/fulfill` [POST]
  - `v1/provider/onramps/:id/complete` [POST]
  - `v1/provider/onramps/:id/cancel` [POST]

---

## Provider Selection & Token Liquidity

- Use `ProviderOrderToken` as the source of capability:
  - `providerordertoken.Address` is the provider on-chain wallet for the token/network.
  - `MinOrderAmount`, `MaxOrderAmount`, `RateSlippage` apply similarly to offramp.
  - Ensure provider supports the requested fiat currency for PSP settlement.

- Check token liquidity via RPC:
  - Query ERC20 `balanceOf(providerordertoken.Address)` using Alchemy or configured RPC (see `services/engine.go` patterns and `types.NewEthClient`).
  - Ensure balance ≥ `amount_token_out + safety_margin`.

- Ranking candidates:
  - Availability flags, KYB status (`providerprofile`), rate competitiveness, historical success.

- Reservation:
  - Option 1 (recommended): Redis key `onramp_reservation_{orderId}` (amount, provider, TTL) for speed + TTL.
  - Option 2: `OnrampReservation` row with `reserved_until`.
  - On cancellation/expiry, release reservation.

- Rates:
  - Reuse `controllers/index.go:GetTokenRate()` flow and `u.ValidateRate` with a new `direction=onramp` param or a wrapper `u.ValidateRateOnramp()` to avoid impacting offramp.

---

## On-Chain Confirmation

- Primary: provider node callback `POST /v1/provider/onramps/:id/complete` with `txHash`.
- Optional verification:
  - Extend indexer (patterned on `services/indexer/evm.go`) to verify an ERC20 Transfer to `user_wallet_address` using `txHash` or address filters.
  - Alternatively, use `services/engine.go:GetContractEventsRPC` with topics = `[Transfer]` and `to=user address`.

---

## Provision Node Changes

Follow `docs/PROVISION_NODE_SPEC.md` and `docs/PROVISION_NODE_GO_GUIDE.md`, adding onramp endpoints and logic.

### Node Endpoints (Provider Node)

- POST `/onramps` (prepare)
  - Request (from Aggregator): orderId, token, network, fiatCurrency (ISO), amountToken, amountFiat, userWalletAddress
  - Response: deposit details (accountNumber, bankCode, accountName, reference, expiresAt)
  - Behavior: create virtual account/reference with PSP; persist; return to aggregator.

- POST `/onramps/:id/fulfill`
  - Trigger on-chain ERC20 transfer from provider wallet to user wallet.
  - Use configured signer: Thirdweb Engine (`ENGINE_BASE_URL`, `ENGINE_ACCESS_TOKEN`) or direct RPC with local key.
  - Return 202; completion will be async.

- PSP Webhook `/webhooks/psp`
  - On deposit success:
    - Update local order.
    - POST to Aggregator `/v1/provider/onramps/:id/fiat-confirm` [HMAC].

- Completion callback
  - After sending ERC20, POST to Aggregator `/v1/provider/onramps/:id/complete` with `txHash`, `blockNumber`.

### Node Config

Add to `.env`:
- PSP keys: `LENCO_*` or other PSP provider.
- On-chain sending:
  - `ENGINE_BASE_URL`, `ENGINE_ACCESS_TOKEN`, `THIRDWEB_SECRET_KEY`
  - or `RPC_URL_*`, `PROVIDER_WALLET_KEY_*` per network (if signing locally)
- `AGGREGATOR_BASE_URL`, `AGGREGATOR_CLIENT_ID`, `AGGREGATOR_SECRET_KEY`

---

## Aggregator–Provider Node Comms

- Aggregator → Node: prepare deposit, fulfill
  - Add `services/onramp/node_client.go` in aggregator:
    - `PrepareOnramp(order)`, `FulfillOnramp(order)`, signed if needed (mutual HMAC or allowlist IP).
- Node → Aggregator: fiat-confirm, complete
  - Reuse HMAC verification (as documented in `docs/PROVISION_NODE_SPEC.md`).

---

## Backward Compatibility

- No changes to `PaymentOrder`, `LockPaymentOrder`, or existing routes.
- New features hidden behind feature flag `ENABLE_ONRAMP`.
- New Ent schemas generate new tables only.
- No impact on `controllers/sender/sender.go` or `controllers/provider/provider.go` logic for offramp.

---

## Environment Variables

Aggregator `.env.example` additions:

```bash
ENABLE_ONRAMP=true
PROVIDER_NODE_TIMEOUT_MS=30000
ONRAMP_DEPOSIT_TTL_MIN=60
ONRAMP_RESERVATION_TTL_MIN=60
# Optional RPCs per network for liquidity checks
RPC_URL_BASE_SEPOLIA=
# If verifying with Alchemy webhooks/polling (see WEBHOOK_IMPLEMENTATION_SUMMARY.md)
ALCHEMY_AUTH_TOKEN=
```

Provision Node `.env` additions:

```bash
# On-chain sender mode
ONCHAIN_SENDER=engine # or local

# Engine mode
ENGINE_BASE_URL=
ENGINE_ACCESS_TOKEN=
THIRDWEB_SECRET_KEY=

# Local signer mode
RPC_URL_BASE_SEPOLIA=
PROVIDER_WALLET_KEY_BASE_SEPOLIA=

# PSP
LENCO_BASE_URL=
LENCO_API_KEY=
LENCO_ACCOUNT_ID=

# Aggregator HMAC
AGGREGATOR_BASE_URL=
AGGREGATOR_CLIENT_ID=
AGGREGATOR_SECRET_KEY=
```

---

## Controller/Service Changes (Aggregator)

- Add:
  - `controllers/onramp/sender_onramp.go`
  - `controllers/onramp/provider_onramp.go`
  - `services/onramp/selection.go` (provider selection, rate calc, reservation)
  - `services/onramp/node_client.go` (calls to provider node)
  - `services/indexer/onramp.go` (optional user receipt verification)
  - `ent/schema/onramporder.go`
  - `ent/schema/fiatdeposit.go`
  - `ent/schema/onramptransfer.go`
  - `ent/schema/onrampreservation.go` (optional)
- Modify:
  - `routers/index.go` to register new routes (guarded by `ENABLE_ONRAMP`)
  - `.env.example` to add onramp vars

No changes to existing offramp-specific implementations:
- `controllers/sender/sender.go`
- `controllers/provider/provider.go`
- `services/common/order.go` (offramp)
- `services/engine.go` (unless reused for verification; avoid tight coupling)

---

## State Machine

- OnrampOrder.status transitions:
  - initiated → awaiting_deposit → fiat_confirmed → transfer_pending → settled
  - Any → cancelled/expired/failed
  - Optional: refunded (if PSP reverses or manual refund)

- Reservation.status:
  - active → released/expired

---

## Security

- HMAC on all aggregator↔provider node POSTs with timestamp window (as per `docs/PROVISION_NODE_SPEC.md`).
- Idempotency-Key header on aggregator POSTs to node and node POSTs to aggregator to avoid double-processing.
- Validate all addresses (`u.IsValidEthereumAddress`), tokens (`Token` exists and enabled), networks.
- Enforce KYC/KYB gates for onramp if required (reuse endpoints in `controllers/index.go`).

---

## Observability

- Transaction logs (`ent/transactionlog`):
  - Write entries for each onramp milestone with `Network`, `TxHash`, and context in `Metadata`.
- Metrics:
  - `onramp_initiated_total`, `onramp_deposit_confirmed_total`, `onramp_settled_total`
  - SLA timers: `deposit_wait_ms`, `transfer_wait_ms`
- Alerts:
  - Deposit confirmed but no transfer after threshold
  - Transfer failed/error

---

## Testing Plan

- Unit tests:
  - Provider selection with on-chain mocked balances.
  - Rate calculation (onramp invert + slippage).
  - State transitions and idempotency.
- Integration:
  - Mock PSP webhook to provider node → aggregator fiat-confirm → fulfill → complete with mock RPC receipt.
- E2E (Base Sepolia):
  - Happy path and edge cases (expired deposit, insufficient liquidity, double webhooks).

---

## Rollout

- Phase 1: Schemas + feature flag + list endpoints that return 501 when disabled.
- Phase 2: Provider node support with mock PSP; internal testing.
- Phase 3: Limited beta with a single provider; enable `ENABLE_ONRAMP=true` per environment.

Runbooks:
- How to manually mark order failed/refund.
- How to rotate keys/secrets.
- How to reverify transfers.

---

## Concrete Schema Sketch (Ent Example)

```go
// ent/schema/onramporder.go
package schema

import (
  "time"
  "entgo.io/ent"
  "entgo.io/ent/schema/field"
  "entgo.io/ent/schema/edge"
  "github.com/google/uuid"
)

type OnrampOrder struct{ ent.Schema }

func (OnrampOrder) Fields() []ent.Field {
  return []ent.Field{
    field.UUID("id", uuid.UUID{}).Default(uuid.New),
    field.Enum("status").Values(
      "initiated","awaiting_deposit","fiat_confirmed","transfer_pending",
      "settled","cancelled","expired","failed","refunded",
    ),
    field.UUID("sender_profile_id", uuid.UUID{}),
    field.String("provider_profile_id").Optional(),
    field.UUID("token_id", uuid.UUID{}),
    field.UUID("network_id", uuid.UUID{}),
    field.UUID("fiat_currency_id", uuid.UUID{}),
    field.String("user_wallet_address"),
    field.String("psp_name").Optional(),
    field.String("deposit_reference").Optional(),
    field.JSON("deposit_details", map[string]any{}).Optional(),
    field.Time("deposit_expires_at").Optional(),
    field.String("tx_hash").Optional(),
    field.Int64("block_number").Optional(),
    field.JSON("metadata", map[string]any{}).Optional(),
    field.Time("created_at").Default(time.Now),
    field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
  }
}

func (OnrampOrder) Edges() []ent.Edge {
  return []ent.Edge{
    edge.From("sender", SenderProfile.Type).Ref("onramp_orders").Unique().Field("sender_profile_id"),
    edge.From("provider", ProviderProfile.Type).Ref("onramp_orders").Unique().Field("provider_profile_id").Optional(),
    edge.From("token", Token.Type).Ref("onramp_orders").Unique().Field("token_id"),
    edge.From("network", Network.Type).Ref("onramp_orders").Unique().Field("network_id"),
    edge.From("fiat_currency", FiatCurrency.Type).Ref("onramp_orders").Unique().Field("fiat_currency_id"),
  }
}
```

---

## File Changes Summary

- New:
  - `docs/ONRAMP_MASTER_PLAN.md` (this doc)
  - `ent/schema/onramporder.go`, `ent/schema/fiatdeposit.go`, `ent/schema/onramptransfer.go`, `ent/schema/onrampreservation.go`
  - `controllers/onramp/sender_onramp.go`, `controllers/onramp/provider_onramp.go`
  - `services/onramp/selection.go`, `services/onramp/node_client.go`, `services/indexer/onramp.go` (optional)
- Modified:
  - `routers/index.go` (route registration behind `ENABLE_ONRAMP`)
  - `.env.example` (new variables)

---

## References

- Offramp sender flow: `controllers/sender/sender.go`
- Offramp provider flows: `controllers/provider/provider.go`
- Engine/Insight helpers: `services/engine.go`
- Indexer patterns: `services/indexer/evm.go`
- Common order logic: `services/common/order.go`
- Webhooks (Alchemy): `docs/WEBHOOK_IMPLEMENTATION_SUMMARY.md`
- Token flow reference: `docs/TOKEN_FLOW_EXPLAINED.md`
- Provider node spec: `docs/PROVISION_NODE_SPEC.md` / `docs/PROVISION_NODE_GO_GUIDE.md`
