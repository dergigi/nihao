package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
)

// MintInfo holds the result of validating a Cashu mint.
type MintInfo struct {
	URL           string   `json:"url"`
	Name          string   `json:"name,omitempty"`
	Version       string   `json:"version,omitempty"`
	Reachable     bool     `json:"reachable"`
	HasSatKeyset  bool     `json:"has_sat_keyset"`
	SupportsP2PK  bool     `json:"supports_p2pk"`  // NUT-11
	SupportsMint  bool     `json:"supports_mint"`   // NUT-04
	SupportsMelt  bool     `json:"supports_melt"`   // NUT-05
	Valid         bool     `json:"valid"`            // all checks pass
	SupportedNuts []string `json:"supported_nuts,omitempty"`
	Error         string   `json:"error,omitempty"`
}

// mintInfoResponse represents the /v1/info response from a Cashu mint.
type mintInfoResponse struct {
	Name    string                       `json:"name"`
	Version string                       `json:"version"`
	Nuts    map[string]json.RawMessage   `json:"nuts"`
}

// mintKeysResponse represents the /v1/keys response.
type mintKeysResponse struct {
	Keysets []mintKeyset `json:"keysets"`
}

type mintKeyset struct {
	ID     string            `json:"id"`
	Unit   string            `json:"unit"`
	Keys   map[string]string `json:"keys"`
	Active bool              `json:"active"`
}

// Default mints for NIP-60 wallet setup — curated for reliability.
// All must support NUT-11 (P2PK), NUT-04 (mint), NUT-05 (melt), sat unit.
var defaultMints = []string{
	"https://mint.minibits.cash/Bitcoin",
	"https://mint.coinos.io",
	"https://mint.macadamia.cash",
}

// validateMint probes a Cashu mint and checks if it meets our requirements.
func validateMint(ctx context.Context, mintURL string) MintInfo {
	info := MintInfo{URL: mintURL}

	// Normalize URL
	mintURL = strings.TrimRight(mintURL, "/")

	// Step 1: Fetch /v1/info
	mintResp, err := httpGetJSON[mintInfoResponse](ctx, mintURL+"/v1/info")
	if err != nil {
		info.Error = fmt.Sprintf("unreachable: %s", err)
		return info
	}
	info.Reachable = true
	info.Name = mintResp.Name
	info.Version = mintResp.Version

	// Parse supported NUTs
	for nut := range mintResp.Nuts {
		info.SupportedNuts = append(info.SupportedNuts, nut)
	}

	// Check required NUTs
	_, info.SupportsMint = mintResp.Nuts["4"]   // NUT-04: mint tokens
	_, info.SupportsMelt = mintResp.Nuts["5"]   // NUT-05: melt tokens
	_, info.SupportsP2PK = mintResp.Nuts["11"]  // NUT-11: P2PK spending conditions

	// Step 2: Fetch /v1/keys — check for active sat keyset
	keysResp, err := httpGetJSON[mintKeysResponse](ctx, mintURL+"/v1/keys")
	if err != nil {
		info.Error = fmt.Sprintf("failed to fetch keysets: %s", err)
		return info
	}

	for _, ks := range keysResp.Keysets {
		if ks.Unit == "sat" && len(ks.Keys) > 0 {
			info.HasSatKeyset = true
			break
		}
	}

	// Determine overall validity
	info.Valid = info.Reachable && info.HasSatKeyset && info.SupportsP2PK && info.SupportsMint && info.SupportsMelt

	if !info.Valid {
		var missing []string
		if !info.HasSatKeyset {
			missing = append(missing, "no sat keyset")
		}
		if !info.SupportsP2PK {
			missing = append(missing, "no P2PK (NUT-11)")
		}
		if !info.SupportsMint {
			missing = append(missing, "no mint (NUT-04)")
		}
		if !info.SupportsMelt {
			missing = append(missing, "no melt (NUT-05)")
		}
		info.Error = strings.Join(missing, ", ")
	}

	return info
}

// validateMints validates multiple mints in sequence and returns only the valid ones.
func validateMints(ctx context.Context, urls []string) (valid []MintInfo, invalid []MintInfo) {
	for _, url := range urls {
		info := validateMint(ctx, url)
		if info.Valid {
			valid = append(valid, info)
		} else {
			invalid = append(invalid, info)
		}
	}
	return
}

// selectMints returns the mint URLs to use for wallet setup.
// If user provided --mint flags, use those. Otherwise use curated defaults.
// All mints are validated before use.
func selectMints(ctx context.Context, userMints []string, quiet bool) ([]MintInfo, error) {
	candidates := defaultMints
	if len(userMints) > 0 {
		candidates = userMints
	}

	valid, invalid := validateMints(ctx, candidates)

	// Log invalid mints
	if !quiet {
		for _, m := range invalid {
			fmt.Printf("   ✗ %s (%s)\n", m.URL, m.Error)
		}
	}

	if len(valid) == 0 {
		return nil, fmt.Errorf("no valid mints found")
	}

	// Cap at 2 mints for simplicity
	if len(valid) > 2 && slices.Equal(candidates, defaultMints) {
		valid = valid[:2]
	}

	return valid, nil
}

// httpGetJSON fetches a URL and decodes the JSON response.
func httpGetJSON[T any](ctx context.Context, url string) (*T, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var result T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}
