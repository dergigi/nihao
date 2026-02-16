# nihao 👋

**Nostr Identity Health-check Automation & Optimization**

A CLI tool to set up and verify Nostr identities. Agent-friendly by design.

## What it does

Running `nihao` with no arguments creates a fully-equipped Nostr identity:

- 🔑 Generate keypair
- 👤 Publish profile metadata (kind 0)
- 📡 Publish relay list (kind 10002)
- 👥 Publish follow list (kind 3)
- 💰 Set up NIP-60 Cashu wallet
- ⚡ Register npub.cash lightning address

Running `nihao check` audits an existing identity and gives it a health score.

## Install

```bash
go install github.com/dergigi/nihao@latest
```

## Usage

```bash
# Full identity setup with sane defaults
nihao

# Non-interactive setup
nihao --name "MyAgent" --about "I do things" --json

# Check an existing identity
nihao check <npub>

# Check your own identity
nihao check
```

## Built with

- [nak](https://github.com/fiatjaf/nak) — the nostr army knife (used as a Go library)
- [go-nostr](https://pkg.go.dev/fiatjaf.com/nostr) — Nostr protocol library for Go

## License

MIT
