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

# Check an existing identity
nihao check <npub>
```

## Features

### Setup (`nihao`)

- [x] Generate keypair (or use `--sec`)
- [x] Publish profile metadata (kind 0)
- [x] Publish relay list (kind 10002)
- [x] Publish follow list (kind 3)
- [x] `--json` output for agent consumption
- [ ] NIP-60 Cashu wallet setup
- [ ] npub.cash registration
- [ ] NIP-05 setup assistance
- [ ] Lightning address setup
- [x] Parallel relay publishing

### Check (`nihao check <npub>`)

- [x] Profile metadata (kind 0)
- [x] NIP-05 verification (live DNS + HTTP check)
- [x] Lightning address verification (LNURL resolution)
- [x] Relay list (kind 10002)
- [x] Follow list (kind 3)
- [x] NIP-60 wallet detection
- [x] Health score
- [x] `--json` output
- [x] Meaningful exit codes (0 = healthy, 1 = issues)
- [x] Profile completeness breakdown (fields present/missing)
- [ ] Relay quality analysis

### General

- [x] Single binary, zero dependencies
- [x] Non-interactive by default
- [x] Meaningful exit codes
- [x] Stdin support (pipe nsec)
- [ ] `nihao check` without args (read local key)
- [ ] OpenClaw skill wrapper

## Built with

- [go-nostr](https://pkg.go.dev/fiatjaf.com/nostr) — the library that powers [nak](https://github.com/fiatjaf/nak)

## License

MIT
