package handler

import (
	"fmt"
	"net/http"

	"claude2api"
	"claude2api/internal/service"

	"github.com/gin-gonic/gin"
	"golang.org/x/mod/semver"
)

func AppVersion(c *gin.Context) {
	var current string
	if _, err := fmt.Sscan(claude2api.Version, &current); err != nil || !semver.IsValid(current) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "当前版本信息无法读取"})
		return
	}
	latest, err := service.LatestVersion()
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"version": current, "latest_version": latest, "update_available": err == nil && semver.Compare(latest, current) > 0, "check_failed": err != nil})
}

func AppChangelog(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(claude2api.Changelog))
}
