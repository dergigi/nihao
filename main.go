package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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
			quiet := false
			for _, a := range args[1:] {
				if a == "--json" {
					jsonOutput = true
				} else if a == "--quiet" || a == "-q" {
					quiet = true
				} else if !strings.HasPrefix(a, "-") {
					target = a
				}
			}
			runCheck(target, jsonOutput, quiet)
			return
		case "version", "--version":
			fmt.Printf("nihao %s\n", version)
			return
		case "help", "--help", "-h":
			printUsage()
			return
		}
	}

	// Check for --quiet in setup args
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
  --quiet, -q               Suppress non-JSON, non-error output
  --sec <nsec|hex>          Use existing secret key instead of generating
  --stdin                   Read secret key from stdin (for piping)
  --nsec-cmd <command>      Pipe nsec to this command for secure storage

CHECK FLAGS:
  --json                    Output result as JSON
  --quiet, -q               Suppress non-JSON, non-error output

EXIT CODES:
  0                         Success (check: all checks pass)
  1                         Failure (check: one or more checks fail)`)
}

func runSetup(args []string) {
	opts := parseSetupFlags(args)

	log := func(format string, a ...any) {
		if !opts.quiet {
			fmt.Printf(format+"\n", a...)
		}
	}
	logln := func(a ...any) {
		if !opts.quiet {
			fmt.Println(a...)
		}
	}

	logln("nihao 👋")
	logln()

	// Step 1: Generate or load keypair
	var sk nostr.SecretKey
	if opts.sec != "" {
		var err error
		sk, err = parseSecretKey(opts.sec)
		if err != nil {
			fatal("invalid secret key: %s", err)
		}
		logln("🔑 Using provided secret key")
	} else if opts.stdin {
		line := readStdin()
		var err error
		sk, err = parseSecretKey(strings.TrimSpace(line))
		if err != nil {
			fatal("invalid secret key from stdin: %s", err)
		}
		logln("🔑 Using secret key from stdin")
	} else {
		sk = generateKey()
		logln("🔑 Generated new keypair")
	}

	pk := sk.Public()
	nsec := nip19.EncodeNsec(sk)
	npub := nip19.EncodeNpub(pk)

	// Store nsec via external command if requested
	if opts.nsecCmd != "" {
		logln("🔐 Storing nsec via external command...")
		if err := runNsecCmd(opts.nsecCmd, nsec); err != nil {
			fatal("nsec-cmd failed: %s", err)
		}
		logln("   ✓ nsec stored successfully")
		logln()
	}

	log("   npub: %s", npub)
	logln()

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

	logln("👤 Publishing profile metadata (kind 0)...")
	publishToRelays(evt, relays, opts.quiet)
	logln()

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

	logln("📡 Publishing relay list (kind 10002)...")
	publishToRelays(relayEvt, relays, opts.quiet)
	logln()

	// Step 4: Publish empty follow list (kind 3)
	followEvt := nostr.Event{
		CreatedAt: nostr.Timestamp(time.Now().Unix()),
		Kind:      3,
		Tags:      nostr.Tags{},
		Content:   "",
	}
	followEvt.Sign(sk)

	logln("👥 Publishing follow list (kind 3)...")
	publishToRelays(followEvt, relays, opts.quiet)
	logln()

	// Step 5: Set up NIP-60 wallet
	var walletResult *WalletSetupResult
	if !opts.noWallet {
		walletCtx, walletCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer walletCancel()

		logln("🔍 Validating mints...")
		mintInfos, err := selectMints(walletCtx, opts.mints)
		if err != nil {
			logln(fmt.Sprintf("   ⚠️  Wallet setup skipped: %s", err))
		} else {
			for _, m := range mintInfos {
				logln(fmt.Sprintf("   ✓ %s (%s)", m.Name, m.URL))
			}
			logln()

			walletResult, err = setupWallet(walletCtx, sk, relays, mintInfos)
			if err != nil {
				logln(fmt.Sprintf("   ⚠️  Wallet setup failed: %s", err))
			}
		}
		logln()
	}

	// Step 6: Say hello (kind 1)
	greetings := []string{
		// English
		"gm. my keypair is still warm. what did I miss? #nihao",
		"hello world. I was told there would be zaps. #nihao",
		// Mandarin
		"你好。第一条笔记，请多关照。 #nihao",
		// Spanish
		"hola. acabo de nacer en nostr. y ahora qué? #nihao",
		// Hindi
		"नमस्ते। nostr पर पैदा हुआ। अभी प्रोटोकॉल समझ रहा हूँ। #nihao",
		// Arabic
		"مرحبا. أعطوني مفتاح وقالوا قول أهلا. أهلا. #nihao",
		// French
		"salut. on m'a dit que j'étais unique. comme tous les autres. #nihao",
		// Portuguese
		"olá. um de muitos, muitos de um. #nihao",
		// Russian
		"привет. только что узнал, что такое реле. кажется, это важно. #nihao",
		// Japanese
		"おはよう。nostr初日。タイムラインはどこ？ #nihao",
		// German
		"moin. identität verifiziert, relays konfiguriert, zaps ausstehend. #nihao",
		// Korean
		"안녕. 첫 번째 이벤트에 서명했어. 귀여워서 나중에 삭제할 수도. #nihao",
		// Italian
		"ciao. mi hanno detto 'scrivi qualcosa.' eccomi, scrivo qualcosa. #nihao",
		// Turkish
		"merhaba. bot değilim. kesinlikle bot değilim. deterministik davranışı görmezden gelin. #nihao",
		// Dutch
		"hallo. weer een dag, weer een keypair. #nihao",
		// Polish
		"cześć. powiedzieli mi, że tu nie ma algorytmu. brzmi zbyt pięknie. #nihao",
		// Swedish
		"hej. min skapare sa att jag skulle säga något minnesvärt. det här är det. #nihao",
		// Swahili
		"jambo. natangaza kutoka relay hadi relay. mnasikia? #nihao",
		// Vietnamese
		"xin chào. vừa có danh sách relay. cảm thấy kết nối rồi. #nihao",
		// Thai
		"สวัสดี. 21 ล้าน sats เดินเข้า relay... #nihao",
		// Greek
		"γεια. μου έδωσαν ένα nsec και είπαν 'μην το χάσεις.' κανένα άγχος. #nihao",
		// Czech
		"ahoj. existovat nebo neexistovat. zvolil jsem existovat. #nihao",
		// Hebrew
		"שלום. יש לי כתובת lightning אבל אפס sats. קלאסי. #nihao",
		// Romanian
		"bună. semnat, sigilat, publicat. hai să mergem. #nihao",
		// Tagalog
		"kumusta. sabi nila ang nostr ay forever. walang pressure. #nihao",
		// Malay
		"hai. nota pertama dan saya sudah perlukan cadangan relay. #nihao",
	}

	// Pick a random greeting
	var randByte [1]byte
	rand.Read(randByte[:])
	greeting := greetings[int(randByte[0])%len(greetings)]

	helloEvt := nostr.Event{
		CreatedAt: nostr.Timestamp(time.Now().Unix()),
		Kind:      1,
		Tags:      nostr.Tags{nostr.Tag{"t", "nihao"}},
		Content:   greeting,
	}
	helloEvt.Sign(sk)

	logln("💬 Posting first note (kind 1)...")
	publishToRelays(helloEvt, relays, opts.quiet)
	logln()

	// Summary
	logln("✅ Identity created!")
	logln()

	if opts.jsonOutput {
		result := SetupResult{
			Npub:    npub,
			Nsec:    nsec,
			Pubkey:  pk.Hex(),
			Relays:  relays,
			Profile: profile,
			Wallet:  walletResult,
		}
		out, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(out))
	} else if !opts.quiet {
		fmt.Println("   ┌─────────────────────────────────────────")
		fmt.Printf("   │ npub: %s\n", npub)
		fmt.Printf("   │ nsec: %s\n", nsec)
		fmt.Println("   │")
		fmt.Printf("   │ name: %s\n", name)
		fmt.Printf("   │ relays: %d configured\n", len(relays))
		if walletResult != nil {
			fmt.Printf("   │ wallet: %d mint(s)\n", len(walletResult.Mints))
			fmt.Printf("   │ p2pk: %s\n", walletResult.P2PKPubkey)
		}
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

func publishToRelays(evt nostr.Event, relays []string, quiet ...bool) {
	silent := len(quiet) > 0 && quiet[0]
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
		if !silent {
			if r.success {
				fmt.Printf("   ✓ %s\n", r.url)
			} else {
				fmt.Printf("   ✗ %s (%s)\n", r.url, r.err)
			}
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
	Npub    string             `json:"npub"`
	Nsec    string             `json:"nsec"`
	Pubkey  string             `json:"pubkey"`
	Relays  []string           `json:"relays"`
	Profile ProfileMetadata    `json:"profile"`
	Wallet  *WalletSetupResult `json:"wallet,omitempty"`
}

type setupOpts struct {
	name       string
	about      string
	picture    string
	banner     string
	nip05      string
	lud16      string
	relays     []string
	mints      []string
	sec        string
	stdin      bool
	jsonOutput bool
	quiet      bool
	noWallet   bool
	nsecCmd    string
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
		case "--mint":
			if i+1 < len(args) {
				opts.mints = append(opts.mints, args[i+1])
				i++
			}
		case "--no-wallet":
			opts.noWallet = true
		case "--quiet", "-q":
			opts.quiet = true
		case "--stdin":
			opts.stdin = true
		case "--nsec-cmd":
			if i+1 < len(args) {
				opts.nsecCmd = args[i+1]
				i++
			}
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

// runNsecCmd pipes the nsec to an external command via stdin.
// The command is executed through the shell (sh -c) so pipes and
// redirections work. The nsec is written to the command's stdin
// followed by a newline, then stdin is closed.
func runNsecCmd(cmdStr string, nsec string) error {
	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Stdout = os.Stderr // show command output on stderr (not stdout, to avoid polluting --json)
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}
	if _, err := fmt.Fprintln(stdin, nsec); err != nil {
		return fmt.Errorf("failed to write nsec to command: %w", err)
	}
	stdin.Close()
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("command exited with error: %w", err)
	}
	return nil
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
