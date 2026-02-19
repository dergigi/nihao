package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/keyer"
	"github.com/btcsuite/btcd/btcec/v2"
)

// WalletSetupResult holds the output of wallet creation.
type WalletSetupResult struct {
	P2PKPubkey string   `json:"p2pk_pubkey"`
	Mints      []string `json:"mints"`
}

// setupWallet creates a NIP-60 wallet and publishes kind 17375 + kind 10019.
// Returns the wallet setup result or an error.
// The quiet parameter suppresses non-error output to avoid polluting --json.
func setupWallet(ctx context.Context, sk nostr.SecretKey, relays []string, mintInfos []MintInfo, quiet bool, pool ...*RelayPool) (*WalletSetupResult, error) {
	kr := keyer.NewPlainKeySigner(sk)

	// Step 1: Generate a separate P2PK private key for the wallet
	var walletSkBytes [32]byte
	if _, err := rand.Read(walletSkBytes[:]); err != nil {
		return nil, fmt.Errorf("failed to generate wallet key: %w", err)
	}

	walletPrivKey, walletPubKey := btcec.PrivKeyFromBytes(walletSkBytes[:])
	_ = walletPrivKey // used in encrypted content

	// Compressed pubkey hex (02-prefixed for cashu P2PK compatibility)
	p2pkPubkey := nostr.HexEncodeToString(walletPubKey.SerializeCompressed())

	// Collect mint URLs
	var mintURLs []string
	for _, m := range mintInfos {
		mintURLs = append(mintURLs, m.URL)
	}

	// Step 2: Build and publish wallet event (kind 17375)
	// Encrypted content: [["privkey", "<hex>"], ["mint", "<url>"], ...]
	encryptedTags := nostr.Tags{
		nostr.Tag{"privkey", nostr.HexEncodeToString(walletPrivKey.Serialize())},
	}
	for _, url := range mintURLs {
		encryptedTags = append(encryptedTags, nostr.Tag{"mint", url})
	}

	tagsJSON, _ := json.Marshal(encryptedTags)

	pk, _ := kr.GetPublicKey(ctx)
	encryptedContent, err := kr.Encrypt(ctx, string(tagsJSON), pk)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt wallet event: %w", err)
	}

	walletEvt := nostr.Event{
		CreatedAt: nostr.Now(),
		Kind:      17375,
		Tags:      nostr.Tags{},
		Content:   encryptedContent,
	}
	if err := kr.SignEvent(ctx, &walletEvt); err != nil {
		return nil, fmt.Errorf("failed to sign wallet event: %w", err)
	}

	if !quiet {
		fmt.Println("💰 Publishing wallet (kind 17375)...")
	}
	if len(pool) > 0 && pool[0] != nil {
		pool[0].Publish(walletEvt)
	} else {
		publishToRelays(walletEvt, relays, quiet)
	}
	if !quiet {
		fmt.Println()
	}

	// Step 3: Build and publish nutzap info (kind 10019)
	nutzapTags := nostr.Tags{}

	// Add relay tags
	for _, r := range relays {
		nutzapTags = append(nutzapTags, nostr.Tag{"relay", r})
	}

	// Add mint tags with sat unit
	for _, url := range mintURLs {
		nutzapTags = append(nutzapTags, nostr.Tag{"mint", url, "sat"})
	}

	// Add P2PK pubkey
	nutzapTags = append(nutzapTags, nostr.Tag{"pubkey", p2pkPubkey})

	nutzapEvt := nostr.Event{
		CreatedAt: nostr.Now(),
		Kind:      10019,
		Tags:      nutzapTags,
		Content:   "",
	}
	if err := kr.SignEvent(ctx, &nutzapEvt); err != nil {
		return nil, fmt.Errorf("failed to sign nutzap info event: %w", err)
	}

	if !quiet {
		fmt.Println("⚡ Publishing nutzap info (kind 10019)...")
	}
	if len(pool) > 0 && pool[0] != nil {
		pool[0].Publish(nutzapEvt)
	} else {
		publishToRelays(nutzapEvt, relays, quiet)
	}
	if !quiet {
		fmt.Println()
	}

	return &WalletSetupResult{
		P2PKPubkey: p2pkPubkey,
		Mints:      mintURLs,
	}, nil
}
