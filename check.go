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
	Npub     string      `json:"npub"`
	Pubkey   string      `json:"pubkey"`
	Score    int         `json:"score"`
	MaxScore int         `json:"max_score"`
	Checks   []CheckItem `json:"checks"`
}

type CheckItem struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "pass", "fail", "warn"
	Detail string `json:"detail,omitempty"`
}

func runCheck(target string, jsonOutput bool) {
	if target == "" {
		fatal("usage: nihao check <npub|hex>")
	}

	pk, err := parsePubkey(target)
	if err != nil {
		fatal("invalid pubkey: %s", err)
	}

	npub := nip19.EncodeNpub(pk)
	if !jsonOutput {
		fmt.Printf("nihao check 🔍 %s\n\n", npub)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result := CheckResult{
		Npub:     npub,
		Pubkey:   pk.Hex(),
		MaxScore: 6,
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
		checkProfileImages(ctx, &result, meta.Picture, meta.Banner)

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

	// Check 6: NIP-60 wallet (kind 37375)
	_, walletEvt := fetchKind(ctx, pk, 37375)
	if walletEvt != nil {
		result.addCheck("nip60_wallet", "pass", "wallet event found")
		result.Score++
	} else {
		result.addCheck("nip60_wallet", "fail", "no NIP-60 wallet found")
	}

	if jsonOutput {
		out, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(out))
		if result.Score < result.MaxScore {
			os.Exit(1)
		}
	} else {
		printCheckResult(result)
		if result.Score < result.MaxScore {
			os.Exit(1)
		}
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

	for _, url := range defaultRelays {
		relayCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		relay, err := nostr.RelayConnect(relayCtx, url, nostr.RelayOptions{})
		if err != nil {
			cancel()
			continue
		}
		for evt := range relay.QueryEvents(filter) {
			relay.Close()
			cancel()
			return url, &evt
		}
		relay.Close()
		cancel()
	}
	return "", nil
}

func verifyNIP05(ctx context.Context, identifier string, expectedPK nostr.PubKey) bool {
	parts := strings.Split(identifier, "@")
	if len(parts) != 2 {
		return false
	}
	name, domain := parts[0], parts[1]

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

func checkProfileImages(ctx context.Context, result *CheckResult, picture, banner string) {
	images := []struct {
		name string
		url  string
	}{
		{"picture", picture},
		{"banner", banner},
	}

	for _, img := range images {
		if img.url == "" {
			continue // already flagged as missing in profile completeness
		}

		info := probeImage(ctx, img.url)

		var parts []string

		// Reachability
		if info.Status == -1 {
			result.addCheck(img.name, "warn", fmt.Sprintf("unreachable: %s", img.url))
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

		// Hosting
		if info.Blossom {
			parts = append(parts, "blossom ✓")
		} else {
			parts = append(parts, "not on blossom")
		}

		// Size
		if info.Size >= 0 {
			sizeStr := formatSize(info.Size)
			if info.SizeWarn {
				parts = append(parts, fmt.Sprintf("%s ⚠️ large", sizeStr))
			} else {
				parts = append(parts, sizeStr)
			}
		}

		status := "pass"
		if info.SizeWarn {
			status = "warn"
		}

		result.addCheck(img.name, status, strings.Join(parts, ", "))
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
