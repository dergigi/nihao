# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [0.4.0] - 2025-02-16

### Added
- Image hosting tier scoring: blossom (best) > own domain (root NIP-05) > third-party (warn) > broken (fail)
- Root NIP-05 detection (`_@domain` or bare domain) shown as `(root)` in check output
- Picture and banner each count toward health score (max score now 8)

### Fixed
- Bare domain NIP-05 (e.g. `dergigi.com`) now resolves correctly as `_@domain`

## [0.3.0] - 2025-02-16

### Added
- First note (kind 1) posted automatically after identity setup
- `#nihao` hashtag tag on first note for discoverability
- 26 randomized multilingual greetings, each fully in its native language

## [0.2.0] - 2025-02-16

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

## [0.1.0] - 2025-02-16

### Added
- Initial release
- `nihao` — full identity setup (keypair, kind 0, kind 10002, kind 3)
- `nihao check <npub>` — identity health audit with score (0–6)
- NIP-05 verification (live HTTP check)
- Lightning address verification (LNURL resolution)
- NIP-60 wallet detection (kind 37375)
- `--json` output for agent consumption
- `--sec` flag to use existing secret key
