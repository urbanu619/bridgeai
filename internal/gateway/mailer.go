package main

import (
	"fmt"
	"net/smtp"
	"os"
)

// SendKeyEmail sends the issued API key to the applicant via Gmail SMTP.
func SendKeyEmail(toEmail, apiKey string) error {
	from := os.Getenv("GMAIL_USER")
	pass := os.Getenv("GMAIL_APP_PASSWORD")
	if from == "" || pass == "" {
		return fmt.Errorf("GMAIL_USER or GMAIL_APP_PASSWORD not set")
	}

	subject := "Your BridgeAI API Key"
	body := fmt.Sprintf(`Hi,

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
`, apiKey, gatewayURL(), gatewayURL(), apiKey)

	msg := "From: " + from + "\r\n" +
		"To: " + toEmail + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"\r\n" + body

	auth := smtp.PlainAuth("", from, pass, "smtp.gmail.com")
	return smtp.SendMail("smtp.gmail.com:587", auth, from, []string{toEmail}, []byte(msg))
}

func gatewayURL() string {
	if u := os.Getenv("GATEWAY_URL"); u != "" {
		return u
	}
	return "https://your-gateway-url"
}
