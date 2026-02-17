# nihao 👋

**Nostr Identity Health-check Automation & Optimization**

A CLI tool to set up and verify Nostr identities. Agent-friendly by design.

## Install

```bash
go install github.com/dergigi/nihao@latest
```

## Usage

```bash
# Full identity setup with sane defaults
nihao

# Non-interactive setup with options
nihao --name "MyAgent" --about "I do things" --json

# Use an existing secret key
nihao --sec nsec1...
echo $NSEC | nihao --stdin --name "MyAgent"

# Setup with custom mints
nihao --mint https://mint.minibits.cash/Bitcoin --mint https://mint.coinos.io

# Store nsec securely via external command
nihao --name "MyAgent" --nsec-cmd "pass insert -e nostr/myagent"
nihao --nsec-cmd "age -r age1... -o ~/.nostr/nsec.age"
nihao --nsec-cmd "secret-tool store --label='nostr nsec' service nostr account default"

# Setup without wallet
nihao --no-wallet

# Check an existing identity
nihao check <npub>
nihao check <npub> --json
```

## Default Relays

New identities are published to a curated set of reliable relays:

- `wss://relay.damus.io` — large, general-purpose
- `wss://relay.primal.net` — large, general-purpose
- `wss://nos.lol` — solid general-purpose
- `wss://purplepag.es` — NIP-65 relay list aggregator (outbox model)

Override with `--relays wss://my.relay,wss://other.relay`.

## Features

### Setup (`nihao`)

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
- [x] `--nsec-cmd` for secure key storage via external command
- [ ] NIP-05 setup assistance

### Check (`nihao check <npub>`)

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
- [ ] Relay quality analysis
- [ ] Dynamic relay discovery (NIP-66 relay monitors)
- [ ] Relay discovery from well-connected npubs (sample kind 10002 lists)

### General

- [x] Single binary, zero dependencies
- [x] Non-interactive by default
- [x] Meaningful exit codes
- [ ] `nihao check` without args (read local key)
- [ ] OpenClaw skill wrapper

## Key Management

nihao does **not** store your nsec. By design, it generates (or accepts) a secret key, uses it to sign events, and then outputs it — but never writes it to disk.

Use `--nsec-cmd` to pipe the nsec to any storage backend:

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

## Built with

- [go-nostr](https://pkg.go.dev/fiatjaf.com/nostr) — the library that powers [nak](https://github.com/fiatjaf/nak)

## Versioning

This project follows [Semantic Versioning](https://semver.org/). See [CHANGELOG.md](CHANGELOG.md) for release history.

## License

MIT
