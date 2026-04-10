package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Provider is the interface every upstream backend must implement.
type Provider interface {
	Type() string
	// Send transforms the OpenAI-format body, calls upstream, returns the raw response.
	// If byokKey is non-empty it overrides the platform API key (BYOK mode).
	Send(ctx context.Context, body map[string]any, requestID string, byokKey string) (*http.Response, error)
	// NeedsTransform reports whether this provider returns Anthropic-format responses
	// that must be converted to OpenAI format before sending to the client.
	NeedsTransform() bool
}

// ---------- shared HTTP client ----------

var sharedClient = &http.Client{}

// ---------- Anthropic ----------

type AnthropicProvider struct {
	apiKey string
}

var anthropicModelMap = map[string]string{
	"smart-quality":            "claude-3-5-sonnet-latest",
	"smart-fast":               "claude-3-5-haiku-latest",
	"claude-3-5-sonnet-latest": "claude-3-5-sonnet-latest",
	"claude-3-5-haiku-latest":  "claude-3-5-haiku-latest",
}

func (p *AnthropicProvider) Type() string { return "claude" }
func (p *AnthropicProvider) NeedsTransform() bool { return true }

func (p *AnthropicProvider) Send(ctx context.Context, body map[string]any, requestID string, byokKey string) (*http.Response, error) {
	b := copyBody(body)
	if m, ok := b["model"].(string); ok {
		if resolved, ok := anthropicModelMap[m]; ok {
			b["model"] = resolved
		}
	} else {
		b["model"] = "claude-3-5-haiku-latest"
	}

	anthropicBody, err := toAnthropicRequest(b)
	if err != nil {
		return nil, err
	}

	raw, _ := json.Marshal(anthropicBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.anthropic.com/v1/messages", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	apiKey := p.apiKey
	if byokKey != "" {
		apiKey = byokKey
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("X-Request-ID", requestID)
	if stream, _ := body["stream"].(bool); stream {
		req.Header.Set("Accept", "text/event-stream")
	}

	return sharedClient.Do(req)
}

// toAnthropicRequest converts OpenAI chat completions body → Anthropic Messages body.
func toAnthropicRequest(body map[string]any) (map[string]any, error) {
	result := map[string]any{}
	result["model"] = body["model"]
	if v, ok := body["max_tokens"]; ok {
		result["max_tokens"] = v
	} else {
		result["max_tokens"] = 4096
	}
	if v, ok := body["temperature"]; ok {
		result["temperature"] = v
	}
	if v, ok := body["stream"]; ok {
		result["stream"] = v
	}

	messages, ok := body["messages"].([]any)
	if !ok {
		return nil, fmt.Errorf("messages field missing or invalid")
	}

	var systemContent string
	var userMessages []map[string]any
	for _, m := range messages {
		msg, ok := m.(map[string]any)
		if !ok {
			continue
		}
		if msg["role"] == "system" {
			systemContent, _ = msg["content"].(string)
		} else {
			userMessages = append(userMessages, msg)
		}
	}
	if systemContent != "" {
		result["system"] = systemContent
	}
	result["messages"] = userMessages
	return result, nil
}

// ---------- Groq (OpenAI-compatible) ----------

type GroqProvider struct {
	apiKey string
}

var groqModelMap = map[string]string{
	"smart-quality":           "llama-3.3-70b-versatile",
	"smart-fast":              "llama-3.1-8b-instant",
	"llama-3.3-70b-versatile": "llama-3.3-70b-versatile",
	"llama-3.1-8b-instant":   "llama-3.1-8b-instant",
}

func (p *GroqProvider) Type() string         { return "groq" }
func (p *GroqProvider) NeedsTransform() bool { return false }

func (p *GroqProvider) Send(ctx context.Context, body map[string]any, requestID string, byokKey string) (*http.Response, error) {
	b := copyBody(body)
	if m, ok := b["model"].(string); ok {
		if resolved, ok := groqModelMap[m]; ok {
			b["model"] = resolved
		}
	} else {
		b["model"] = "llama-3.1-8b-instant"
	}
	apiKey := p.apiKey
	if byokKey != "" {
		apiKey = byokKey
	}
	return sendOpenAICompatible(ctx, "https://api.groq.com/openai/v1", apiKey, b, requestID)
}

// ---------- Gemini (OpenAI-compatible) ----------

type GeminiProvider struct {
	apiKey string
}

var geminiModelMap = map[string]string{
	"smart-quality":    "gemini-2.0-flash",
	"smart-fast":       "gemini-2.0-flash",
	"gemini-2.0-flash": "gemini-2.0-flash",
	"gemini-1.5-pro":   "gemini-1.5-pro",
}

func (p *GeminiProvider) Type() string         { return "gemini" }
func (p *GeminiProvider) NeedsTransform() bool { return false }

func (p *GeminiProvider) Send(ctx context.Context, body map[string]any, requestID string, byokKey string) (*http.Response, error) {
	b := copyBody(body)
	if m, ok := b["model"].(string); ok {
		if resolved, ok := geminiModelMap[m]; ok {
			b["model"] = resolved
		}
	} else {
		b["model"] = "gemini-2.0-flash"
	}
	apiKey := p.apiKey
	if byokKey != "" {
		apiKey = byokKey
	}
	return sendOpenAICompatible(ctx, "https://generativelanguage.googleapis.com/v1beta/openai", apiKey, b, requestID)
}

// ---------- Qwen (via OpenRouter, OpenAI-compatible) ----------

type QwenProvider struct {
	apiKey string
}

var qwenModelMap = map[string]string{
	"smart-quality":             "qwen/qwen3-235b-a22b",
	"smart-fast":                "qwen/qwen3-30b-a3b",
	"qwen3-235b":                "qwen/qwen3-235b-a22b",
	"qwen3-30b":                 "qwen/qwen3-30b-a3b",
	"qwen/qwen3-235b-a22b":     "qwen/qwen3-235b-a22b",
	"qwen/qwen3-30b-a3b":       "qwen/qwen3-30b-a3b",
}

func (p *QwenProvider) Type() string         { return "qwen" }
func (p *QwenProvider) NeedsTransform() bool { return false }

func (p *QwenProvider) Send(ctx context.Context, body map[string]any, requestID string, byokKey string) (*http.Response, error) {
	b := copyBody(body)
	if m, ok := b["model"].(string); ok {
		if resolved, ok := qwenModelMap[m]; ok {
			b["model"] = resolved
		}
	} else {
		b["model"] = "qwen/qwen3-30b-a3b"
	}
	apiKey := p.apiKey
	if byokKey != "" {
		apiKey = byokKey
	}
	return sendOpenAICompatible(ctx, "https://openrouter.ai/api/v1", apiKey, b, requestID)
}

// ---------- DeepSeek (OpenAI-compatible) ----------

type DeepSeekProvider struct {
	apiKey string
}

var deepseekModelMap = map[string]string{
	"smart-quality":        "deepseek-chat",
	"smart-fast":           "deepseek-chat",
	"deepseek-chat":        "deepseek-chat",
	"deepseek-reasoner":    "deepseek-reasoner",
}

func (p *DeepSeekProvider) Type() string         { return "deepseek" }
func (p *DeepSeekProvider) NeedsTransform() bool { return false }

func (p *DeepSeekProvider) Send(ctx context.Context, body map[string]any, requestID string, byokKey string) (*http.Response, error) {
	b := copyBody(body)
	if m, ok := b["model"].(string); ok {
		if resolved, ok := deepseekModelMap[m]; ok {
			b["model"] = resolved
		}
	} else {
		b["model"] = "deepseek-chat"
	}
	apiKey := p.apiKey
	if byokKey != "" {
		apiKey = byokKey
	}
	return sendOpenAICompatible(ctx, "https://api.deepseek.com/v1", apiKey, b, requestID)
}

// ---------- OpenAI ----------

type OpenAIProvider struct {
	apiKey string
}

var openaiModelMap = map[string]string{
	"smart-quality": "gpt-4o",
	"smart-fast":    "gpt-4o-mini",
	"gpt-4o":        "gpt-4o",
	"gpt-4o-mini":   "gpt-4o-mini",
	"o3":            "o3",
	"o4-mini":       "o4-mini",
}

func (p *OpenAIProvider) Type() string         { return "openai" }
func (p *OpenAIProvider) NeedsTransform() bool { return false }

// openaiReasoningModels do not support streaming.
var openaiReasoningModels = map[string]bool{"o3": true, "o4-mini": true}

func (p *OpenAIProvider) Send(ctx context.Context, body map[string]any, requestID string, byokKey string) (*http.Response, error) {
	b := copyBody(body)
	model := "gpt-4o-mini"
	if m, ok := b["model"].(string); ok {
		if resolved, ok := openaiModelMap[m]; ok {
			model = resolved
		} else {
			model = m
		}
	}
	b["model"] = model
	// Reasoning models do not support streaming.
	if openaiReasoningModels[model] {
		delete(b, "stream")
	}
	apiKey := p.apiKey
	if byokKey != "" {
		apiKey = byokKey
	}
	return sendOpenAICompatible(ctx, "https://api.openai.com/v1", apiKey, b, requestID)
}

// ---------- Mistral ----------

type MistralProvider struct {
	apiKey string
}

var mistralModelMap = map[string]string{
	"smart-quality":    "mistral-large-latest",
	"smart-fast":       "mistral-small-latest",
	"mistral-large":    "mistral-large-latest",
	"mistral-small":    "mistral-small-latest",
}

func (p *MistralProvider) Type() string         { return "mistral" }
func (p *MistralProvider) NeedsTransform() bool { return false }

func (p *MistralProvider) Send(ctx context.Context, body map[string]any, requestID string, byokKey string) (*http.Response, error) {
	b := copyBody(body)
	if m, ok := b["model"].(string); ok {
		if resolved, ok := mistralModelMap[m]; ok {
			b["model"] = resolved
		}
	} else {
		b["model"] = "mistral-small-latest"
	}
	apiKey := p.apiKey
	if byokKey != "" {
		apiKey = byokKey
	}
	return sendOpenAICompatible(ctx, "https://api.mistral.ai/v1", apiKey, b, requestID)
}

// ---------- Grok (xAI, OpenAI-compatible) ----------

type GrokProvider struct {
	apiKey string
}

var grokModelMap = map[string]string{
	"smart-quality": "grok-3",
	"smart-fast":    "grok-3-mini",
	"grok-3":        "grok-3",
	"grok-3-mini":   "grok-3-mini",
}

func (p *GrokProvider) Type() string         { return "grok" }
func (p *GrokProvider) NeedsTransform() bool { return false }

func (p *GrokProvider) Send(ctx context.Context, body map[string]any, requestID string, byokKey string) (*http.Response, error) {
	b := copyBody(body)
	if m, ok := b["model"].(string); ok {
		if resolved, ok := grokModelMap[m]; ok {
			b["model"] = resolved
		}
	} else {
		b["model"] = "grok-3-fast"
	}
	apiKey := p.apiKey
	if byokKey != "" {
		apiKey = byokKey
	}
	return sendOpenAICompatible(ctx, "https://api.x.ai/v1", apiKey, b, requestID)
}

// ---------- Zhipu AI / GLM (OpenAI-compatible) ----------

type ZhipuProvider struct {
	apiKey string
}

var zhipuModelMap = map[string]string{
	"smart-quality": "glm-4-plus",
	"smart-fast":    "glm-4-flash",
	"glm-4-plus":    "glm-4-plus",
	"glm-4-flash":   "glm-4-flash",
	"glm-4":         "glm-4",
}

func (p *ZhipuProvider) Type() string         { return "zhipu" }
func (p *ZhipuProvider) NeedsTransform() bool { return false }

func (p *ZhipuProvider) Send(ctx context.Context, body map[string]any, requestID string, byokKey string) (*http.Response, error) {
	b := copyBody(body)
	if m, ok := b["model"].(string); ok {
		if resolved, ok := zhipuModelMap[m]; ok {
			b["model"] = resolved
		}
	} else {
		b["model"] = "glm-4-flash"
	}
	apiKey := p.apiKey
	if byokKey != "" {
		apiKey = byokKey
	}
	return sendOpenAICompatible(ctx, "https://open.bigmodel.cn/api/paas/v4", apiKey, b, requestID)
}

// ---------- helpers ----------

// sendOpenAICompatible sends body to any OpenAI-compatible base URL.
func sendOpenAICompatible(ctx context.Context, baseURL, apiKey string, body map[string]any, requestID string) (*http.Response, error) {
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		baseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("X-Request-ID", requestID)
	return sharedClient.Do(req)
}

// copyBody shallow-copies a map so providers don't mutate the original.
func copyBody(body map[string]any) map[string]any {
	out := make(map[string]any, len(body))
	for k, v := range body {
		out[k] = v
	}
	return out
}
