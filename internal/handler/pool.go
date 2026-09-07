package handler

import (
	"log/slog"
	"net/http"

	"claude2api/internal/repository"
	"claude2api/internal/service"

	"github.com/gin-gonic/gin"
)

func PoolList(c *gin.Context) {
	current, _ := c.Cookie("pool_acct")
	items := []gin.H{}
	for _, account := range repository.LoadAccounts() {
		if !service.AccountUsable(&account) {
			continue
		}
		status := account.Status
		if status == "" {
			status = "unknown"
		}
		items = append(items, gin.H{"email": account.Email, "status": status})
	}
	c.JSON(http.StatusOK, gin.H{"accounts": items, "current": current})
}

func PoolSelect(c *gin.Context) {
	email := c.Query("email")
	if service.AccountByEmail(email) != nil {
		c.SetCookie("pool_acct", email, 31536000, "/", "", false, false)
		c.SetCookie("mirror", "claude", 31536000, "/", "", false, false)
		slog.Info("[号池] 切换当前账号", "email", email)
	} else {
		c.SetCookie("pool_acct", "", -1, "/", "", false, false)
		slog.Info("[号池] 切换为自动轮询")
	}
	c.Redirect(http.StatusFound, "/new")
}

// PoolRefreshCurrent refreshes and returns the current pool account's session limits
func PoolRefreshCurrent(c *gin.Context) {
	email, err := c.Cookie("pool_acct")
	if err != nil || email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No pool account selected"})
		return
	}
	
	account, removed := service.RefreshAccount(email)
	if removed {
		c.JSON(http.StatusOK, gin.H{"removed": true, "message": "Account was removed"})
		return
	}
	if account == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Account not found"})
		return
	}
	
	slog.Info("[号池] 刷新当前账号会话限制", "email", email)
	c.JSON(http.StatusOK, gin.H{"account": service.PublicAccountView(account)})
}
