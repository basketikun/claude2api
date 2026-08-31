package handler

import (
	"net/http"

	"claude2api/internal/config"
	"claude2api/internal/middleware"
	"claude2api/internal/repository"

	"github.com/gin-gonic/gin"
)

func AdminLogin(c *gin.Context) {
	var body struct {
		Credential string `json:"credential"`
	}
	_ = c.ShouldBindJSON(&body)
	if body.Credential == config.Get().AdminPassword && body.Credential != "" {
		middleware.SetAuthCookie(c, body.Credential)
		c.JSON(http.StatusOK, gin.H{"role": "admin"})
	} else if repository.ValidateAPIKey(body.Credential) {
		middleware.SetAuthCookie(c, body.Credential)
		c.JSON(http.StatusOK, gin.H{"role": "pool"})
	} else {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "密码或密钥错误"})
	}
}

func AdminLogout(c *gin.Context) {
	middleware.ClearAuthCookie(c)
	c.Status(http.StatusNoContent)
}
