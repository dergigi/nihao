---
name: nihao
description: Set up and verify Nostr identities using the nihao CLI. Use when creating new Nostr keypairs/profiles, auditing existing npub health, or helping users get started on Nostr. Covers keypair generation, profile metadata (kind 0), relay lists (kind 10002), follow lists (kind 3), NIP-60 Cashu wallets, NIP-05, lightning addresses, and identity health scoring.
---

# nihao 👋

Nostr identity setup and health-check CLI. Single binary, non-interactive, agent-friendly.

## Install

```bash
go install github.com/dergigi/nihao@latest
```

Verify: `nihao version`

If `go` is unavailable, download the binary from https://github.com/dergigi/nihao/releases.

## Setup — Create a New Identity

```bash
nihao --name "AgentName" --about "I do things" --json --nsec-cmd "pass insert -e nostr/agentname"
```

This single command:
1. Generates a keypair (or use `--sec`/`--stdin` for existing key)
2. Publishes profile metadata (kind 0)
3. Publishes relay list (kind 10002) to default relays
4. Publishes follow list (kind 3)
5. Sets up a NIP-60 Cashu wallet (kind 17375 + kind 10019)
6. Auto-sets lud16 to `<npub>@npub.cash`
7. Posts a first note with `#nihao` hashtag
8. Pipes nsec to `--nsec-cmd` for secure storage (runs before publishing — aborts on failure)

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
| `--sec <nsec\|hex>` | Use existing secret key |
| `--stdin` | Read secret key from stdin |
| `--nsec-cmd <cmd>` | Pipe nsec to command for secure storage |
| `--json` | JSON output (for parsing) |
| `--quiet` | Suppress all non-JSON, non-error output |

### Secure Key Storage

Always use `--nsec-cmd` to store the nsec. The command receives the nsec on stdin (one line + EOF):

```bash
# GNU pass
--nsec-cmd "pass insert -e nostr/myagent"

# age encryption
--nsec-cmd "age -r age1... -o ~/.nostr/nsec.age"

# Linux keyring
--nsec-cmd "secret-tool store --label='nostr' service nostr account default"

# File (least secure)
--nsec-cmd "tee ~/.nostr/nsec > /dev/null && chmod 600 ~/.nostr/nsec"
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

Typical agent flow:

```bash
# Create identity and store key
nihao --name "mybot" --json --quiet --nsec-cmd "pass insert -e nostr/mybot" > /tmp/nihao-result.json

# Later: verify identity health
nihao check npub1... --json --quiet
```

Parse JSON output, check exit codes, act on results.
