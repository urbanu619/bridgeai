package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)


// BYOKKeys holds per-provider keys supplied by the client.
type BYOKKeys struct {
	Anthropic string
	Groq      string
	Gemini    string
}

// For returns the BYOK key for the given provider type, or empty string if not set.
func (b BYOKKeys) For(providerType string) string {
	switch providerType {
	case "anthropic":
		return b.Anthropic
	case "groq":
		return b.Groq
	case "gemini":
		return b.Gemini
	}
	return ""
}

// Chain tries providers in order, falling back on retryable errors.
type Chain struct {
	providers []Provider
	budget    *BudgetStore
}

// isRetryable returns true for errors we should try the next provider for.
func isRetryable(statusCode int) bool {
	return statusCode == 429 || (statusCode >= 500 && statusCode < 600)
}

// ChatCompletions is the Gin handler for POST /v1/chat/completions.
func (ch *Chain) ChatCompletions(c *gin.Context) {
	requestID := c.GetString("request_id")

	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "request_id": requestID})
		return
	}

	// Extract BridgeAI key and BYOK keys from headers.
	apiKey := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	byok := BYOKKeys{
		Anthropic: c.GetHeader("X-Anthropic-Key"),
		Groq:      c.GetHeader("X-Groq-Key"),
		Gemini:    c.GetHeader("X-Gemini-Key"),
	}
	isBYOK := byok.Anthropic != "" || byok.Groq != "" || byok.Gemini != ""

	// Budget pre-check — skip if user supplies their own keys.
	if !isBYOK && ch.budget != nil && !ch.budget.Check(apiKey) {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error":      "budget_exceeded",
			"retryable":  false,
			"request_id": requestID,
		})
		return
	}

	stream, _ := body["stream"].(bool)

	var lastStatus int
	for _, p := range ch.providers {
		resp, err := p.Send(c.Request.Context(), body, requestID, byok.For(p.Type()))
		if err != nil {
			log.Printf("[%s] provider=%s network_err=%v — trying next", requestID, p.Type(), err)
			continue
		}

		if isRetryable(resp.StatusCode) {
			resp.Body.Close()
			lastStatus = resp.StatusCode
			log.Printf("[%s] provider=%s status=%d — trying next", requestID, p.Type(), resp.StatusCode)
			continue
		}

		// Non-retryable: deliver to client.
		log.Printf("[%s] provider=%s status=%d stream=%v", requestID, p.Type(), resp.StatusCode, stream)
		c.Header("X-Request-ID", requestID)

		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			c.Data(resp.StatusCode, "application/json", b)
			return
		}

		defer resp.Body.Close()
		billingKey := apiKey
		if isBYOK {
			billingKey = "" // don't bill platform budget for BYOK
		}
		if stream {
			ch.handleStream(c, resp, p, requestID)
		} else {
			ch.handleSync(c, resp, p, requestID, billingKey)
		}
		return
	}

	// All providers exhausted.
	status := http.StatusBadGateway
	if lastStatus != 0 {
		status = lastStatus
	}
	c.JSON(status, gin.H{"error": "all providers failed", "request_id": requestID})
}

// handleSync delivers a non-streaming response.
func (ch *Chain) handleSync(c *gin.Context, resp *http.Response, p Provider, requestID string, apiKey string) {
	if !p.NeedsTransform() {
		// OpenAI-compatible: decode to extract usage for billing, then re-encode.
		var r map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "decode error", "request_id": requestID})
			return
		}
		ch.recordUsage(apiKey, r)
		c.Header("Content-Type", "application/json")
		b, _ := json.Marshal(r)
		c.Data(http.StatusOK, "application/json", b)
		return
	}
	// Anthropic → OpenAI conversion
	var r map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "decode error", "request_id": requestID})
		return
	}
	content := ""
	if contents, ok := r["content"].([]any); ok && len(contents) > 0 {
		if block, ok := contents[0].(map[string]any); ok {
			content, _ = block["text"].(string)
		}
	}
	out := map[string]any{
		"id":     requestID,
		"object": "chat.completion",
		"model":  r["model"],
		"choices": []map[string]any{{
			"index":         0,
			"message":       map[string]any{"role": "assistant", "content": content},
			"finish_reason": r["stop_reason"],
		}},
	}
	if usage, ok := r["usage"].(map[string]any); ok {
		out["usage"] = map[string]any{
			"prompt_tokens":     usage["input_tokens"],
			"completion_tokens": usage["output_tokens"],
		}
		ch.recordUsageRaw(apiKey, modelStr(r["model"]),
			intVal(usage["input_tokens"]), intVal(usage["output_tokens"]))
	}
	c.JSON(http.StatusOK, out)
}

// recordUsage extracts token counts from an OpenAI-format response and bills the key.
func (ch *Chain) recordUsage(apiKey string, r map[string]any) {
	if ch.budget == nil {
		return
	}
	model := modelStr(r["model"])
	if usage, ok := r["usage"].(map[string]any); ok {
		ch.recordUsageRaw(apiKey, model, intVal(usage["prompt_tokens"]), intVal(usage["completion_tokens"]))
	}
}

func (ch *Chain) recordUsageRaw(apiKey, model string, prompt, completion int) {
	if ch.budget == nil {
		return
	}
	ch.budget.Add(apiKey, model, prompt, completion)
}

func modelStr(v any) string {
	s, _ := v.(string)
	return s
}

func intVal(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return 0
}

// handleStream pipes an SSE response to the client.
func (ch *Chain) handleStream(c *gin.Context, resp *http.Response, p Provider, requestID string) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return
	}

	if !p.NeedsTransform() {
		// OpenAI-compatible: pipe directly.
		io.Copy(c.Writer, resp.Body)
		return
	}

	// Anthropic SSE → OpenAI SSE conversion.
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
			flusher.Flush()
			return
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}
		chunk := anthropicEventToOpenAI(event, requestID)
		if chunk == nil {
			continue
		}
		b, _ := json.Marshal(chunk)
		fmt.Fprintf(c.Writer, "data: %s\n\n", b)
		flusher.Flush()
	}
}

func anthropicEventToOpenAI(event map[string]any, requestID string) map[string]any {
	switch event["type"] {
	case "content_block_delta":
		delta, _ := event["delta"].(map[string]any)
		text, _ := delta["text"].(string)
		return map[string]any{
			"id": requestID, "object": "chat.completion.chunk",
			"choices": []map[string]any{{"index": 0, "delta": map[string]any{"role": "assistant", "content": text}}},
		}
	case "message_stop":
		return map[string]any{
			"id": requestID, "object": "chat.completion.chunk",
			"choices": []map[string]any{{"index": 0, "delta": map[string]any{}, "finish_reason": "stop"}},
		}
	}
	return nil
}
