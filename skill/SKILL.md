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

This single command:
1. Generates a keypair (or use `--sec` / `--stdin` for an existing key)
2. Publishes profile metadata (kind 0)
3. Publishes relay list (kind 10002) to default relays
4. Publishes follow list (kind 3)
5. Sets up a NIP-60 Cashu wallet (kind 17375 + kind 10019)
6. Auto-sets lud16 to `<npub>@npub.cash`
7. Posts a first note with `#nihao` hashtag

### Key Flags

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
| `--no-wallet` | Skip NIP-60 wallet setup |
| `--sec <nsec>` | Use existing key |
| `--stdin` | Read key from stdin |
| `--nsec-cmd <cmd>` | Delegate key storage to an external tool (e.g. `pass`, `age`) |
| `--json` | JSON output (for parsing) |
| `--quiet` | Suppress all non-JSON, non-error output |

### Key Storage

Use `--nsec-cmd` to delegate key storage to a password manager. The tool receives the key on stdin. Runs before publishing — if it fails, nihao aborts and nothing is published.

Examples:

```bash
nihao --name "mybot" --nsec-cmd "pass insert -e nostr/mybot"
nihao --name "mybot" --nsec-cmd "age -r age1... -o key.age"
```

## Check — Audit an Existing Identity

```bash
nihao check <npub> --json
```

Checks and scores (0–8):
- Profile metadata completeness (name, about, picture, banner)
- Image health (404 detection, Blossom hosting)
- NIP-05 verification (live HTTP)
- Lightning address (LNURL resolution)
- Relay list (kind 10002)
- Follow list (kind 3)
- NIP-60 wallet + mint validation
- Nutzap info (kind 10019)

Exit codes: `0` = healthy, `1` = issues found.

### JSON Output

Use `--json` for structured output. Setup returns `{npub, nsec, pubkey, relays, profile, wallet}`. Check returns `{npub, score, max_score, checks: [...]}`.

## Agent Workflow

```bash
# Create identity
nihao --name "mybot" --json --quiet > /tmp/nihao-result.json

# Verify identity health
nihao check npub1... --json --quiet
```

Parse JSON output, check exit codes, act on results.
