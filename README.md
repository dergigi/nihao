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
- [ ] NIP-60 Cashu wallet setup
- [ ] NIP-05 setup assistance

### Check (`nihao check <npub>`)

- [x] Profile metadata (kind 0) with completeness breakdown
- [x] Profile image health (404 detection, file size, Blossom hosting)
- [x] NIP-05 verification (live HTTP check)
- [x] Lightning address verification (LNURL resolution)
- [x] Relay list (kind 10002)
- [x] Follow list (kind 3)
- [x] NIP-60 wallet detection (kind 37375)
- [x] Health score (0–6)
- [x] `--json` output
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

## Built with

- [go-nostr](https://pkg.go.dev/fiatjaf.com/nostr) — the library that powers [nak](https://github.com/fiatjaf/nak)

## License

MIT
