# Vision

**nihao is the simplest way to properly set up a nostr identity — for agents and humans alike.**

## The Problem

Getting started on nostr is harder than it should be. Generate a keypair, pick relays, publish metadata, set up a lightning address, configure a wallet, find a blossom server, set NIP-05 verification — each step has its own tools, its own quirks, its own footguns. Most people skip half of it. Most agents skip all of it.

The result: broken profiles, missing relay lists, no way to receive payments, no way to host media. A half-formed identity in a network that rewards completeness.

## The Goal

One command. Full identity. No accounts, no API keys, no KYC.

```
nihao --name "Alice"
```

That's it. After this runs, you have:

- **A keypair** — your sovereign identity
- **A published profile** — name, about, picture, the works
- **A relay list** — properly configured with NIP-65 read/write markers
- **A DM relay list** — so people can actually reach you (NIP-17)
- **A follow list** — seeded or empty, your choice
- **A lightning address** — ready to receive sats
- **A Cashu wallet** — NIP-60, with validated mints and nutzap support
- **Blossom media hosting** — a place to put your profile picture and media
- **NIP-05 verification** — human-readable identity, if you have a domain

Everything a nostr client expects. Everything another user or agent needs to interact with you. Zero configuration beyond a name.

## Principles

**Sane defaults, full control.** nihao should work perfectly with zero flags. But every default should be overridable for power users who know what they want.

**Verify, don't trust.** When nihao sets something up, it checks that it actually worked. Mints are validated. Relays are tested. Images are reachable. If something fails, you know about it.

**Agents are first-class.** `--json` and `--quiet` aren't afterthoughts. nihao is designed to be called by scripts, AI agents, and automation pipelines just as easily as by humans at a terminal.

**Fix what's broken.** `nihao check` doesn't just report problems — it should be able to fix them. Missing relay list? Re-publish it. Dead profile picture? Flag it. Unreachable mint? Suggest alternatives.

**One binary, zero dependencies.** No runtime, no daemon, no config files. Download and run. Works on every platform Go compiles to.

## Non-Goals

- nihao is not a client. It sets you up, then gets out of the way.
- nihao is not a key manager. Use `pass`, `age`, or `--nsec-cmd` for that.
- nihao is not a relay. It publishes to relays, it doesn't run them.
