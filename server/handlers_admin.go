package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// handleAdminKeys handles GET and POST on /v1/admin/keys.
func (s *Server) handleAdminKeys(c *gin.Context) {
	switch c.Request.Method {
	case http.MethodGet:
		keys := s.keyStore.ListKeys()
		c.JSON(200, gin.H{"keys": keys, "count": len(keys)})

	case http.MethodPost:
		var req struct {
			Key string `json:"key"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "invalid json"})
			return
		}
		key := req.Key
		if key == "" {
			key = generateKey()
		}
		ak := s.keyStore.AddKey(key)
		c.JSON(201, gin.H{
			"key":        ak.Key,
			"masked":     ak.Masked,
			"created_at": ak.CreatedAt,
		})

	default:
		c.AbortWithStatus(405)
	}
}

// handleAdminKeyByID handles DELETE on /v1/admin/keys/:id.
func (s *Server) handleAdminKeyByID(c *gin.Context) {
	maskedOrKey := c.Param("id")

	if c.Request.Method != http.MethodDelete {
		c.AbortWithStatus(405)
		return
	}

	// Try exact key match first
	if s.keyStore.DeleteKey(maskedOrKey) {
		c.JSON(200, gin.H{"deleted": maskKey(maskedOrKey)})
		return
	}

	// Try masked match
	for _, k := range s.keyStore.ListKeys() {
		if k.Masked == maskedOrKey {
			s.keyStore.DeleteKey(k.Key)
			c.JSON(200, gin.H{"deleted": maskedOrKey})
			return
		}
	}

	c.JSON(404, gin.H{"error": "key not found"})
}
