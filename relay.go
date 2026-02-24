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

// ──────────────────────────────────────────────────────────────
// Relay classification config
//
// We classify relays into purposes to decide what events to send
// where. This avoids publishing kind 1 to purplepag.es, sending
// events to paid relays that will reject them, etc.
// ──────────────────────────────────────────────────────────────

// knownRelayPurposes maps specific relay URLs to their purpose.
var knownRelayPurposes = map[string]string{
	// Outbox-only: relay list aggregators (accept kind 0, 3, 10002)
	"wss://purplepag.es": "outbox",

	// Inbox: accept mentions/replies but reject general posts
	"wss://relay.nos.social":  "inbox",
	"wss://inbox.relays.land": "inbox",

	// Search: NIP-50 search relays
	"wss://search.nos.today": "search",

	// Paid: accept connections but reject writes without subscription
	"wss://premium.primal.net": "paid",
	"wss://nostr.wine":         "paid",
}

// urlPatterns maps URL substrings to relay purposes.
// Checked in order when no exact match is found.
var urlPatterns = []struct {
	pattern string
	purpose string
}{
	{"/inbox", "inbox"},     // e.g. pyramid.fiatjaf.com/inbox
	{"nwc.", "nwc"},         // NWC endpoints, not general relays
	{"pyramid.", "paid"},    // pyramid relays require membership
	{"premium.", "paid"},    // premium tier relays
}

// wellConnectedNpubs are hex pubkeys of well-known, well-connected users.
// Used to sample relay lists (kind 10002) and DM relays (kind 10050)
// during discovery.
var wellConnectedNpubs = []string{
	"3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d", // fiatjaf
	"32e1827635450ebb3c5a7d12c1f8e7b2b514439ac10a67eef3d9fd9c5c68e245", // jb55
	"e88a691e98d9987c964521dff60025f60700378a4879180dcbbb4a5027850411", // NVK
	"04c915daefee38317fa734444acee390a8269fe5810b2241e5e6dd343dfbecc9", // odell
	"82341f882b6eabcd2ba7f1ef90aad961cf074af15b9ef44a09f9d2a8fbfbe6a2", // jack
}

// outboxKinds are the only event kinds sent to outbox-purpose relays.
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

// classifyRelay determines a relay's purpose.
func classifyRelay(relayURL string) string {
	if purpose, ok := knownRelayPurposes[relayURL]; ok {
		return purpose
	}
	for _, p := range urlPatterns {
		if strings.Contains(relayURL, p.pattern) {
			return p.purpose
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	relaySet := make(map[string]int) // url -> count of npubs using it

	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, hexKey := range wellConnectedNpubs {
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

// RelayMarker represents the NIP-65 read/write marker for a relay
type RelayMarker string

const (
	RelayMarkerRead  RelayMarker = "read"
	RelayMarkerWrite RelayMarker = "write"
	RelayMarkerBoth  RelayMarker = "" // no marker = both
)

// MarkedRelay is a relay URL with its NIP-65 read/write marker
type MarkedRelay struct {
	URL    string      `json:"url"`
	Marker RelayMarker `json:"marker,omitempty"` // "read", "write", or "" (both)
}

// DefaultMarkedRelays returns the default relay set with proper NIP-65 markers
func DefaultMarkedRelays() []MarkedRelay {
	return []MarkedRelay{
		{URL: "wss://relay.damus.io", Marker: RelayMarkerBoth},
		{URL: "wss://relay.primal.net", Marker: RelayMarkerBoth},
		{URL: "wss://nos.lol", Marker: RelayMarkerBoth},
		// purplepag.es is NOT in kind 10002 — it's a relay list aggregator, not a content relay
		// We still publish TO it (outbox kinds only), but don't advertise it
	}
}

// DefaultDMRelays is nil — when no --dm-relays are specified, nihao uses the
// same relays as kind 10002 (general relay list). This avoids the problem of
// defaulting to paid relays that the user can't read from.
var DefaultDMRelays []string

// ClassifyDiscoveredRelay assigns a NIP-65 marker to a discovered relay
func ClassifyDiscoveredRelay(url string) (MarkedRelay, bool) {
	purpose := classifyRelay(url)
	switch purpose {
	case "outbox":
		// purplepag.es etc should NOT be in kind 10002
		return MarkedRelay{}, false
	case "inbox":
		return MarkedRelay{URL: url, Marker: RelayMarkerRead}, true
	case "paid", "nwc", "search":
		return MarkedRelay{}, false
	}
	// General relays are both read+write
	return MarkedRelay{URL: url, Marker: RelayMarkerBoth}, true
}

// MarkedRelaysToTags converts marked relays to NIP-65 tags
func MarkedRelaysToTags(relays []MarkedRelay) nostr.Tags {
	var tags nostr.Tags
	for _, r := range relays {
		if r.Marker == RelayMarkerBoth {
			tags = append(tags, nostr.Tag{"r", r.URL})
		} else {
			tags = append(tags, nostr.Tag{"r", r.URL, string(r.Marker)})
		}
	}
	return tags
}

// MarkedRelayURLs extracts just the URLs from marked relays
func MarkedRelayURLs(relays []MarkedRelay) []string {
	var urls []string
	for _, r := range relays {
		urls = append(urls, r.URL)
	}
	return urls
}

// DiscoverDMRelays looks for kind 10050 events from well-connected npubs
func DiscoverDMRelays(seedRelays []string) []string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	relaySet := make(map[string]int)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, hexKey := range wellConnectedNpubs {
		wg.Add(1)
		go func(hex string) {
			defer wg.Done()
			pk, err := nostr.PubKeyFromHex(hex)
			if err != nil {
				return
			}
			filter := nostr.Filter{
				Authors: []nostr.PubKey{pk},
				Kinds:   []nostr.Kind{10050},
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
						if len(tag) >= 2 && tag[0] == "relay" {
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
				break
			}
		}(hexKey)
	}
	wg.Wait()

	// Return relays used by 2+ npubs, or fall back to defaults
	var discovered []string
	for url, count := range relaySet {
		if count >= 2 {
			discovered = append(discovered, url)
		}
	}
	if len(discovered) == 0 {
		return DefaultDMRelays
	}
	return discovered
}

func normalizeRelayURL(url string) string {
	url = strings.TrimSpace(url)
	url = strings.TrimRight(url, "/")
	if !strings.HasPrefix(url, "wss://") && !strings.HasPrefix(url, "ws://") {
		return ""
	}
	return url
}
