package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/nip19"
)

var version = "dev"

// Default relays for new identities — curated for reliability and coverage.
// General-purpose relays (read + write):
//   damus, primal, nos.lol — large, long-running, well-connected
// Specialized relays (important for discoverability):
//   purplepag.es — NIP-65 relay list aggregator, critical for outbox model
//
// Future: discover relays dynamically via NIP-66 relay monitors or by
// sampling kind 10002 lists from well-connected npubs.
var defaultRelays = []string{
	"wss://relay.damus.io",
	"wss://relay.primal.net",
	"wss://nos.lol",
	"wss://purplepag.es",
}

func main() {
	args := os.Args[1:]

	if len(args) > 0 {
		switch args[0] {
		case "check":
			target := ""
			jsonOutput := false
			for _, a := range args[1:] {
				if a == "--json" {
					jsonOutput = true
				} else if !strings.HasPrefix(a, "-") {
					target = a
				}
			}
			runCheck(target, jsonOutput)
			return
		case "version", "--version":
			fmt.Printf("nihao %s\n", version)
			return
		case "help", "--help", "-h":
			printUsage()
			return
		}
	}

	runSetup(args)
}

func printUsage() {
	fmt.Println(`nihao 👋 — nostr identity health-check automation & optimization

USAGE:
  nihao                     Set up a new Nostr identity with sane defaults
  nihao check [npub]        Check the health of a Nostr identity
  nihao version             Print version

SETUP FLAGS:
  --name <name>             Display name
  --about <text>            About/bio text
  --picture <url>           Profile picture URL
  --banner <url>            Banner image URL
  --nip05 <user@domain>     NIP-05 identifier
  --lud16 <user@domain>     Lightning address
  --relays <r1,r2,...>      Comma-separated relay URLs
  --json                    Output result as JSON
  --sec <nsec|hex>          Use existing secret key instead of generating
  --stdin                   Read secret key from stdin (for piping)

CHECK FLAGS:
  --json                    Output result as JSON

EXIT CODES:
  0                         Success (check: all checks pass)
  1                         Failure (check: one or more checks fail)`)
}

func runSetup(args []string) {
	opts := parseSetupFlags(args)

	fmt.Println("nihao 👋")
	fmt.Println()

	// Step 1: Generate or load keypair
	var sk nostr.SecretKey
	if opts.sec != "" {
		var err error
		sk, err = parseSecretKey(opts.sec)
		if err != nil {
			fatal("invalid secret key: %s", err)
		}
		fmt.Println("🔑 Using provided secret key")
	} else if opts.stdin {
		line := readStdin()
		var err error
		sk, err = parseSecretKey(strings.TrimSpace(line))
		if err != nil {
			fatal("invalid secret key from stdin: %s", err)
		}
		fmt.Println("🔑 Using secret key from stdin")
	} else {
		sk = generateKey()
		fmt.Println("🔑 Generated new keypair")
	}

	pk := sk.Public()
	nsec := nip19.EncodeNsec(sk)
	npub := nip19.EncodeNpub(pk)

	fmt.Printf("   npub: %s\n", npub)
	fmt.Println()

	// Step 2: Build and publish profile metadata (kind 0)
	name := opts.name
	if name == "" {
		name = "nihao-user"
	}

	profile := ProfileMetadata{
		Name:        name,
		DisplayName: name,
	}
	if opts.about != "" {
		profile.About = opts.about
	}
	if opts.picture != "" {
		profile.Picture = opts.picture
	}
	if opts.banner != "" {
		profile.Banner = opts.banner
	}
	if opts.nip05 != "" {
		profile.NIP05 = opts.nip05
	}
	if opts.lud16 != "" {
		profile.LUD16 = opts.lud16
	} else {
		// Default: npub.cash lightning address (works without registration)
		profile.LUD16 = npub + "@npub.cash"
	}

	contentBytes, _ := json.Marshal(profile)

	evt := nostr.Event{
		CreatedAt: nostr.Timestamp(time.Now().Unix()),
		Kind:      0,
		Tags:      nostr.Tags{},
		Content:   string(contentBytes),
	}
	evt.Sign(sk)

	relays := defaultRelays
	if opts.relays != nil {
		relays = opts.relays
	}

	fmt.Println("👤 Publishing profile metadata (kind 0)...")
	publishToRelays(evt, relays)
	fmt.Println()

	// Step 3: Publish relay list (kind 10002)
	var relayTags nostr.Tags
	for _, r := range relays {
		relayTags = append(relayTags, nostr.Tag{"r", r})
	}

	relayEvt := nostr.Event{
		CreatedAt: nostr.Timestamp(time.Now().Unix()),
		Kind:      10002,
		Tags:      relayTags,
		Content:   "",
	}
	relayEvt.Sign(sk)

	fmt.Println("📡 Publishing relay list (kind 10002)...")
	publishToRelays(relayEvt, relays)
	fmt.Println()

	// Step 4: Publish empty follow list (kind 3)
	followEvt := nostr.Event{
		CreatedAt: nostr.Timestamp(time.Now().Unix()),
		Kind:      3,
		Tags:      nostr.Tags{},
		Content:   "",
	}
	followEvt.Sign(sk)

	fmt.Println("👥 Publishing follow list (kind 3)...")
	publishToRelays(followEvt, relays)
	fmt.Println()

	// Step 5: Say hello (kind 1)
	helloEvt := nostr.Event{
		CreatedAt: nostr.Timestamp(time.Now().Unix()),
		Kind:      1,
		Tags:      nostr.Tags{},
		Content:   "nihao 👋 world",
	}
	helloEvt.Sign(sk)

	fmt.Println("💬 Posting first note (kind 1)...")
	publishToRelays(helloEvt, relays)
	fmt.Println()

	// Summary
	fmt.Println("✅ Identity created!")
	fmt.Println()

	if opts.jsonOutput {
		result := SetupResult{
			Npub:    npub,
			Nsec:    nsec,
			Pubkey:  pk.Hex(),
			Relays:  relays,
			Profile: profile,
		}
		out, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(out))
	} else {
		fmt.Println("   ┌─────────────────────────────────────────")
		fmt.Printf("   │ npub: %s\n", npub)
		fmt.Printf("   │ nsec: %s\n", nsec)
		fmt.Println("   │")
		fmt.Printf("   │ name: %s\n", name)
		fmt.Printf("   │ relays: %d configured\n", len(relays))
		fmt.Println("   └─────────────────────────────────────────")
		fmt.Println()
		fmt.Println("   ⚠️  Save your nsec! It cannot be recovered.")
	}
}

type publishResult struct {
	url     string
	success bool
	err     string
}

func publishToRelays(evt nostr.Event, relays []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	results := make(chan publishResult, len(relays))
	var wg sync.WaitGroup

	for _, url := range relays {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			relayCtx, relayCancel := context.WithTimeout(ctx, 5*time.Second)
			defer relayCancel()

			relay, err := nostr.RelayConnect(relayCtx, url, nostr.RelayOptions{})
			if err != nil {
				results <- publishResult{url, false, "connection failed"}
				return
			}
			defer relay.Close()

			err = relay.Publish(relayCtx, evt)
			if err != nil {
				results <- publishResult{url, false, err.Error()}
			} else {
				results <- publishResult{url, true, ""}
			}
		}(url)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for r := range results {
		if r.success {
			fmt.Printf("   ✓ %s\n", r.url)
		} else {
			fmt.Printf("   ✗ %s (%s)\n", r.url, r.err)
		}
	}
}

func parseSecretKey(input string) (nostr.SecretKey, error) {
	if strings.HasPrefix(input, "nsec1") {
		prefix, val, err := nip19.Decode(input)
		if err != nil {
			return nostr.SecretKey{}, err
		}
		if prefix != "nsec" {
			return nostr.SecretKey{}, fmt.Errorf("expected nsec, got %s", prefix)
		}
		return val.(nostr.SecretKey), nil
	}
	return nostr.SecretKeyFromHex(input)
}

// ProfileMetadata represents kind 0 content
type ProfileMetadata struct {
	Name        string `json:"name,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	About       string `json:"about,omitempty"`
	Picture     string `json:"picture,omitempty"`
	Banner      string `json:"banner,omitempty"`
	NIP05       string `json:"nip05,omitempty"`
	LUD16       string `json:"lud16,omitempty"`
	Website     string `json:"website,omitempty"`
}

type SetupResult struct {
	Npub    string          `json:"npub"`
	Nsec    string          `json:"nsec"`
	Pubkey  string          `json:"pubkey"`
	Relays  []string        `json:"relays"`
	Profile ProfileMetadata `json:"profile"`
}

type setupOpts struct {
	name       string
	about      string
	picture    string
	banner     string
	nip05      string
	lud16      string
	relays     []string
	sec        string
	stdin      bool
	jsonOutput bool
}

func parseSetupFlags(args []string) setupOpts {
	opts := setupOpts{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--name":
			if i+1 < len(args) {
				opts.name = args[i+1]
				i++
			}
		case "--about":
			if i+1 < len(args) {
				opts.about = args[i+1]
				i++
			}
		case "--picture":
			if i+1 < len(args) {
				opts.picture = args[i+1]
				i++
			}
		case "--banner":
			if i+1 < len(args) {
				opts.banner = args[i+1]
				i++
			}
		case "--nip05":
			if i+1 < len(args) {
				opts.nip05 = args[i+1]
				i++
			}
		case "--lud16":
			if i+1 < len(args) {
				opts.lud16 = args[i+1]
				i++
			}
		case "--relays":
			if i+1 < len(args) {
				opts.relays = strings.Split(args[i+1], ",")
				i++
			}
		case "--sec":
			if i+1 < len(args) {
				opts.sec = args[i+1]
				i++
			}
		case "--json":
			opts.jsonOutput = true
		case "--stdin":
			opts.stdin = true
		}
	}
	return opts
}

func generateKey() nostr.SecretKey {
	var sk nostr.SecretKey
	if _, err := rand.Read(sk[:]); err != nil {
		fatal("failed to generate random key: %s", err)
	}
	return sk
}

func readStdin() string {
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return scanner.Text()
	}
	return ""
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
