# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [0.9.0] - 2026-02-18

### Added
- **NIP-65 read/write markers**: Kind 10002 relay list now uses proper `read`/`write` markers per NIP-65
  - General relays (damus, primal, nos.lol) marked as both read+write
  - purplepag.es excluded from kind 10002 (still used for publishing outbox events)
  - `--discover` classifies relays automatically
- **Kind 10050 DM relay list**: Publishes DM inbox relays per NIP-17
  - Default DM relays: inbox.nostr.wine, auth.nostr1.com
  - `--dm-relays <r1,r2,...>` flag to override defaults
  - `--no-dm-relays` flag to skip DM relay publishing
  - `--discover` samples kind 10050 from well-connected npubs
- **`nihao check` relay marker analysis**: Warns if all relays have bare tags (no read/write markers)
- **`nihao check` DM relay detection**: Checks for kind 10050, warns if missing
- **Relay purpose display**: Per-relay detail output now shows read/write/read+write purpose

## [0.9.0] - 2026-02-18

### Added
- NIP-65 relay markers: kind 10002 now uses proper read/write tags
- Kind 10050 DM inbox relay list: published during setup per NIP-17
- `--dm-relays` and `--no-dm-relays` flags
- DM relay discovery from well-connected npubs
- `nihao check` audits relay markers (warns if all bare) and DM relays (warns if missing)
- Per-relay purpose display in check output (read, write, read+write)

### Changed
- **Connection pooling:** setup connects to relays once and reuses for all publishes (was 7+ separate connections)
- **Rate limiting:** 300ms delay between publishes to avoid damus rate-limiting on fresh npubs
- **Relay config cleanup:** consolidated all hardcoded relay maps into clear config blocks at top of relay.go
- **Scoring fairness:** removed reliability bonus — scores based purely on observed metrics (latency, NIP-11, reachability)
- `wellConnectedNpubs` extracted as shared constant (was duplicated)
- `classifyRelay()` simplified to single lookup path

## [0.8.0] - 2026-02-18

### Added
- **Relay discovery:** `--discover` flag samples kind 10002 lists from well-connected npubs (fiatjaf, jb55, NVK, odell, jack) and scores relays by latency, NIP-11 support, and reachability
- **Relay quality analysis in check:** `nihao check` now tests each relay in the npub's kind 10002 — latency, NIP-11, reachability with per-relay detail output
- **Kind-aware publishing:** specialized relays (e.g. purplepag.es) only receive compatible event kinds — no more kind 1 rejections
- **Relay scoring engine:** `relay.go` with NIP-11 fetching, WebSocket latency measurement, and 0.0–1.0 scoring
- **Smart relay selection:** `SelectRelays` picks optimal set guaranteeing at least one outbox relay
- **Relay classification:** detects paid (pyramid, premium), inbox, NWC, and search relays by URL patterns
- Known paid/restricted relays filtered from selection automatically

### Fixed
- purplepag.es no longer receives kind 1 events (was causing rejections)
- Paid relays (premium.primal.net, pyramid) no longer selected during discovery
- Inbox relays (inbox.relays.land) and NWC endpoints filtered from selection

## [0.7.0] - 2026-02-18

### Changed
- Complete README rewrite: leads with the "why" — self-sovereign identity and unstoppable money for agents
- New sections: Why, Quick Start, The Stack, Philosophy
- Updated SKILL.md description to match
- Marked OpenClaw skill wrapper as complete

## [0.6.0] - 2026-02-17

### Added
- NIP-60 Cashu wallet setup during identity creation (kind 17375 + kind 10019)
- Mint validation: checks reachability, sat keyset, NUT-04/05/11 support
- `--mint <url>` flag to override default mints (repeatable)
- `--no-wallet` flag to skip wallet setup
- `nihao check` now validates wallet mints (reachability, name display)
- `nihao check` warns if wallet exists but nutzap info (kind 10019) is missing
- `nihao check` supports both kind 17375 (new) and kind 37375 (old) wallet events
- Wallet mint details in `--json` output (URLs, names, reachability, NUT support)
- Curated default mints: minibits, coinos, macadamia

## [0.5.0] - 2026-02-16

### Added
- `--quiet` / `-q` flag for agent-friendly silent mode (setup and check)
- Parallel relay fetching in `nihao check` for faster health checks

### Fixed
- README health score range corrected from 0–6 to 0–8

## [0.4.0] - 2026-02-16

### Added
- Image hosting tier scoring: blossom (best) > own domain (root NIP-05) > third-party (warn) > broken (fail)
- Root NIP-05 detection (`_@domain` or bare domain) shown as `(root)` in check output
- Picture and banner each count toward health score (max score now 8)

### Fixed
- Bare domain NIP-05 (e.g. `dergigi.com`) now resolves correctly as `_@domain`

## [0.3.0] - 2026-02-16

### Added
- First note (kind 1) posted automatically after identity setup
- `#nihao` hashtag tag on first note for discoverability
- 26 randomized multilingual greetings, each fully in its native language

## [0.2.0] - 2026-02-16

### Added
- Profile image health checks (404 detection, file size, Blossom hosting)
- Auto-set lud16 to `<npub>@npub.cash` (works without registration)
- Stdin support for piping secret keys (`echo $NSEC | nihao --stdin`)
- Profile completeness breakdown (name, display_name, about, picture, banner)
- Parallel relay publishing with per-relay timeouts
- `--json` output on `nihao check`
- Meaningful exit codes (0 = healthy, 1 = issues found)

### Changed
- Curated default relay list: damus, primal, nos.lol, purplepag.es

### Removed
- relay.nostr.band from defaults (unmaintained)
- relay.snort.social from defaults (unreliable)

## [0.1.0] - 2026-02-16

### Added
- Initial release
- `nihao` — full identity setup (keypair, kind 0, kind 10002, kind 3)
- `nihao check <npub>` — identity health audit with score (0–6)
- NIP-05 verification (live HTTP check)
- Lightning address verification (LNURL resolution)
- NIP-60 wallet detection (kind 37375)
- `--json` output for agent consumption
- `--sec` flag to use existing secret key
