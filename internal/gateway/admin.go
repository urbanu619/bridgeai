package main

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// AdminAuthMiddleware checks the ADMIN_TOKEN bearer token.
func AdminAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := os.Getenv("ADMIN_TOKEN")
		if token == "" {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "ADMIN_TOKEN not configured"})
			return
		}
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") || strings.TrimPrefix(header, "Bearer ") != token {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}

type issueKeyRequest struct {
	Email  string  `json:"email" binding:"required,email"`
	Budget float64 `json:"budget"` // monthly USD limit, 0 = unlimited
}

// IssueKeyHandler generates a new key, saves it, and emails it to the applicant.
func IssueKeyHandler(store *KeyStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req issueKeyRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		key, err := store.Issue(req.Email, req.Budget)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate key: " + err.Error()})
			return
		}

		if err := SendKeyEmail(req.Email, key); err != nil {
			// Key is already saved — return it even if email fails
			c.JSON(http.StatusOK, gin.H{
				"key":          key,
				"email":        req.Email,
				"email_status": "failed: " + err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"key":          key,
			"email":        req.Email,
			"email_status": "sent",
		})
	}
}
