package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/nip19"
)

// BackupResult holds all identity events for export.
type BackupResult struct {
	Npub   string             `json:"npub"`
	Pubkey string             `json:"pubkey"`
	Events []BackupEvent      `json:"events"`
	Meta   BackupMeta         `json:"meta"`
}

// BackupEvent wraps a nostr event with its kind label for readability.
type BackupEvent struct {
	Kind      int              `json:"kind"`
	KindLabel string           `json:"kind_label"`
	Event     *nostr.Event     `json:"event"`
}

// BackupMeta holds metadata about the backup itself.
type BackupMeta struct {
	CreatedAt string `json:"created_at"`
	Version   string `json:"version"`
	Relays    []string `json:"relays_queried"`
}

// kindLabels maps event kinds to human-readable labels.
var kindLabels = map[int]string{
	0:     "profile",
	3:     "follow_list",
	10002: "relay_list",
	10050: "dm_relay_list",
	10019: "nutzap_info",
	17375: "wallet",
	37375: "wallet_old",
}

// backupKinds is the ordered list of kinds to back up.
var backupKinds = []int{0, 3, 10002, 10050, 10019, 17375, 37375}

func runBackup(target string, quiet bool, relays []string) {
	if target == "" {
		fatal("usage: nihao backup <npub|nip05>")
	}

	pk, err := resolveTarget(target, quiet)
	if err != nil {
		fatal("%s", err)
	}

	npub := nip19.EncodeNpub(pk)
	if !quiet {
		fmt.Fprintf(os.Stderr, "nihao backup 📦 %s\n\n", npub)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Connect to relays
	checkRelays := connectCheckRelays(ctx, relays)
	if len(checkRelays) == 0 {
		fatal("could not connect to any relay")
	}
	defer func() {
		for _, cr := range checkRelays {
			cr.relay.Close()
		}
	}()

	var relayURLs []string
	for _, cr := range checkRelays {
		relayURLs = append(relayURLs, cr.url)
	}

	result := BackupResult{
		Npub:   npub,
		Pubkey: pk.Hex(),
		Events: []BackupEvent{}, // empty slice, not nil (ensures JSON "events": [] not null)
		Meta: BackupMeta{
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
			Version:   version,
			Relays:    relayURLs,
		},
	}

	found := 0
	for _, kind := range backupKinds {
		kindCtx, kindCancel := context.WithTimeout(ctx, 5*time.Second)
		_, evt := fetchKindFrom(kindCtx, checkRelays, pk, kind)
		kindCancel()
		if evt != nil {
			label := kindLabels[kind]
			if label == "" {
				label = fmt.Sprintf("kind_%d", kind)
			}
			result.Events = append(result.Events, BackupEvent{
				Kind:      kind,
				KindLabel: label,
				Event:     evt,
			})
			found++
			if !quiet {
				fmt.Fprintf(os.Stderr, "  ✓ kind %d (%s)\n", kind, label)
			}
		} else if !quiet {
			label := kindLabels[kind]
			if label == "" {
				label = fmt.Sprintf("kind_%d", kind)
			}
			fmt.Fprintf(os.Stderr, "  · kind %d (%s) — not found\n", kind, label)
		}
	}

	if !quiet {
		fmt.Fprintf(os.Stderr, "\n  📦 %d event(s) backed up\n", found)
	}

	// Always output JSON to stdout (this IS the backup)
	out, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(out))
}
