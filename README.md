# nihao 👋

**Self-sovereign identity in one command.**

nihao gives any human or agent a complete Nostr identity — keypair, profile, relay
list, lightning address, and a Cashu ecash wallet — in a single command. No accounts.
No API keys. No KYC. No permission from anyone.

One command. One binary. Unstoppable identity. Unstoppable money.

## Why

Every agent needs an identity. Every identity needs money. Today, both require
permission — platform accounts, API keys, KYC gates. nihao removes the gatekeepers:

- **Identity** via [Nostr](https://nostr.com) — censorship-resistant, self-sovereign, based on public-key cryptography
- **Money** via [Cashu](https://cashu.space) + [Lightning](https://lightning.network) — bearer ecash, no bank account needed, works globally
- **Discovery** via relay lists and NIP-05 — your identity is portable, not locked to any platform

An agent that runs `nihao` walks away with a keypair, a profile on the relay
network, a lightning address that can receive payments, and a wallet to hold
and spend ecash. That's self-sovereignty in about 3 seconds.

## Install

```bash
go install github.com/dergigi/nihao@latest
```

## Quick Start

```bash
# One command. Full identity. No questions asked.
nihao --name "satoshi" --json

# Bring your own key
nihao --sec nsec1... --name "satoshi"
echo $NSEC | nihao --stdin --name "satoshi"

# Custom mints for your wallet
nihao --mint https://mint.minibits.cash/Bitcoin --mint https://mint.coinos.io

# Store your nsec to a file (0600 perms)
nihao --name "satoshi" --nsec-file ./nsec.key

# Or pipe to a password manager
nihao --name "satoshi" --nsec-cmd "pass insert -e nostr/satoshi"

# Skip the wallet if you just need identity
nihao --no-wallet

# Audit any npub's identity health
nihao check npub1...
nihao check npub1... --json
```

## Default Relays

New identities publish a kind 10002 relay list with proper NIP-65 read/write markers:

- `wss://relay.damus.io` — read+write (general-purpose)
- `wss://relay.primal.net` — read+write (general-purpose)
- `wss://nos.lol` — read+write (general-purpose)
- `wss://purplepag.es` — used for publishing outbox events, but NOT advertised in kind 10002 (it's a relay list aggregator)

DM relays (kind 10050, per NIP-17):

- `wss://inbox.nostr.wine` — auth-required inbox relay
- `wss://auth.nostr1.com` — auth-required inbox relay

Override with `--relays`, `--dm-relays`, or use `--discover` to automatically find
relays from well-connected npubs.

## What You Get

### Setup (`nihao`) — from zero to full identity

- [x] Generate keypair (or use `--sec` / `--stdin`)
- [x] Publish profile metadata (kind 0)
- [x] Publish relay list (kind 10002)
- [x] Publish follow list (kind 3)
- [x] Post first note (kind 1) with `#nihao` hashtag
- [x] Auto-set lud16 to `<npub>@npub.cash` (no registration needed)
- [x] Randomized multilingual greeting (26 languages)
- [x] Parallel relay publishing
- [x] `--json` output for agent consumption
- [x] `--quiet` mode for agent consumption
- [x] NIP-60 Cashu wallet setup (kind 17375 + kind 10019)
- [x] Mint validation (NUT-04, NUT-05, NUT-11, sat keyset)
- [x] `--mint <url>` flag to override default mints
- [x] `--no-wallet` flag to skip wallet setup
- [x] `--nsec-file` for AV-friendly key storage to file
- [x] `--nsec-cmd` / `--nsec-exec` for secure key storage via external command
- [x] `--discover` flag to find relays from well-connected npubs
- [x] Relay kind filtering (specialized relays only get compatible events)
- [x] NIP-65 read/write markers on kind 10002 relay list
- [x] Kind 10050 DM relay list (NIP-17) with `--dm-relays` / `--no-dm-relays`
- [ ] NIP-05 setup assistance

### Check (`nihao check <npub>`) — audit any identity

- [x] Profile metadata (kind 0) with completeness breakdown
- [x] Profile image health (404 detection, file size, Blossom hosting)
- [x] NIP-05 verification (live HTTP check)
- [x] Lightning address verification (LNURL resolution)
- [x] Relay list (kind 10002)
- [x] Follow list (kind 3)
- [x] NIP-60 wallet detection (kind 17375 + kind 37375 backwards compat)
- [x] Wallet mint validation (reachability, name, NUT support)
- [x] Nutzap info (kind 10019) detection with missing-warning
- [x] Health score (0–8)
- [x] Parallel relay fetching
- [x] `--json` output
- [x] `--quiet` mode for agent consumption
- [x] Meaningful exit codes (0 = healthy, 1 = issues found)
- [x] Relay quality analysis (NIP-11, latency, reachability scoring)
- [x] Relay discovery from well-connected npubs (sample kind 10002 lists)
- [x] NIP-65 relay marker analysis (warn if all bare)
- [x] Kind 10050 DM relay detection
- [x] Relay purpose display in detail output
- [ ] Dynamic relay discovery (NIP-66 relay monitors)

### General

- [x] Single binary, zero dependencies
- [x] Non-interactive by default
- [x] Meaningful exit codes
- [x] OpenClaw skill wrapper

## Key Management

nihao does **not** store your nsec. By design, it generates (or accepts) a secret key, uses it to sign events, and then outputs it — but never writes it to disk unless you ask.

### Simple: write to file

```bash
nihao --name "satoshi" --nsec-file ./nsec.key
```

The file is created with `0600` permissions (owner read/write only). No shell execution involved.

### Advanced: pipe to a password manager

Use `--nsec-cmd` (or `--nsec-exec`) to pipe the nsec to any storage backend:

```bash
# GNU pass (GPG-encrypted, git-friendly)
nihao --nsec-cmd "pass insert -e nostr/myidentity"

# age (simple file encryption)
nihao --nsec-cmd "age -r age1abc... -o ~/keys/nostr.age"

# Linux keyring (GNOME Keyring / KDE Wallet)
nihao --nsec-cmd "secret-tool store --label='nostr' service nostr account default"

# macOS Keychain
nihao --nsec-cmd "security add-generic-password -a nostr -s nsec -w \$(cat)"

# 1Password (op CLI)
nihao --nsec-cmd "op item create --category=password --title='nostr nsec' password=\$(cat)"

# Bitwarden (bw CLI — must be unlocked first)
nihao --nsec-cmd "bw create item \$(jq -n --arg n \"\$(cat)\" '{type:2,secureNote:{type:0},name:\"nostr-nsec\",notes:\$n}' | bw encode)"

# KeePassXC (keepassxc-cli)
nihao --nsec-cmd "keepassxc-cli add -q ~/Passwords.kdbx nostr/nsec"

# Hashicorp Vault
nihao --nsec-cmd 'vault kv put secret/nostr nsec=$(cat)'

# gopass (pass-compatible, written in Go)
nihao --nsec-cmd "gopass insert -f nostr/nsec"

# Simple file (chmod 600 — least secure, but works everywhere)
nihao --nsec-cmd "tee ~/.nostr/nsec > /dev/null && chmod 600 ~/.nostr/nsec"
```

The command receives the nsec on **stdin** (one line, followed by EOF). It runs through `sh -c`, so pipes and redirections work. If the command exits non-zero, nihao aborts before publishing anything.

### For Agents

Agents should always use `--nsec-cmd` (or `--json` and handle storage themselves). Example with `pass`:

```bash
nihao --name "my-bot" --json --nsec-cmd "pass insert -e nostr/my-bot"
```

The nsec is stored *before* any events are published, so if storage fails, no identity is created on relays.

### Retrieving Your nsec

To use a stored nsec with nihao later (e.g. for updates), pipe it back in via `--stdin`:

```bash
# GNU pass / gopass
pass nostr/myidentity | nihao --stdin --name "NewName"

# 1Password
op item get 'nostr nsec' --fields password | nihao --stdin --name "NewName"

# Bitwarden
bw get notes nostr-nsec | nihao --stdin --name "NewName"

# KeePassXC
keepassxc-cli show -s ~/Passwords.kdbx nostr/nsec | nihao --stdin --name "NewName"

# Hashicorp Vault
vault kv get -field=nsec secret/nostr | nihao --stdin --name "NewName"

# age-encrypted file
age -d ~/keys/nostr.age | nihao --stdin --name "NewName"
```

## The Stack

- **[Nostr](https://nostr.com)** — the protocol. Censorship-resistant social identity based on secp256k1 keys and relays.
- **[Cashu](https://cashu.space)** — Chaumian ecash on Bitcoin. Bearer tokens with blind signatures for privacy.
- **[Lightning](https://lightning.network)** — instant Bitcoin payments. Every nihao identity gets a lightning address out of the box.
- **[go-nostr](https://pkg.go.dev/fiatjaf.com/nostr)** — the Go library that powers [nak](https://github.com/fiatjaf/nak). nihao uses it directly as a dependency.

## Philosophy

**No gatekeepers.** Identity should not require permission. Money should not require a bank account. nihao exists because agents (and humans) deserve self-sovereignty by default.

**Agent-first, human-friendly.** Every command works non-interactively with `--json` output and meaningful exit codes. But it's just as useful for a human setting up their first Nostr identity.

**Single binary, zero dependencies.** `go install` and you're done. No Docker, no config files, no databases.

## Versioning

[Semantic Versioning](https://semver.org/). See [CHANGELOG.md](CHANGELOG.md) for release history.

## License

MIT — free as in freedom.
