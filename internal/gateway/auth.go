package main

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware validates the Bearer token against the configured key set.
// Keys are loaded once at startup from the keyStore.
func AuthMiddleware(store *KeyStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":      "missing or malformed Authorization header",
				"request_id": c.GetString("request_id"),
			})
			return
		}

		token := strings.TrimPrefix(header, "Bearer ")
		if !store.Valid(token) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":      "invalid API key",
				"request_id": c.GetString("request_id"),
			})
			return
		}

		c.Next()
	}
}

// KeyStore holds the set of valid BridgeAI API keys and their optional budget limits.
type KeyStore struct {
	keys   map[string]struct{}
	limits map[string]float64 // key → monthly USD limit (0 = unlimited)
}

// NewKeyStore parses a comma-separated list of "key" or "key:limit" entries.
// Example: "bridge-key-001:5.00,bridge-key-002"
func NewKeyStore(raw string) *KeyStore {
	ks := &KeyStore{
		keys:   make(map[string]struct{}),
		limits: make(map[string]float64),
	}
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, ":", 2)
		key := parts[0]
		ks.keys[key] = struct{}{}
		if len(parts) == 2 {
			if limit, err := strconv.ParseFloat(parts[1], 64); err == nil {
				ks.limits[key] = limit
			}
		}
	}
	return ks
}

func (ks *KeyStore) Valid(key string) bool {
	_, ok := ks.keys[key]
	return ok
}
