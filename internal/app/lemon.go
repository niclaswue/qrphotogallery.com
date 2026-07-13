package app

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

const lemonCheckoutsURL = "https://api.lemonsqueezy.com/v1/checkouts"

var lemonHTTPClient = &http.Client{Timeout: 15 * time.Second}

// createLemonCheckout asks Lemon Squeezy to mint a hosted checkout URL for
// the selected plan variant. user_id and tier travel through the webhook in
// custom_data so we can upgrade the right account.
func createLemonCheckout(variantID, userID, userEmail, tierName, redirectURL string) (string, error) {
	ls := appConfig.LemonSqueezy
	if ls.APIKey == "" || ls.StoreID == "" {
		return "", fmt.Errorf("lemon squeezy not configured")
	}

	// enabled_variants locks the LS hosted checkout to a single variant so a
	// visitor on the €29 price page can't switch to the €69 variant from a
	// dropdown on the LS page.
	payload := map[string]any{
		"data": map[string]any{
			"type": "checkouts",
			"attributes": map[string]any{
				"checkout_data": map[string]any{
					"email":  userEmail,
					"custom": map[string]string{"user_id": userID, "tier": tierName},
				},
				"product_options": map[string]any{
					"redirect_url":     redirectURL,
					"enabled_variants": []string{variantID},
				},
			},
			"relationships": map[string]any{
				"store":   map[string]any{"data": map[string]string{"type": "stores", "id": ls.StoreID}},
				"variant": map[string]any{"data": map[string]string{"type": "variants", "id": variantID}},
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, lemonCheckoutsURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.api+json")
	req.Header.Set("Content-Type", "application/vnd.api+json")
	req.Header.Set("Authorization", "Bearer "+ls.APIKey)

	resp, err := lemonHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("lemon squeezy returned %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed struct {
		Data struct {
			Attributes struct {
				URL string `json:"url"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", err
	}
	if parsed.Data.Attributes.URL == "" {
		return "", fmt.Errorf("lemon squeezy response missing checkout url")
	}
	return parsed.Data.Attributes.URL, nil
}

// tierNameByVariantID returns the configured tier whose Lemon Squeezy variant
// ID matches, or "" if none match.
func tierNameByVariantID(variantID string) string {
	if variantID == "" {
		return ""
	}
	for _, t := range appConfig.Tiers {
		if t.LemonSqueezyVariantID == variantID {
			return t.Name
		}
	}
	return ""
}

// verifyLemonSignature compares HMAC-SHA256(body, secret) with the value of
// X-Signature (hex). Uses constant-time comparison to avoid timing leaks.
func verifyLemonSignature(body []byte, signature, secret string) bool {
	if secret == "" || signature == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := mac.Sum(nil)
	provided, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}
	return hmac.Equal(expected, provided)
}

type lemonWebhookPayload struct {
	Meta struct {
		EventName  string `json:"event_name"`
		CustomData struct {
			UserID string `json:"user_id"`
			Tier   string `json:"tier"`
		} `json:"custom_data"`
	} `json:"meta"`
	Data struct {
		Type       string `json:"type"`
		Attributes struct {
			Status         string `json:"status"`
			UserEmail      string `json:"user_email"`
			FirstOrderItem struct {
				VariantID int64 `json:"variant_id"`
			} `json:"first_order_item"`
		} `json:"attributes"`
	} `json:"data"`
}

// isKnownTierName guards against trusting an arbitrary string from
// custom_data.
func isKnownTierName(name string) bool {
	if name == "" {
		return false
	}
	for _, t := range appConfig.Tiers {
		if t.Name == name {
			return true
		}
	}
	return false
}

func handleLemonWebhook(e *core.RequestEvent) error {
	secret := appConfig.LemonSqueezy.WebhookSecret
	if secret == "" {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{"error": "webhook not configured"})
	}

	body, err := io.ReadAll(e.Request.Body)
	if err != nil {
		return e.BadRequestError("Failed to read body", err)
	}

	sig := e.Request.Header.Get("X-Signature")
	if !verifyLemonSignature(body, sig, secret) {
		log.Printf("lemon webhook: invalid signature")
		return e.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid signature"})
	}

	var payload lemonWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return e.BadRequestError("Invalid payload", err)
	}

	// Only upgrade on a paid order. We intentionally ignore subscription_* for
	// this MVP since the configured products are one-time payments.
	if payload.Meta.EventName != "order_created" {
		return e.JSON(http.StatusOK, map[string]string{"status": "ignored"})
	}
	if payload.Data.Attributes.Status != "paid" {
		return e.JSON(http.StatusOK, map[string]string{"status": "not paid"})
	}

	userID := payload.Meta.CustomData.UserID
	if userID == "" {
		log.Printf("lemon webhook: missing user_id in custom_data")
		return e.BadRequestError("Missing user_id", nil)
	}

	variantID := fmt.Sprintf("%d", payload.Data.Attributes.FirstOrderItem.VariantID)
	// Prefer the tier declared in custom_data. Fall back to inferring from the
	// Lemon Squeezy variant ID for historical payments created before `tier`
	// was included, and reject anything that isn't a known tier.
	tierName := payload.Meta.CustomData.Tier
	if !isKnownTierName(tierName) {
		tierName = tierNameByVariantID(variantID)
	}
	if tierName == "" {
		log.Printf("lemon webhook: unknown variant_id %s", variantID)
		return e.JSON(http.StatusOK, map[string]string{"status": "unknown variant"})
	}

	user, err := e.App.FindRecordById("users", userID)
	if err != nil {
		log.Printf("lemon webhook: user %s not found: %v", userID, err)
		return e.JSON(http.StatusOK, map[string]string{"status": "user not found"})
	}
	user.Set("tier", tierName)
	if err := e.App.Save(user); err != nil {
		log.Printf("lemon webhook: failed to upgrade user %s to %s: %v", userID, tierName, err)
		return e.InternalServerError("Failed to upgrade user", err)
	}
	log.Printf("lemon webhook: upgraded user %s to tier %s", userID, tierName)
	return e.JSON(http.StatusOK, map[string]string{"status": "upgraded", "tier": tierName})
}
