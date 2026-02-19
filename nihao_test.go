package main

import (
	"strings"
	"testing"

	"fiatjaf.com/nostr"
)

func TestIsRootNIP05(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"_@dergigi.com", true},
		{"dergigi.com", true},
		{"gigi@dergigi.com", false},
		{"bob@example.com", false},
		{"_@example.com", true},
	}
	for _, tt := range tests {
		if got := isRootNIP05(tt.input); got != tt.want {
			t.Errorf("isRootNIP05(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{-1, "unknown size"},
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{2621440, "2.5 MB"},
	}
	for _, tt := range tests {
		if got := formatSize(tt.input); got != tt.want {
			t.Errorf("formatSize(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestClassifyRelay(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"wss://purplepag.es", "outbox"},
		{"wss://relay.nos.social", "inbox"},
		{"wss://relay.damus.io", "general"},
		{"wss://relay.primal.net", "general"},
		{"wss://nos.lol", "general"},
		{"wss://premium.primal.net", "paid"},
		{"wss://nostr.wine", "paid"},
		{"wss://search.nos.today", "search"},
		{"wss://pyramid.fiatjaf.com/inbox", "inbox"},
		{"wss://nwc.example.com", "nwc"},
	}
	for _, tt := range tests {
		if got := classifyRelay(tt.url); got != tt.want {
			t.Errorf("classifyRelay(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestNormalizeRelayURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"wss://relay.damus.io", "wss://relay.damus.io"},
		{"wss://relay.damus.io/", "wss://relay.damus.io"},
		{"wss://relay.damus.io///", "wss://relay.damus.io"},
		{"  wss://nos.lol  ", "wss://nos.lol"},
		{"https://example.com", ""},
		{"", ""},
		{"ws://localhost:8080", "ws://localhost:8080"},
	}
	for _, tt := range tests {
		if got := normalizeRelayURL(tt.input); got != tt.want {
			t.Errorf("normalizeRelayURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestShouldPublishTo(t *testing.T) {
	tests := []struct {
		url  string
		kind nostr.Kind
		want bool
	}{
		// General relays accept everything
		{"wss://relay.damus.io", 0, true},
		{"wss://relay.damus.io", 1, true},
		{"wss://relay.damus.io", 10002, true},
		// Outbox relays only accept metadata kinds
		{"wss://purplepag.es", 0, true},
		{"wss://purplepag.es", 3, true},
		{"wss://purplepag.es", 10002, true},
		{"wss://purplepag.es", 1, false},
		{"wss://purplepag.es", 17375, false},
		// Inbox, search, paid, NWC — skip all
		{"wss://relay.nos.social", 0, false},
		{"wss://search.nos.today", 1, false},
		{"wss://premium.primal.net", 0, false},
	}
	for _, tt := range tests {
		if got := ShouldPublishTo(tt.url, tt.kind); got != tt.want {
			t.Errorf("ShouldPublishTo(%q, %d) = %v, want %v", tt.url, tt.kind, got, tt.want)
		}
	}
}

func TestParsePubkey(t *testing.T) {
	// Valid hex
	hex := "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d"
	pk, err := parsePubkey(hex)
	if err != nil {
		t.Fatalf("parsePubkey(hex) error: %v", err)
	}
	if pk.Hex() != hex {
		t.Errorf("parsePubkey(hex) = %s, want %s", pk.Hex(), hex)
	}

	// Valid npub (same key)
	npub := "npub180cvv07tjdrrgpa0j7j7tmnyl2yr6yr7l8j4s3evf6u64th6gkwsyjh6w6"
	pk2, err := parsePubkey(npub)
	if err != nil {
		t.Fatalf("parsePubkey(npub) error: %v", err)
	}
	if pk2.Hex() != hex {
		t.Errorf("parsePubkey(npub) = %s, want %s", pk2.Hex(), hex)
	}

	// Invalid
	_, err = parsePubkey("garbage")
	if err == nil {
		t.Error("parsePubkey(garbage) should error")
	}
}

func TestParseSetupFlags(t *testing.T) {
	args := []string{
		"--name", "test",
		"--about", "bio",
		"--json",
		"--quiet",
		"--discover",
		"--no-wallet",
		"--no-dm-relays",
		"--relays", "wss://a.com,wss://b.com",
		"--dm-relays", "wss://dm1.com,wss://dm2.com",
		"--sec", "deadbeef",
		"--nsec-cmd", "pass insert nostr",
		"--mint", "https://mint1.com",
		"--mint", "https://mint2.com",
	}
	opts := parseSetupFlags(args)

	if opts.name != "test" {
		t.Errorf("name = %q, want %q", opts.name, "test")
	}
	if opts.about != "bio" {
		t.Errorf("about = %q, want %q", opts.about, "bio")
	}
	if !opts.jsonOutput {
		t.Error("jsonOutput should be true")
	}
	if !opts.quiet {
		t.Error("quiet should be true")
	}
	if !opts.discover {
		t.Error("discover should be true")
	}
	if !opts.noWallet {
		t.Error("noWallet should be true")
	}
	if !opts.noDMRelays {
		t.Error("noDMRelays should be true")
	}
	if len(opts.relays) != 2 || opts.relays[0] != "wss://a.com" {
		t.Errorf("relays = %v", opts.relays)
	}
	if len(opts.dmRelays) != 2 || opts.dmRelays[0] != "wss://dm1.com" {
		t.Errorf("dmRelays = %v", opts.dmRelays)
	}
	if opts.sec != "deadbeef" {
		t.Errorf("sec = %q", opts.sec)
	}
	if opts.nsecCmd != "pass insert nostr" {
		t.Errorf("nsecCmd = %q", opts.nsecCmd)
	}
	if len(opts.mints) != 2 {
		t.Errorf("mints = %v, want 2 items", opts.mints)
	}

	// Test --nsec alias
	nsecOpts := parseSetupFlags([]string{"--nsec", "deadbeef2"})
	if nsecOpts.sec != "deadbeef2" {
		t.Errorf("--nsec alias: sec = %q, want %q", nsecOpts.sec, "deadbeef2")
	}
}

func TestMarkedRelaysToTags(t *testing.T) {
	relays := []MarkedRelay{
		{URL: "wss://a.com", Marker: RelayMarkerBoth},
		{URL: "wss://b.com", Marker: RelayMarkerRead},
		{URL: "wss://c.com", Marker: RelayMarkerWrite},
	}
	tags := MarkedRelaysToTags(relays)

	if len(tags) != 3 {
		t.Fatalf("got %d tags, want 3", len(tags))
	}
	// Both = no marker (2 elements)
	if len(tags[0]) != 2 || tags[0][1] != "wss://a.com" {
		t.Errorf("tag[0] = %v", tags[0])
	}
	// Read = 3 elements
	if len(tags[1]) != 3 || tags[1][2] != "read" {
		t.Errorf("tag[1] = %v", tags[1])
	}
	// Write = 3 elements
	if len(tags[2]) != 3 || tags[2][2] != "write" {
		t.Errorf("tag[2] = %v", tags[2])
	}
}

func TestImageHostingTier(t *testing.T) {
	tests := []struct {
		url         string
		nip05Domain string
		wantTier    string
	}{
		{"https://blossom.primal.net/abc.jpg", "", "blossom"},
		{"https://files.v0l.io/abc.jpg", "", "blossom"},
		{"https://dergigi.com/img.jpg", "dergigi.com", "own"},
		{"https://dergigi.com/img.jpg", "", "third-party"},
		{"https://imgur.com/abc.jpg", "dergigi.com", "third-party"},
	}
	for _, tt := range tests {
		info := imageInfo{URL: tt.url, Status: 200}
		// Set Blossom flag based on known hosts
		for host := range knownBlossomHosts {
			if strings.Contains(tt.url, host) {
				info.Blossom = true
				break
			}
		}
		tier, _ := imageHostingTier(info, tt.nip05Domain)
		if tier != tt.wantTier {
			t.Errorf("imageHostingTier(%q, %q) = %q, want %q", tt.url, tt.nip05Domain, tier, tt.wantTier)
		}
	}
}

func TestAddCheck(t *testing.T) {
	r := &CheckResult{}
	r.addCheck("test", "pass", "detail")
	if len(r.Checks) != 1 {
		t.Fatalf("got %d checks, want 1", len(r.Checks))
	}
	if r.Checks[0].Name != "test" || r.Checks[0].Status != "pass" || r.Checks[0].Detail != "detail" {
		t.Errorf("check = %+v", r.Checks[0])
	}
}
