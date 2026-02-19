# nihao TODO

## Bugs / DX
- [x] Unknown flags silently ignored — `--nsec` is a natural guess for `--sec`, should error or warn on unknown flags
- [ ] Relay markers missing in kind 10002 — `check` flags "no read/write markers" even though setup publishes them; verify markers are set correctly

## Setup
- [ ] NIP-05 setup assistance — help users configure `/.well-known/nostr.json` on their domain
- [x] `--sec` → `--nsec` alias — add `--nsec` as an alias since it's the more intuitive flag name
- [ ] Profile picture via Blossom — upload a picture to a Blossom server during setup and set it in kind 0
- [ ] Follow yourself — optionally add the created npub to its own follow list (some clients show this)
- [ ] `nihao repair` — re-publish events that failed (e.g. due to rate-limiting on some relays) without regenerating anything

## Check
- [ ] Dynamic relay discovery via NIP-66 relay monitors — use relay monitor events to find/recommend relays instead of hardcoded lists
- [ ] `nihao check --fix` — auto-fix issues found during check (re-publish missing events, add markers, etc.)
- [ ] Check for stale relay list — warn if kind 10002 relays are unreachable
- [ ] Check event timestamps — warn if events are very old and might benefit from republishing

## Ideas
- [ ] `nihao update` — update an existing identity (change name, picture, etc.)
- [x] `nihao backup` — export full identity state (profile, relay list, follows, wallet)
- [ ] `nihao nuke` — publish deletion events for all identity-related kinds
- [ ] `nihao restore` — import identity from backup JSON (counterpart to `nihao backup`)
- [ ] `nihao follow <npub>` — add someone to follow list
- [ ] `nihao whoami` — show current identity info (npub, profile, relay list) from a stored/cached nsec
- [ ] Agent mode (`--agent`) — non-interactive, structured JSON errors, exit codes for every failure type, designed for scripting and AI agents
- [ ] Retry with backoff — when a relay rate-limits, wait and retry instead of giving up
