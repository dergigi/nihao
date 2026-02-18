package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"


	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/nip19"
)

type CheckResult struct {
	Npub     string          `json:"npub"`
	Pubkey   string          `json:"pubkey"`
	Score    int             `json:"score"`
	MaxScore int             `json:"max_score"`
	Checks   []CheckItem     `json:"checks"`
	Wallet   *WalletCheckInfo `json:"wallet,omitempty"`
}

// WalletCheckInfo holds wallet details discovered during check.
type WalletCheckInfo struct {
	WalletKind int         `json:"wallet_kind"`
	HasNutzap  bool        `json:"has_nutzap_info"`
	Mints      []MintInfo  `json:"mints,omitempty"`
	P2PKPubkey string      `json:"p2pk_pubkey,omitempty"`
}

type CheckItem struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "pass", "fail", "warn"
	Detail string `json:"detail,omitempty"`
}

func runCheck(target string, jsonOutput bool, quiet bool) {
	if target == "" {
		target = os.Getenv("NOSTR_NPUB")
	}
	if target == "" {
		fatal("usage: nihao check <npub|hex>\n\nTip: set NOSTR_NPUB to skip the argument")
	}

	pk, err := parsePubkey(target)
	if err != nil {
		fatal("invalid pubkey: %s", err)
	}

	npub := nip19.EncodeNpub(pk)
	if !jsonOutput && !quiet {
		fmt.Printf("nihao check 🔍 %s\n\n", npub)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result := CheckResult{
		Npub:     npub,
		Pubkey:   pk.Hex(),
		MaxScore: 8,
	}

	// Fetch profile (kind 0)
	_, profileEvt := fetchKind(ctx, pk, 0)
	if profileEvt != nil {
		var meta ProfileMetadata
		json.Unmarshal([]byte(profileEvt.Content), &meta)

		// Check 1: Profile exists with completeness
		fields := []string{}
		missing := []string{}
		for _, f := range []struct{ name, val string }{
			{"name", meta.Name},
			{"display_name", meta.DisplayName},
			{"about", meta.About},
			{"picture", meta.Picture},
			{"banner", meta.Banner},
		} {
			if f.val != "" {
				fields = append(fields, f.name)
			} else {
				missing = append(missing, f.name)
			}
		}

		detail := fmt.Sprintf("name=%q, %d/5 fields", meta.Name, len(fields))
		if len(missing) > 0 {
			detail += fmt.Sprintf(" (missing: %s)", strings.Join(missing, ", "))
		}

		if len(fields) >= 3 {
			result.addCheck("profile", "pass", detail)
			result.Score++
		} else if len(fields) >= 1 {
			result.addCheck("profile", "warn", detail)
			result.Score++ // still counts, just not complete
		} else {
			result.addCheck("profile", "fail", "empty profile")
		}

		// Check 2: NIP-05
		if meta.NIP05 != "" {
			if verifyNIP05(ctx, meta.NIP05, pk) {
				// Check for root NIP-05 (_@domain)
				nip05Display := meta.NIP05
				isRoot := isRootNIP05(meta.NIP05)
				if isRoot {
					nip05Display += " (root)"
				}
				result.addCheck("nip05", "pass", nip05Display)
				result.Score++
			} else {
				result.addCheck("nip05", "warn", fmt.Sprintf("%s (set but doesn't resolve)", meta.NIP05))
			}
		} else {
			result.addCheck("nip05", "fail", "not set")
		}

		// Check: Profile images health
		// Extract NIP-05 domain for own-domain hosting detection
		nip05Domain := ""
		if meta.NIP05 != "" {
			if strings.Contains(meta.NIP05, "@") {
				parts := strings.SplitN(meta.NIP05, "@", 2)
				if parts[0] == "_" {
					nip05Domain = parts[1]
				}
			} else {
				nip05Domain = meta.NIP05 // bare domain = root
			}
		}
		checkProfileImages(ctx, &result, meta.Picture, meta.Banner, nip05Domain)

		// Check 3: Lightning address
		if meta.LUD16 != "" {
			if verifyLUD16(ctx, meta.LUD16) {
				result.addCheck("lud16", "pass", meta.LUD16)
				result.Score++
			} else {
				result.addCheck("lud16", "warn", fmt.Sprintf("%s (set but doesn't resolve)", meta.LUD16))
			}
		} else {
			result.addCheck("lud16", "fail", "not set")
		}
	} else {
		result.addCheck("profile", "fail", "no kind 0 found")
		result.addCheck("nip05", "fail", "no profile")
		result.addCheck("lud16", "fail", "no profile")
	}

	// Check 4: Relay list (kind 10002)
	_, relayEvt := fetchKind(ctx, pk, 10002)
	if relayEvt != nil {
		relayCount := 0
		for _, tag := range relayEvt.Tags {
			if len(tag) >= 2 && tag[0] == "r" {
				relayCount++
			}
		}
		if relayCount >= 2 {
			result.addCheck("relay_list", "pass", fmt.Sprintf("%d relays", relayCount))
			result.Score++
		} else {
			result.addCheck("relay_list", "warn", fmt.Sprintf("only %d relay(s)", relayCount))
		}
	} else {
		result.addCheck("relay_list", "fail", "no kind 10002 found")
	}

	// Check 5: Follow list (kind 3)
	_, followEvt := fetchKind(ctx, pk, 3)
	if followEvt != nil {
		followCount := 0
		for _, tag := range followEvt.Tags {
			if len(tag) >= 2 && tag[0] == "p" {
				followCount++
			}
		}
		if followCount > 0 {
			result.addCheck("follow_list", "pass", fmt.Sprintf("%d follows", followCount))
			result.Score++
		} else {
			result.addCheck("follow_list", "warn", "empty follow list")
		}
	} else {
		result.addCheck("follow_list", "fail", "no kind 3 found")
	}

	// Check 6: NIP-60 wallet (kind 17375 new, 37375 old)
	walletKind := 0
	_, walletEvt := fetchKind(ctx, pk, 17375)
	if walletEvt != nil {
		walletKind = 17375
	} else {
		_, walletEvt = fetchKind(ctx, pk, 37375) // backwards compat
		if walletEvt != nil {
			walletKind = 37375
		}
	}
	if walletEvt != nil {
		kindLabel := fmt.Sprintf("kind %d", walletKind)
		if walletKind == 37375 {
			kindLabel += " (old)"
		}
		result.addCheck("nip60_wallet", "pass", fmt.Sprintf("wallet event found (%s)", kindLabel))
		result.Score++

		// Check for nutzap info (kind 10019)
		walletInfo := &WalletCheckInfo{WalletKind: walletKind}
		_, nutzapEvt := fetchKind(ctx, pk, 10019)
		if nutzapEvt != nil {
			walletInfo.HasNutzap = true

			// Extract mints and P2PK pubkey from kind 10019
			var mintURLs []string
			for _, tag := range nutzapEvt.Tags {
				if len(tag) >= 2 && tag[0] == "mint" {
					mintURLs = append(mintURLs, tag[1])
				}
				if len(tag) >= 2 && tag[0] == "pubkey" {
					walletInfo.P2PKPubkey = tag[1]
				}
			}

			if len(mintURLs) > 0 {
				// Validate mints (don't fail check, just report status)
				for _, mintURL := range mintURLs {
					mintInfo := validateMint(ctx, mintURL)
					walletInfo.Mints = append(walletInfo.Mints, mintInfo)
				}

				// Report mint status
				reachable := 0
				for _, m := range walletInfo.Mints {
					if m.Reachable {
						reachable++
					}
				}

				mintDetail := fmt.Sprintf("%d mint(s), %d reachable", len(mintURLs), reachable)
				if reachable == len(mintURLs) {
					result.addCheck("wallet_mints", "pass", mintDetail)
				} else if reachable > 0 {
					result.addCheck("wallet_mints", "warn", mintDetail)
				} else {
					result.addCheck("wallet_mints", "warn", mintDetail+" — all mints unreachable")
				}
			}

			result.addCheck("nutzap_info", "pass", "kind 10019 found")
		} else {
			walletInfo.HasNutzap = false
			result.addCheck("nutzap_info", "warn", "wallet exists but no kind 10019 (nutzap info) — others can't send you nutzaps")
		}

		result.Wallet = walletInfo
	} else {
		result.addCheck("nip60_wallet", "fail", "no NIP-60 wallet found")
	}

	if jsonOutput {
		out, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(out))
	} else if !quiet {
		printCheckResult(result)
	}
	if result.Score < result.MaxScore {
		os.Exit(1)
	}
}

func (r *CheckResult) addCheck(name, status, detail string) {
	r.Checks = append(r.Checks, CheckItem{
		Name:   name,
		Status: status,
		Detail: detail,
	})
}

func fetchKind(ctx context.Context, pk nostr.PubKey, kind int) (string, *nostr.Event) {
	filter := nostr.Filter{
		Authors: []nostr.PubKey{pk},
		Kinds:   []nostr.Kind{nostr.Kind(kind)},
		Limit:   1,
	}

	type fetchResult struct {
		url string
		evt *nostr.Event
	}

	results := make(chan fetchResult, len(defaultRelays))
	fetchCtx, fetchCancel := context.WithCancel(ctx)
	defer fetchCancel()

	for _, u := range defaultRelays {
		go func(u string) {
			relayCtx, cancel := context.WithTimeout(fetchCtx, 5*time.Second)
			defer cancel()
			relay, err := nostr.RelayConnect(relayCtx, u, nostr.RelayOptions{})
			if err != nil {
				results <- fetchResult{u, nil}
				return
			}
			defer relay.Close()
			for evt := range relay.QueryEvents(filter) {
				results <- fetchResult{u, &evt}
				return
			}
			results <- fetchResult{u, nil}
		}(u)
	}

	for range defaultRelays {
		r := <-results
		if r.evt != nil {
			return r.url, r.evt
		}
	}
	return "", nil
}

func verifyNIP05(ctx context.Context, identifier string, expectedPK nostr.PubKey) bool {
	var name, domain string
	if strings.Contains(identifier, "@") {
		parts := strings.SplitN(identifier, "@", 2)
		name, domain = parts[0], parts[1]
	} else {
		// Bare domain (e.g. "dergigi.com") is treated as _@domain
		name, domain = "_", identifier
	}

	url := fmt.Sprintf("https://%s/.well-known/nostr.json?name=%s", domain, name)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		return false
	}
	defer resp.Body.Close()

	var result struct {
		Names map[string]string `json:"names"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}

	pk, ok := result.Names[name]
	if !ok {
		return false
	}

	return pk == expectedPK.Hex()
}

func verifyLUD16(ctx context.Context, lud16 string) bool {
	parts := strings.Split(lud16, "@")
	if len(parts) != 2 {
		return false
	}
	name, domain := parts[0], parts[1]

	url := fmt.Sprintf("https://%s/.well-known/lnurlp/%s", domain, name)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		return false
	}
	defer resp.Body.Close()

	var result struct {
		Callback string `json:"callback"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}

	return result.Callback != ""
}

func parsePubkey(input string) (nostr.PubKey, error) {
	if strings.HasPrefix(input, "npub1") {
		prefix, val, err := nip19.Decode(input)
		if err != nil {
			return nostr.PubKey{}, err
		}
		if prefix != "npub" {
			return nostr.PubKey{}, fmt.Errorf("expected npub, got %s", prefix)
		}
		return val.(nostr.PubKey), nil
	}
	return nostr.PubKeyFromHex(input)
}

// imageInfo holds the result of probing a profile image URL.
type imageInfo struct {
	URL      string `json:"url"`
	Status   int    `json:"status"`
	Size     int64  `json:"size_bytes"` // -1 if unknown
	Blossom  bool   `json:"blossom"`
	SizeWarn bool   `json:"size_warn"` // true if > 1MB
}

// knownBlossomHosts is a set of known Blossom media servers.
var knownBlossomHosts = map[string]bool{
	"blossom.primal.net":  true,
	"cdn.satellite.earth": true,
	"files.v0l.io":        true,
	"blossom.oxtr.dev":    true,
	"blossom.band":        true,
	"media.nostr.build":   true,
}

const maxRecommendedImageSize = 1 << 20 // 1 MB

func probeImage(ctx context.Context, rawURL string) imageInfo {
	info := imageInfo{URL: rawURL, Size: -1}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		info.Status = -1
		return info
	}

	host := strings.ToLower(parsed.Hostname())
	info.Blossom = knownBlossomHosts[host]

	req, err := http.NewRequestWithContext(ctx, "HEAD", rawURL, nil)
	if err != nil {
		info.Status = -1
		return info
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		info.Status = -1
		return info
	}
	resp.Body.Close()

	info.Status = resp.StatusCode
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		if n, err := strconv.ParseInt(cl, 10, 64); err == nil {
			info.Size = n
			info.SizeWarn = n > maxRecommendedImageSize
		}
	}

	return info
}

func formatSize(bytes int64) string {
	if bytes < 0 {
		return "unknown size"
	}
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	if bytes < 1<<20 {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(bytes)/float64(1<<20))
}

// imageHostingTier classifies where an image is hosted.
// blossom > own domain (root NIP-05) > third-party
func imageHostingTier(info imageInfo, nip05Domain string) (tier string, label string) {
	if info.Blossom {
		return "blossom", "blossom"
	}
	if nip05Domain != "" {
		parsed, err := url.Parse(info.URL)
		if err == nil && strings.ToLower(parsed.Hostname()) == strings.ToLower(nip05Domain) {
			return "own", "own domain"
		}
	}
	return "third-party", "third-party"
}

func checkProfileImages(ctx context.Context, result *CheckResult, picture, banner, nip05Domain string) {
	images := []struct {
		name string
		url  string
	}{
		{"picture", picture},
		{"banner", banner},
	}

	for _, img := range images {
		if img.url == "" {
			result.addCheck(img.name, "fail", "not set")
			continue
		}

		info := probeImage(ctx, img.url)

		// Reachability
		if info.Status == -1 {
			result.addCheck(img.name, "fail", fmt.Sprintf("unreachable: %s", img.url))
			continue
		}
		if info.Status == 404 {
			result.addCheck(img.name, "fail", fmt.Sprintf("404 not found: %s", img.url))
			continue
		}
		if info.Status >= 400 {
			result.addCheck(img.name, "warn", fmt.Sprintf("HTTP %d: %s", info.Status, img.url))
			continue
		}

		// Hosting tier
		tier, tierLabel := imageHostingTier(info, nip05Domain)
		var parts []string
		parts = append(parts, tierLabel)

		// Size
		if info.Size >= 0 {
			sizeStr := formatSize(info.Size)
			if info.SizeWarn {
				parts = append(parts, fmt.Sprintf("%s (too large)", sizeStr))
			} else {
				parts = append(parts, sizeStr)
			}
		}

		status := "pass"
		if info.SizeWarn {
			status = "warn"
		} else if tier == "third-party" {
			status = "warn"
		}

		result.addCheck(img.name, status, strings.Join(parts, ", "))

		// Score: blossom or own domain = 1 point, third-party reachable = 0.5 (round down)
		if tier == "blossom" || tier == "own" {
			result.Score++
		}
	}
}

// isRootNIP05 checks if a NIP-05 identifier uses the root _ name,
// meaning the user controls the domain (e.g. _@fiatjaf.com or just fiatjaf.com).
func isRootNIP05(nip05 string) bool {
	if !strings.Contains(nip05, "@") {
		// bare domain like "fiatjaf.com" is treated as _@fiatjaf.com
		return true
	}
	parts := strings.SplitN(nip05, "@", 2)
	return parts[0] == "_"
}

func printCheckResult(r CheckResult) {
	statusIcon := map[string]string{
		"pass": "✅",
		"fail": "❌",
		"warn": "⚠️ ",
	}

	for _, c := range r.Checks {
		icon := statusIcon[c.Status]
		fmt.Printf("  %s %s: %s\n", icon, c.Name, c.Detail)
	}

	// Show wallet mint details if available
	if r.Wallet != nil && len(r.Wallet.Mints) > 0 {
		fmt.Println()
		fmt.Println("  Wallet mints:")
		for _, m := range r.Wallet.Mints {
			if m.Reachable {
				name := m.Name
				if name == "" {
					name = "unnamed"
				}
				fmt.Printf("    ✓ %s (%s)\n", m.URL, name)
			} else {
				fmt.Printf("    ✗ %s (unreachable)\n", m.URL)
			}
		}
	}

	fmt.Println()
	pct := 0
	if r.MaxScore > 0 {
		pct = (r.Score * 100) / r.MaxScore
	}
	fmt.Printf("  Score: %d/%d (%d%%)\n", r.Score, r.MaxScore, pct)

	if r.Score == r.MaxScore {
		fmt.Println("  🎉 Perfect identity!")
	} else if r.Score >= r.MaxScore/2 {
		fmt.Println("  👍 Good, but could be better")
	} else {
		fmt.Println("  👎 Needs work")
	}
}
