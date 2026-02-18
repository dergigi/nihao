package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"fiatjaf.com/nostr"
)

// NIP-11 relay information document
type RelayInfo struct {
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Pubkey        string   `json:"pubkey"`
	Contact       string   `json:"contact"`
	SupportedNIPs []int    `json:"supported_nips"`
	Software      string   `json:"software"`
	Version       string   `json:"version"`
	Limitation    *RelayLimitation `json:"limitation,omitempty"`
	PaymentRequired bool   `json:"payments_url,omitempty"`
}

type RelayLimitation struct {
	MaxMessageLength int  `json:"max_message_length"`
	MaxSubscriptions int  `json:"max_subscriptions"`
	MaxFilters       int  `json:"max_filters"`
	MaxEventTags     int  `json:"max_event_tags"`
	MaxContentLength int  `json:"max_content_length"`
	AuthRequired     bool `json:"auth_required"`
	PaymentRequired  bool `json:"payment_required"`
}

// RelayScore holds quality metrics for a single relay
type RelayScore struct {
	URL          string      `json:"url"`
	Reachable    bool        `json:"reachable"`
	LatencyMs    int64       `json:"latency_ms"`
	Info         *RelayInfo  `json:"info,omitempty"`
	HasNIP11     bool        `json:"has_nip11"`
	SupportsRead bool        `json:"supports_read"`
	SupportsWrite bool       `json:"supports_write"`
	AuthRequired bool        `json:"auth_required"`
	PaymentRequired bool     `json:"payment_required"`
	Score        float64     `json:"score"`       // 0.0 - 1.0
	Purpose      string      `json:"purpose"`     // "general", "outbox", "inbox", "specialized"
	Issues       []string    `json:"issues,omitempty"`
}

// Known specialized relays that shouldn't receive all event kinds
var specializedRelays = map[string]string{
	"wss://purplepag.es":              "outbox",   // NIP-65 only (kind 10002, kind 0, kind 3)
	"wss://relay.nos.social":          "inbox",    // read-heavy
	"wss://search.nos.today":          "search",   // NIP-50 search relay
	"wss://inbox.relays.land":         "inbox",    // mention-only inbox
}

// Relay URL substrings that indicate inbox/specialized relays (discovered dynamically)
var inboxPatterns = []string{
	"/inbox",        // e.g. pyramid.fiatjaf.com/inbox
	"nwc.",          // NWC endpoints, not general relays
}

// Known paid/auth-required relays that accept connections but reject writes
var paidRelays = map[string]bool{
	"wss://premium.primal.net": true,
	"wss://nostr.wine":         true,
}

// Relay URL patterns that indicate restricted relays
var restrictedPatterns = []string{
	"pyramid.",  // pyramid relays require membership
	"premium.",  // premium relays require payment
}

// Outbox-compatible kinds (safe to send to purplepag.es etc)
var outboxKinds = map[nostr.Kind]bool{
	0:     true, // profile metadata
	3:     true, // follow list
	10002: true, // relay list
}

// ShouldPublishTo checks if a given event kind should be sent to a relay
func ShouldPublishTo(relayURL string, kind nostr.Kind) bool {
	purpose := classifyRelay(relayURL)
	switch purpose {
	case "outbox":
		return outboxKinds[kind]
	case "inbox":
		return false // inbox relays need mentions, skip for setup
	case "search", "nwc", "paid":
		return false
	}
	return true // general relay, send everything
}

// classifyRelay determines a relay's purpose
func classifyRelay(relayURL string) string {
	// Check exact matches first
	if purpose, ok := specializedRelays[relayURL]; ok {
		return purpose
	}
	if paidRelays[relayURL] {
		return "paid"
	}
	// Check URL patterns
	for _, pattern := range inboxPatterns {
		if strings.Contains(relayURL, pattern) {
			if strings.Contains(pattern, "nwc") {
				return "nwc"
			}
			return "inbox"
		}
	}
	for _, pattern := range restrictedPatterns {
		if strings.Contains(relayURL, pattern) {
			return "paid"
		}
	}
	return "general"
}

// fetchNIP11 fetches the NIP-11 relay information document
func fetchNIP11(relayURL string) (*RelayInfo, time.Duration, error) {
	// Convert wss:// to https:// for NIP-11
	httpURL := strings.Replace(relayURL, "wss://", "https://", 1)
	httpURL = strings.Replace(httpURL, "ws://", "http://", 1)

	req, err := http.NewRequest("GET", httpURL, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/nostr+json")

	client := &http.Client{Timeout: 5 * time.Second}
	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)
	if err != nil {
		return nil, latency, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, latency, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB max
	if err != nil {
		return nil, latency, err
	}

	var info RelayInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, latency, err
	}

	return &info, latency, nil
}

// testRelayReadWrite does a quick connect + read test
func testRelayReadWrite(relayURL string) (canConnect bool, latency time.Duration, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	relay, err := nostr.RelayConnect(ctx, relayURL, nostr.RelayOptions{})
	latency = time.Since(start)
	if err != nil {
		return false, latency, err
	}
	defer relay.Close()

	return true, latency, nil
}

// ScoreRelay evaluates a single relay's quality
func ScoreRelay(relayURL string) RelayScore {
	rs := RelayScore{
		URL:     relayURL,
		Purpose: "general",
	}

	// Classify relay purpose
	rs.Purpose = classifyRelay(relayURL)

	// Fetch NIP-11
	info, nip11Latency, err := fetchNIP11(relayURL)
	if err == nil && info != nil {
		rs.HasNIP11 = true
		rs.Info = info
		rs.LatencyMs = nip11Latency.Milliseconds()
		if info.Limitation != nil {
			rs.AuthRequired = info.Limitation.AuthRequired
			rs.PaymentRequired = info.Limitation.PaymentRequired
		}
	}

	// Test WebSocket connectivity
	canConnect, wsLatency, err := testRelayReadWrite(relayURL)
	rs.Reachable = canConnect
	if canConnect {
		// Use WS latency if we didn't get NIP-11 latency
		if rs.LatencyMs == 0 {
			rs.LatencyMs = wsLatency.Milliseconds()
		}
		rs.SupportsRead = true
		rs.SupportsWrite = true // assume until proven otherwise
	}

	// Calculate score (0.0 - 1.0)
	rs.Score = calculateRelayScore(rs)

	return rs
}

func calculateRelayScore(rs RelayScore) float64 {
	if !rs.Reachable {
		rs.Issues = append(rs.Issues, "unreachable")
		return 0.0
	}

	score := 0.5 // base score for being reachable

	// NIP-11 support (+0.15)
	if rs.HasNIP11 {
		score += 0.15
	} else {
		rs.Issues = append(rs.Issues, "no NIP-11")
	}

	// Latency scoring (+0.2 max)
	switch {
	case rs.LatencyMs < 200:
		score += 0.20
	case rs.LatencyMs < 500:
		score += 0.15
	case rs.LatencyMs < 1000:
		score += 0.10
	case rs.LatencyMs < 2000:
		score += 0.05
	default:
		rs.Issues = append(rs.Issues, fmt.Sprintf("slow (%dms)", rs.LatencyMs))
	}

	// Auth/payment penalties
	if rs.AuthRequired {
		score -= 0.1
		rs.Issues = append(rs.Issues, "auth required")
	}
	if rs.PaymentRequired {
		score -= 0.1
		rs.Issues = append(rs.Issues, "payment required")
	}

	// Bonus for known reliable relays (+0.15)
	reliable := map[string]bool{
		"wss://relay.damus.io":   true,
		"wss://relay.primal.net": true,
		"wss://nos.lol":          true,
		"wss://purplepag.es":     true,
	}
	if reliable[rs.URL] {
		score += 0.15
	}

	if score > 1.0 {
		score = 1.0
	}
	if score < 0.0 {
		score = 0.0
	}
	return score
}

// ScoreRelays evaluates multiple relays in parallel
func ScoreRelays(urls []string) []RelayScore {
	scores := make([]RelayScore, len(urls))
	var wg sync.WaitGroup

	for i, url := range urls {
		wg.Add(1)
		go func(i int, url string) {
			defer wg.Done()
			scores[i] = ScoreRelay(url)
		}(i, url)
	}

	wg.Wait()
	return scores
}

// DiscoverRelays fetches relay lists (kind 10002) from well-known npubs
// and returns a deduplicated, scored list of relays
func DiscoverRelays(seedRelays []string) []RelayScore {
	// Well-known, well-connected npubs to sample relay lists from
	wellKnownHexKeys := []string{
		"3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d", // fiatjaf
		"32e1827635450ebb3c5a7d12c1f8e7b2b514439ac10a67eef3d9fd9c5c68e245", // jb55
		"e88a691e98d9987c964521dff60025f60700378a4879180dcbbb4a5027850411", // NVK
		"04c915daefee38317fa734444acee390a8269fe5810b2241e5e6dd343dfbecc9", // odell
		"82341f882b6eabcd2ba7f1ef90aad961cf074af15b9ef44a09f9d2a8fbfbe6a2", // jack
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	relaySet := make(map[string]int) // url -> count of npubs using it

	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, hexKey := range wellKnownHexKeys {
		wg.Add(1)
		go func(hex string) {
			defer wg.Done()
			pk, err := nostr.PubKeyFromHex(hex)
			if err != nil {
				return
			}
			filter := nostr.Filter{
				Authors: []nostr.PubKey{pk},
				Kinds:   []nostr.Kind{10002},
				Limit:   1,
			}

			for _, seedURL := range seedRelays {
				relayCtx, relayCancel := context.WithTimeout(ctx, 5*time.Second)
				relay, err := nostr.RelayConnect(relayCtx, seedURL, nostr.RelayOptions{})
				if err != nil {
					relayCancel()
					continue
				}

				for evt := range relay.QueryEvents(filter) {
					for _, tag := range evt.Tags {
						if len(tag) >= 2 && tag[0] == "r" {
							url := normalizeRelayURL(tag[1])
							if url != "" {
								mu.Lock()
								relaySet[url]++
								mu.Unlock()
							}
						}
					}
				}
				relay.Close()
				relayCancel()
				break // got it from one seed, move on
			}
		}(hexKey)
	}

	wg.Wait()

	// Collect unique URLs
	var urls []string
	for url := range relaySet {
		urls = append(urls, url)
	}

	// Score all discovered relays in parallel
	scores := ScoreRelays(urls)

	// Sort by score descending
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Score > scores[j].Score
	})

	return scores
}

// SelectRelays picks an optimal relay set from scored candidates
func SelectRelays(candidates []RelayScore, maxCount int) []string {
	if maxCount <= 0 {
		maxCount = 5
	}

	var selected []string
	hasOutbox := false

	for _, rs := range candidates {
		if len(selected) >= maxCount {
			break
		}
		if !rs.Reachable {
			continue
		}
		if rs.PaymentRequired {
			continue // skip paid relays for default setup
		}

		// Skip inbox, search, NWC, paid relays — not useful for general publishing
		if rs.Purpose == "inbox" || rs.Purpose == "search" || rs.Purpose == "nwc" || rs.Purpose == "paid" {
			continue
		}

		// Ensure we have at least one outbox relay
		if rs.Purpose == "outbox" {
			if !hasOutbox {
				selected = append(selected, rs.URL)
				hasOutbox = true
			}
			continue
		}

		// General relays — pick by score
		if rs.Score >= 0.5 {
			selected = append(selected, rs.URL)
		}
	}

	// If no outbox relay found, add purplepag.es as fallback
	if !hasOutbox {
		selected = append(selected, "wss://purplepag.es")
	}

	return selected
}

func normalizeRelayURL(url string) string {
	url = strings.TrimSpace(url)
	url = strings.TrimRight(url, "/")
	if !strings.HasPrefix(url, "wss://") && !strings.HasPrefix(url, "ws://") {
		return ""
	}
	return url
}
