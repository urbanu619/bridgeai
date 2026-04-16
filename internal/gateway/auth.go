package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware validates the Bridge Key against the configured key set.
// Accepts either "Authorization: Bearer <key>" (OpenAI-compatible clients) or
// "x-api-key: <key>" (Anthropic-native clients such as Claude Code).
func AuthMiddleware(store *KeyStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var token string
		if h := c.GetHeader("Authorization"); strings.HasPrefix(h, "Bearer ") {
			token = strings.TrimPrefix(h, "Bearer ")
		} else {
			token = c.GetHeader("x-api-key")
		}

		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":      "missing Authorization or x-api-key header",
				"request_id": c.GetString("request_id"),
			})
			return
		}
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

// IssuedKey represents a dynamically issued key stored on disk.
type IssuedKey struct {
	Key       string    `json:"key"`
	Email     string    `json:"email"`
	Budget    float64   `json:"budget"`
	CreatedAt time.Time `json:"created_at"`
}

// KeyStore holds the set of valid BridgeAI API keys and their optional budget limits.
type KeyStore struct {
	mu     sync.RWMutex
	keys   map[string]struct{}
	limits map[string]float64 // key → monthly USD limit (0 = unlimited)
	file   string             // path to issued keys JSON file
}

// NewKeyStore parses a comma-separated list of "key" or "key:limit" entries.
// Example: "bridge-key-001:5.00,bridge-key-002"
func NewKeyStore(raw string) *KeyStore {
	keysFile := os.Getenv("KEYS_FILE")
	if keysFile == "" {
		keysFile = "/data/keys.json"
	}
	ks := &KeyStore{
		keys:   make(map[string]struct{}),
		limits: make(map[string]float64),
		file:   keysFile,
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

// LoadFromFile loads previously issued keys from keys.json into the store.
func (ks *KeyStore) LoadFromFile() error {
	data, err := os.ReadFile(ks.file)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var issued []IssuedKey
	if err := json.Unmarshal(data, &issued); err != nil {
		return err
	}
	ks.mu.Lock()
	defer ks.mu.Unlock()
	for _, k := range issued {
		ks.keys[k.Key] = struct{}{}
		if k.Budget > 0 {
			ks.limits[k.Key] = k.Budget
		}
	}
	return nil
}

// Issue generates a new key for the given email, persists it to disk, and returns it.
func (ks *KeyStore) Issue(email string, budget float64) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	key := "bridge-" + hex.EncodeToString(b)

	entry := IssuedKey{Key: key, Email: email, Budget: budget, CreatedAt: time.Now().UTC()}

	ks.mu.Lock()
	ks.keys[key] = struct{}{}
	if budget > 0 {
		ks.limits[key] = budget
	}
	ks.mu.Unlock()

	if err := ks.appendToFile(entry); err != nil {
		return "", err
	}
	return key, nil
}

// ListIssued returns all dynamically issued keys from the file.
func (ks *KeyStore) ListIssued() ([]IssuedKey, error) {
	data, err := os.ReadFile(ks.file)
	if os.IsNotExist(err) {
		return []IssuedKey{}, nil
	}
	if err != nil {
		return nil, err
	}
	var issued []IssuedKey
	if err := json.Unmarshal(data, &issued); err != nil {
		return nil, err
	}
	return issued, nil
}

func (ks *KeyStore) appendToFile(entry IssuedKey) error {
	var issued []IssuedKey
	data, err := os.ReadFile(ks.file)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &issued); err != nil {
			return err
		}
	}
	issued = append(issued, entry)
	out, err := json.MarshalIndent(issued, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ks.file, out, 0600)
}

func (ks *KeyStore) Valid(key string) bool {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	_, ok := ks.keys[key]
	return ok
}
