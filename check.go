package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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

		// Check 1: Profile exists
		result.addCheck("profile", "pass", fmt.Sprintf("name=%q", meta.Name))
		result.Score++

		// Check 2: NIP-05
		if meta.NIP05 != "" {
			if verifyNIP05(ctx, meta.NIP05, pk) {
				result.addCheck("nip05", "pass", meta.NIP05)
				result.Score++
			} else {
				result.addCheck("nip05", "warn", fmt.Sprintf("%s (set but doesn't resolve)", meta.NIP05))
			}
		} else {
			result.addCheck("nip05", "fail", "not set")
		}

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
