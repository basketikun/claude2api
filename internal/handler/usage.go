package handler

import (
	"log/slog"
	"net/http"

	"claude2api/internal/repository"

	"github.com/gin-gonic/gin"
)

func AdminUsage(c *gin.Context) {
	var query struct {
		Days           int    `form:"days" binding:"gte=0"`
		TimezoneOffset int    `form:"timezone_offset"`
		Account        string `form:"account"`
		Group          string `form:"group" binding:"required,oneof=account model"`
		Offset         int    `form:"offset" binding:"gte=0"`
		Limit          int    `form:"limit" binding:"required,gt=0"`
	}
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择统计范围和分组"})
		return
	}
	usage, err := repository.LoadUsage(query.Days, query.TimezoneOffset, query.Account, query.Group, query.Offset, query.Limit)
	if err != nil {
		slog.Error("[用量统计] 查询失败", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "用量读取失败，请稍后重试"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"usage": usage, "token_source": "estimated", "current_token_method": "cl100k_base", "cache_tokens": nil})
}
