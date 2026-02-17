# NIP-60 Wallet Setup — Implementation Plan

## Overview

When `nihao` creates a new identity, it should also create a fully functional
NIP-60 Cashu wallet. This means the identity is immediately ready to receive
nutzaps (NIP-61) and hold ecash — zero to full Nostr citizen in one command.

## What We Need to Create

### 1. Wallet Event (kind 17375)

Replaceable event containing encrypted:
- `privkey` — a **separate** secp256k1 private key for P2PK ecash (NOT the nostr key)
- `mint` — one or more mint URLs the wallet trusts

```json
{
    "kind": 17375,
    "content": "<nip44_encrypted([['privkey', '<hex>'], ['mint', 'https://...'], ...])>",
    "tags": []
}
```

### 2. Nutzap Info Event (kind 10019)

Tells the world how to send you nutzaps:
- `relay` — relays to publish nutzaps to
- `mint` — trusted mints (must match wallet mints)
- `pubkey` — the P2PK public key from the wallet privkey

```json
{
    "kind": 10019,
    "tags": [
        ["relay", "wss://relay.damus.io"],
        ["mint", "https://mint.example.com", "sat"],
        ["pubkey", "<p2pk-pubkey-hex>"]
    ]
}
```

## The Hard Part: Mint Selection

A wallet is useless without good mints. Bad mint = lost funds. We need smart
defaults that work without user input but allow overrides.

### Strategy: Tiered Mint Discovery

#### Tier 0 — User Override
```
nihao --mint https://my.mint.com
```
Skip all discovery, use what the user says.

#### Tier 1 — Curated Default (ship now)
A small list of well-known, reliable mints with good uptime:
- `https://mint.minibits.cash/Bitcoin` — Minibits, Nutshell 0.18.2, established
- `https://mint.coinos.io` — Coinos, Nutshell 0.18.1, established
- `https://mint.macadamia.cash` — Macadamia, Nutshell 0.18.2

Selection criteria:
- Must respond to `/v1/info` (alive)
- Must support NUT-11 (P2PK, required for nutzaps)
- Must support NUT-04 (mint via bolt11) and NUT-05 (melt via bolt11)
- Must use `sat` unit
- Prefer mints running recent Nutshell versions

#### Tier 2 — Discovery from Social Graph (later)
Fetch kind 10019 events from followed accounts, count which mints appear most.
Most popular mints among your follows = good defaults.

#### Tier 3 — Mint Auditing (later)
For each candidate mint:
- Fetch `/v1/info` — check version, supported NUTs, contact info
- Fetch `/v1/keysets` — verify active keyset exists with `sat` unit
- Optionally: mint a small amount and immediately melt it (liveness test)

### Mint Validation (must-have for any mint we add)

Before adding a mint to the wallet, verify:
1. **Reachable**: GET `/v1/info` returns 200
2. **Has sat keyset**: GET `/v1/keys` includes a keyset with `unit: "sat"`
3. **Supports P2PK**: NUT-11 listed in supported NUTs (required for nutzaps)
4. **Supports bolt11**: NUT-04 + NUT-05 (mint and melt via Lightning)

If validation fails, skip that mint and try the next one.

## Implementation Steps

### Phase 1: Basic Wallet Creation (MVP)

1. **Generate wallet privkey**
   - Generate a random 32-byte secp256k1 private key (separate from nostr key)
   - Derive the public key (compressed, 02-prefixed for cashu compatibility)

2. **Validate default mints**
   - For each default mint: GET `/v1/info`, check NUT support and sat keyset
   - Filter out any that fail validation
   - Require at least 1 valid mint to proceed

3. **Publish kind 17375 (wallet event)**
   - Encrypt `[["privkey", "<hex>"], ["mint", "<url>"], ...]` with NIP-44
   - Sign and publish to user's relays

4. **Publish kind 10019 (nutzap info)**
   - Add relay tags (user's relays)
   - Add mint tags (validated mints)
   - Add pubkey tag (P2PK public key from wallet privkey)
   - Sign and publish to user's relays

### Phase 2: Enhanced Mint Selection

5. **`--mint` flag** to override defaults
6. **Mint info display** — show mint name/version/contact during setup
7. **`nihao wallet` subcommand** for managing mints post-setup

### Phase 3: Social Mint Discovery

8. **Discover mints from follows** — fetch kind 10019 from followed npubs
9. **Mint popularity ranking** — count occurrences across social graph
10. **Smart defaults** — pick top N mints from social graph

### Phase 4: Wallet Operations (stretch)

11. **`nihao wallet balance`** — show balance across mints
12. **`nihao wallet receive <token>`** — receive cashu tokens
13. **`nihao wallet send <amount>`** — send cashu tokens

## go-nostr API Usage

The nip60 package provides most of what we need:

```go
// Load or create wallet
wallet := nip60.LoadWallet(ctx, keyer, pool, relays, nip60.WalletOptions{})

// Add mints
wallet.AddMint(ctx, "https://mint.minibits.cash/Bitcoin")

// Set P2PK private key
wallet.SetPrivateKey(ctx, hexPrivKey)

// Wallet handles publishing via the PublishUpdate callback
wallet.PublishUpdate = func(evt nostr.Event, ...) {
    pool.PublishMany(ctx, relays, evt)
}
```

For mint validation, use the client package:
```go
info, err := client.GetMintInfo(ctx, mintURL)     // /v1/info
keyset, err := client.GetActiveKeyset(ctx, mintURL) // /v1/keys
keysets, err := client.GetAllKeysets(ctx, mintURL)   // /v1/keysets
```

## NIP-44 Encryption Note

The wallet event (kind 17375) uses NIP-44 encryption. go-nostr's `Keyer`
interface handles this — when we call `kr.Encrypt(ctx, content, pk)` it uses
NIP-44 under the hood. The `wallet.toEvent()` method already does this
correctly.

## Risks & Gotchas

- **Mint trust is real**: A malicious mint can steal funds. Our defaults must
  be mints with reputation and track record.
- **NIP-44 vs NIP-04**: Wallet events use NIP-44 encryption (newer, better).
  Make sure we're using the right Keyer implementation.
- **P2PK key ≠ Nostr key**: The wallet privkey is a SEPARATE key used only for
  P2PK-locking ecash. Never reuse the Nostr signing key.
- **Keyset collisions**: When adding multiple mints, check for keyset ID
  collisions (the go-nostr code already does this in `AddMint`).
- **Kind numbers changed**: Old NIP-60 used kind 37375 (addressable), new spec
  uses 17375 (replaceable). Our check currently looks for 37375 — need to check
  both for backwards compatibility.

## TODO Checklist

### Step 1: Mint validation helper ✅
- [x] Create `mint.go` with `validateMint(ctx, url)` function
- [x] Fetch `/v1/info` — check reachable, parse name/version/supported NUTs
- [x] Fetch `/v1/keys` — verify active sat-denominated keyset exists
- [x] Check NUT-11 (P2PK) support — required for nutzaps
- [x] Check NUT-04 + NUT-05 (bolt11 mint/melt) support
- [x] Return structured MintInfo with validation result

### Step 2: Default mint list with validation ✅
- [x] Define curated default mints (minibits, coinos, macadamia)
- [x] On setup, validate each default mint, filter to working ones
- [x] Require at least 1 valid mint to proceed with wallet creation
- [x] Add `--mint` flag to override defaults

### Step 3: Generate wallet privkey ✅
- [x] Generate random 32-byte secp256k1 private key (NOT the nostr key)
- [x] Derive compressed public key (02-prefixed for cashu P2PK compat)

### Step 4: Publish wallet event (kind 17375) ✅
- [x] Build encrypted content: `[["privkey", "<hex>"], ["mint", "<url>"], ...]`
- [x] Use NIP-44 encryption via go-nostr Keyer interface
- [x] Sign and publish to user's relays

### Step 5: Publish nutzap info (kind 10019) ✅
- [x] Add `relay` tags (user's configured relays)
- [x] Add `mint` tags (validated mints, with "sat" unit marker)
- [x] Add `pubkey` tag (P2PK public key from wallet privkey)
- [x] Sign and publish to user's relays

### Step 6: Update nihao check ✅
- [x] Check for kind 17375 (new) AND kind 37375 (old) for backwards compat
- [x] If wallet found, verify mints are still reachable
- [x] Check for kind 10019 (nutzap info) — warn if missing
- [x] Show mint names/URLs in check output

### Step 7: Wire it all together ✅
- [x] Integrate wallet setup into main `nihao` setup flow
- [x] Show wallet info in setup summary (mints, P2PK pubkey)
- [x] Include wallet details in `--json` output
- [x] Test end-to-end: create identity, check it, verify wallet events on relays
- [x] `--no-wallet` flag to skip wallet setup

### Step 8: Release ✅ (partial — ready for review)
- [x] Update README with wallet features
- [x] Update CHANGELOG.md
- [ ] Tag v0.6.0
- [ ] Dogfood: create fresh identity, verify wallet works with nak

## Open Questions

1. Should we mint a tiny amount (1 sat) during setup to verify the full flow?
   Pro: proves everything works. Con: requires Lightning payment.
2. How many mints should we add by default? 1 is simplest. 2-3 gives
   redundancy. More than 3 is probably unnecessary.
3. Should `nihao check` verify that wallet mints are still alive and have
   valid keysets?
