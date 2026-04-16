package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// MessagesPassthrough forwards native Anthropic Messages API calls to
// api.anthropic.com unchanged. BridgeAI only layers auth, budget, and
// observability on top — it never parses or mutates the wire protocol body.
type MessagesPassthrough struct {
	apiKey string
	budget *BudgetStore
}

func (mp *MessagesPassthrough) Handle(c *gin.Context) {
	requestID := c.GetString("request_id")

	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "request_id": requestID})
		return
	}

	var peek struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
	}
	_ = json.Unmarshal(raw, &peek)

	bridgeKey := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	if bridgeKey == "" {
		bridgeKey = c.GetHeader("x-api-key")
	}

	byok := c.GetHeader("X-Anthropic-Key")
	if byok == "" && mp.budget != nil && !mp.budget.Check(bridgeKey) {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error":      "budget_exceeded",
			"retryable":  false,
			"request_id": requestID,
		})
		return
	}

	upstreamKey := mp.apiKey
	if byok != "" {
		upstreamKey = byok
	}
	if upstreamKey == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":      "no Anthropic key configured",
			"request_id": requestID,
		})
		return
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost,
		"https://api.anthropic.com/v1/messages", bytes.NewReader(raw))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "request_id": requestID})
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", upstreamKey)
	if v := c.GetHeader("anthropic-version"); v != "" {
		req.Header.Set("anthropic-version", v)
	} else {
		req.Header.Set("anthropic-version", "2023-06-01")
	}
	if v := c.GetHeader("anthropic-beta"); v != "" {
		req.Header.Set("anthropic-beta", v)
	}
	req.Header.Set("X-Request-ID", requestID)
	if peek.Stream {
		req.Header.Set("Accept", "text/event-stream")
	}

	resp, err := sharedClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error(), "request_id": requestID})
		return
	}
	defer resp.Body.Close()

	c.Header("X-Request-ID", requestID)
	c.Header("X-BridgeAI-Request-Id", requestID)
	c.Header("X-BridgeAI-Provider", "claude")

	billingKey := bridgeKey
	if byok != "" {
		billingKey = ""
	}

	if peek.Stream && resp.StatusCode == http.StatusOK {
		mp.pipeStream(c, resp, billingKey, peek.Model)
		return
	}

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusOK && billingKey != "" && mp.budget != nil {
		var r struct {
			Model string `json:"model"`
			Usage struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal(body, &r) == nil {
			model := r.Model
			if model == "" {
				model = peek.Model
			}
			mp.budget.Add(billingKey, model, r.Usage.InputTokens, r.Usage.OutputTokens)
		}
	}

	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "application/json"
	}
	c.Data(resp.StatusCode, ct, body)
}

// pipeStream copies the SSE stream byte-faithfully while sniffing message_start
// and message_delta events to record usage. Anthropic frames events as
// "event: ...\ndata: ...\n\n" — the blank separator line comes through as an
// empty Scanner token and is re-emitted as a bare "\n" to preserve framing.
func (mp *MessagesPassthrough) pipeStream(c *gin.Context, resp *http.Response, billingKey, fallbackModel string) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")

	flusher, _ := c.Writer.(http.Flusher)

	var model string
	var inputTokens, outputTokens int

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		c.Writer.Write(line)
		c.Writer.Write([]byte{'\n'})
		if flusher != nil {
			flusher.Flush()
		}
		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}
		payload := bytes.TrimPrefix(line, []byte("data: "))
		var ev struct {
			Type    string `json:"type"`
			Message struct {
				Model string `json:"model"`
				Usage struct {
					InputTokens  int `json:"input_tokens"`
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			} `json:"message"`
			Usage struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal(payload, &ev) != nil {
			continue
		}
		switch ev.Type {
		case "message_start":
			if ev.Message.Model != "" {
				model = ev.Message.Model
			}
			if ev.Message.Usage.InputTokens > 0 {
				inputTokens = ev.Message.Usage.InputTokens
			}
		case "message_delta":
			if ev.Usage.OutputTokens > 0 {
				outputTokens = ev.Usage.OutputTokens
			}
		}
	}

	if billingKey != "" && mp.budget != nil {
		if model == "" {
			model = fallbackModel
		}
		mp.budget.Add(billingKey, model, inputTokens, outputTokens)
	}
}
