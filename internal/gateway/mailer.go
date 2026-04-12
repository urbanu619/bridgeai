package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// SendKeyEmail sends the issued API key to the applicant via Resend API.
func SendKeyEmail(toEmail, apiKey string) error {
	resendKey := os.Getenv("RESEND_API_KEY")
	if resendKey == "" {
		return fmt.Errorf("RESEND_API_KEY not set")
	}

	from := os.Getenv("RESEND_FROM")
	if from == "" {
		from = "BridgeAI <onboarding@resend.dev>"
	}

	body := map[string]any{
		"from":    from,
		"to":      []string{toEmail},
		"subject": "Your BridgeAI API Key",
		"text": fmt.Sprintf(`Hi,

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
	req, err := http.NewRequest(http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+resendKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("resend API error %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func gatewayURL() string {
	if u := os.Getenv("GATEWAY_URL"); u != "" {
		return u
	}
	return "https://bridgeai-production-e475.up.railway.app"
}
