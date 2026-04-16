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

	// Admin routes (protected by ADMIN_TOKEN)
	admin := r.Group("/admin", AdminAuthMiddleware())
	admin.POST("/issue-key", IssueKeyHandler(store))
	admin.GET("/keys", ListKeysHandler(store))

	r.Use(AuthMiddleware(store))
	r.POST("/v1/chat/completions", chain.ChatCompletions)

	if anthropicKey := os.Getenv("ANTHROPIC_API_KEY"); anthropicKey != "" {
		mp := &MessagesPassthrough{apiKey: anthropicKey, budget: chain.budget}
		r.POST("/v1/messages", mp.Handle)
		log.Printf("anthropic passthrough registered: POST /v1/messages")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("BridgeAI gateway listening on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}

// buildKeyStore loads BRIDGE_API_KEYS (comma-separated) from env, then loads issued keys from file.
func buildKeyStore() *KeyStore {
	raw := os.Getenv("BRIDGE_API_KEYS")
	ks := NewKeyStore(raw)
	if err := ks.LoadFromFile(); err != nil {
		log.Printf("warning: could not load keys.json: %v", err)
	}
	log.Printf("key store loaded: %d key(s)", len(ks.keys))
	return ks
}

// buildChain reads PROVIDER_CHAIN (e.g. "groq,claude") and builds the fallback chain.
// Limits are read from the KeyStore: keys with a ":<limit>" suffix get a monthly USD budget.
// Falls back to any single configured key if PROVIDER_CHAIN is not set.
func buildChain(store *KeyStore) *Chain {
	var providers []Provider

	chainEnv := os.Getenv("PROVIDER_CHAIN")
	if chainEnv == "" {
		// Auto-detect from available keys.
		if k := os.Getenv("GROQ_API_KEY"); k != "" {
			_ = k
			chainEnv = "groq"
		} else if k := os.Getenv("ANTHROPIC_API_KEY"); k != "" {
			_ = k
			chainEnv = "claude"
		} else if k := os.Getenv("GEMINI_API_KEY"); k != "" {
			_ = k
			chainEnv = "gemini"
		} else if k := os.Getenv("DEEPSEEK_API_KEY"); k != "" {
			_ = k
			chainEnv = "deepseek"
		} else if k := os.Getenv("OPENAI_API_KEY"); k != "" {
			_ = k
			chainEnv = "openai"
		} else if k := os.Getenv("MISTRAL_API_KEY"); k != "" {
			_ = k
			chainEnv = "mistral"
		} else if k := os.Getenv("XAI_API_KEY"); k != "" {
			_ = k
			chainEnv = "grok"
		} else if k := os.Getenv("OPENROUTER_API_KEY"); k != "" {
			_ = k
			chainEnv = "qwen"
		} else if k := os.Getenv("ZHIPU_API_KEY"); k != "" {
			_ = k
			chainEnv = "zhipu"
		} else if k := os.Getenv("MOONSHOT_API_KEY"); k != "" {
			_ = k
			chainEnv = "kimi"
		} else if k := os.Getenv("MINIMAX_API_KEY"); k != "" {
			_ = k
			chainEnv = "minimax"
		} else if k := os.Getenv("ARK_API_KEY"); k != "" {
			_ = k
			chainEnv = "doubao"
		} else if k := os.Getenv("MIMO_API_KEY"); k != "" {
			_ = k
			chainEnv = "mimo"
		}
	}

	if chainEnv == "" {
		log.Fatal("No provider configured. Set PROVIDER_CHAIN and at least one *_API_KEY in .env")
	}

	for _, name := range strings.Split(chainEnv, ",") {
		name = strings.TrimSpace(name)
		switch name {
		case "claude", "anthropic": // "anthropic" kept for backward compatibility
			key := os.Getenv("ANTHROPIC_API_KEY")
			if key == "" {
				log.Fatalf("PROVIDER_CHAIN includes %q but ANTHROPIC_API_KEY is not set", name)
			}
			providers = append(providers, &AnthropicProvider{apiKey: key})
			log.Printf("provider registered: claude")
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
		case "deepseek":
			key := os.Getenv("DEEPSEEK_API_KEY")
			if key == "" {
				log.Fatalf("PROVIDER_CHAIN includes 'deepseek' but DEEPSEEK_API_KEY is not set")
			}
			providers = append(providers, &DeepSeekProvider{apiKey: key})
			log.Printf("provider registered: deepseek")
		case "qwen":
			key := os.Getenv("OPENROUTER_API_KEY")
			if key == "" {
				log.Fatalf("PROVIDER_CHAIN includes 'qwen' but OPENROUTER_API_KEY is not set")
			}
			providers = append(providers, &QwenProvider{apiKey: key})
			log.Printf("provider registered: qwen")
		case "openai":
			key := os.Getenv("OPENAI_API_KEY")
			if key == "" {
				log.Fatalf("PROVIDER_CHAIN includes 'openai' but OPENAI_API_KEY is not set")
			}
			providers = append(providers, &OpenAIProvider{apiKey: key})
			log.Printf("provider registered: openai")
		case "mistral":
			key := os.Getenv("MISTRAL_API_KEY")
			if key == "" {
				log.Fatalf("PROVIDER_CHAIN includes 'mistral' but MISTRAL_API_KEY is not set")
			}
			providers = append(providers, &MistralProvider{apiKey: key})
			log.Printf("provider registered: mistral")
		case "grok":
			key := os.Getenv("XAI_API_KEY")
			if key == "" {
				log.Fatalf("PROVIDER_CHAIN includes 'grok' but XAI_API_KEY is not set")
			}
			providers = append(providers, &GrokProvider{apiKey: key})
			log.Printf("provider registered: grok")
		case "zhipu":
			key := os.Getenv("ZHIPU_API_KEY")
			if key == "" {
				log.Fatalf("PROVIDER_CHAIN includes 'zhipu' but ZHIPU_API_KEY is not set")
			}
			providers = append(providers, &ZhipuProvider{apiKey: key})
			log.Printf("provider registered: zhipu")
		case "kimi":
			key := os.Getenv("MOONSHOT_API_KEY")
			if key == "" {
				log.Fatalf("PROVIDER_CHAIN includes 'kimi' but MOONSHOT_API_KEY is not set")
			}
			providers = append(providers, &KimiProvider{apiKey: key})
			log.Printf("provider registered: kimi")
		case "minimax":
			key := os.Getenv("MINIMAX_API_KEY")
			if key == "" {
				log.Fatalf("PROVIDER_CHAIN includes 'minimax' but MINIMAX_API_KEY is not set")
			}
			providers = append(providers, &MiniMaxProvider{apiKey: key})
			log.Printf("provider registered: minimax")
		case "doubao":
			key := os.Getenv("ARK_API_KEY")
			if key == "" {
				log.Fatalf("PROVIDER_CHAIN includes 'doubao' but ARK_API_KEY is not set")
			}
			providers = append(providers, &DoubaoProvider{apiKey: key})
			log.Printf("provider registered: doubao")
		case "mimo":
			key := os.Getenv("MIMO_API_KEY")
			if key == "" {
				log.Fatalf("PROVIDER_CHAIN includes 'mimo' but MIMO_API_KEY is not set")
			}
			providers = append(providers, &MiMoProvider{apiKey: key})
			log.Printf("provider registered: mimo")
		default:
			log.Fatalf("unknown provider in PROVIDER_CHAIN: %q", name)
		}
	}

	return &Chain{providers: providers, budget: NewBudgetStore(store.limits)}
}
