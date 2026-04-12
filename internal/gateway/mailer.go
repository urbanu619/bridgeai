package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// SendKeyEmail sends the issued API key to the applicant via Postmark API.
func SendKeyEmail(toEmail, apiKey string) error {
	token := os.Getenv("POSTMARK_TOKEN")
	if token == "" {
		return fmt.Errorf("POSTMARK_TOKEN not set")
	}

	from := os.Getenv("RESEND_FROM")
	if from == "" {
		from = "BridgeAI <noreply@dnsv4.cn>"
	}

	body := map[string]any{
		"From":    from,
		"To":      toEmail,
		"Subject": "Your BridgeAI API Key",
		"TextBody": fmt.Sprintf(`Hi,

Your BridgeAI API Key is ready:

  %s

Endpoint: %s/v1/chat/completions

Usage example:
  curl -X POST %s/v1/chat/completions \
    -H "Authorization: Bearer %s" \
    -H "Content-Type: application/json" \
    -d '{"model":"smart-fast","messages":[{"role":"user","content":"Hello"}]}'

Demo page: https://urbanu619.github.io/bridgeai/demo.html

If you have any questions, reply to this email.

— BridgeAI
`, apiKey, gatewayURL(), gatewayURL(), apiKey),
	}

	raw, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, "https://api.postmarkapp.com/email", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("X-Postmark-Server-Token", token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("postmark API error %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func gatewayURL() string {
	if u := os.Getenv("GATEWAY_URL"); u != "" {
		return u
	}
	return "https://bridgeai-production-e475.up.railway.app"
}
