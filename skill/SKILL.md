---
name: nihao
description: Set up and verify Nostr identities using the nihao CLI. Use when creating new Nostr keypairs/profiles, auditing existing npub health, or helping users get started on Nostr. Covers keypair generation, profile metadata (kind 0), relay lists (kind 10002), follow lists (kind 3), NIP-60 Cashu wallets, NIP-05, lightning addresses, and identity health scoring.
---

# nihao 👋

Nostr identity setup and health-check CLI. Single binary, non-interactive, agent-friendly.

Source: https://github.com/dergigi/nihao

## Install

```bash
go install github.com/dergigi/nihao@latest
```

Verify: `nihao version`

## Setup — Create a New Identity

```bash
nihao --name "AgentName" --about "I do things" --json
```

What this does:
1. Generates a Nostr keypair
2. Publishes profile metadata (kind 0)
3. Publishes relay list (kind 10002)
4. Publishes follow list (kind 3)
5. Sets up a NIP-60 Cashu wallet
6. Sets lightning address to `<npub>@npub.cash`
7. Posts a first note with `#nihao` hashtag

### Flags

| Flag | Purpose |
|---|---|
| `--name <name>` | Display name |
| `--about <text>` | Bio |
| `--picture <url>` | Profile picture URL |
| `--banner <url>` | Banner image URL |
| `--nip05 <user@domain>` | NIP-05 identifier |
| `--lud16 <user@domain>` | Lightning address (default: npub@npub.cash) |
| `--relays <r1,r2,...>` | Override default relays |
| `--mint <url>` | Custom Cashu mint (repeatable) |
| `--no-wallet` | Skip wallet setup |
| `--json` | JSON output for parsing |
| `--quiet` | Suppress non-JSON output |

### Key Management

nihao supports `--nsec-cmd` to delegate key storage to a password manager like `pass` or `age`. See the project README for details: https://github.com/dergigi/nihao#key-management

## Check — Audit an Existing Identity

```bash
nihao check npub1... --json
```

Checks and scores (0–8):
- Profile metadata completeness (name, about, picture, banner)
- Image health (404 detection, Blossom hosting)
- NIP-05 verification (live HTTP)
- Lightning address (LNURL resolution)
- Relay list (kind 10002)
- Follow list (kind 3)
- NIP-60 wallet and mint validation
- Nutzap info (kind 10019)

Exit codes: `0` = healthy, `1` = issues found.

## JSON Output

Use `--json` for structured results. Parse the output and check exit codes to act on results.
