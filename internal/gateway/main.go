package main

import (
	"log"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	store := buildKeyStore()
	chain := buildChain(store)

	r := gin.Default()
	r.Use(RequestIDMiddleware())
	r.Use(CORSMiddleware())
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	r.Use(AuthMiddleware(store))
	r.POST("/v1/chat/completions", chain.ChatCompletions)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("BridgeAI gateway listening on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}

// buildKeyStore loads BRIDGE_API_KEYS (comma-separated) from env.
func buildKeyStore() *KeyStore {
	raw := os.Getenv("BRIDGE_API_KEYS")
	if raw == "" {
		log.Fatal("BRIDGE_API_KEYS is not set. Add at least one key to .env")
	}
	ks := NewKeyStore(raw)
	log.Printf("key store loaded: %d key(s)", len(ks.keys))
	return ks
}

// buildChain reads PROVIDER_CHAIN (e.g. "groq,anthropic") and builds the fallback chain.
// Limits are read from the KeyStore: keys with a ":<limit>" suffix get a monthly USD budget.
// Falls back to any single configured key if PROVIDER_CHAIN is not set.
func buildChain(store *KeyStore) *Chain {
	var providers []Provider

	chainEnv := os.Getenv("PROVIDER_CHAIN")
	if chainEnv == "" {
		// Auto-detect from available keys.
		if k := os.Getenv("GROQ_API_KEY"); k != "" {
			chainEnv = "groq"
		} else if k := os.Getenv("ANTHROPIC_API_KEY"); k != "" {
			_ = k
			chainEnv = "anthropic"
		} else if k := os.Getenv("GEMINI_API_KEY"); k != "" {
			_ = k
			chainEnv = "gemini"
		} else if k := os.Getenv("OPENROUTER_API_KEY"); k != "" {
			_ = k
			chainEnv = "qwen"
		} else if k := os.Getenv("DEEPSEEK_API_KEY"); k != "" {
			_ = k
			chainEnv = "deepseek"
		}
	}

	if chainEnv == "" {
		log.Fatal("No provider configured. Set PROVIDER_CHAIN and at least one *_API_KEY in .env")
	}

	for _, name := range strings.Split(chainEnv, ",") {
		name = strings.TrimSpace(name)
		switch name {
		case "anthropic":
			key := os.Getenv("ANTHROPIC_API_KEY")
			if key == "" {
				log.Fatalf("PROVIDER_CHAIN includes 'anthropic' but ANTHROPIC_API_KEY is not set")
			}
			providers = append(providers, &AnthropicProvider{apiKey: key})
			log.Printf("provider registered: anthropic")
		case "groq":
			key := os.Getenv("GROQ_API_KEY")
			if key == "" {
				log.Fatalf("PROVIDER_CHAIN includes 'groq' but GROQ_API_KEY is not set")
			}
			providers = append(providers, &GroqProvider{apiKey: key})
			log.Printf("provider registered: groq")
		case "gemini":
			key := os.Getenv("GEMINI_API_KEY")
			if key == "" {
				log.Fatalf("PROVIDER_CHAIN includes 'gemini' but GEMINI_API_KEY is not set")
			}
			providers = append(providers, &GeminiProvider{apiKey: key})
			log.Printf("provider registered: gemini")
		case "qwen":
			key := os.Getenv("OPENROUTER_API_KEY")
			if key == "" {
				log.Fatalf("PROVIDER_CHAIN includes 'qwen' but OPENROUTER_API_KEY is not set")
			}
			providers = append(providers, &QwenProvider{apiKey: key})
			log.Printf("provider registered: qwen")
		case "deepseek":
			key := os.Getenv("DEEPSEEK_API_KEY")
			if key == "" {
				log.Fatalf("PROVIDER_CHAIN includes 'deepseek' but DEEPSEEK_API_KEY is not set")
			}
			providers = append(providers, &DeepSeekProvider{apiKey: key})
			log.Printf("provider registered: deepseek")
		default:
			log.Fatalf("unknown provider in PROVIDER_CHAIN: %q", name)
		}
	}

	return &Chain{providers: providers, budget: NewBudgetStore(store.limits)}
}
