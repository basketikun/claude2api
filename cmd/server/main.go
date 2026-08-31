package main

import (
	"log/slog"

	"claude2api/internal/config"
	"claude2api/internal/repository"
	"claude2api/internal/router"
	"claude2api/internal/service"
)

func main() {
	config.Load()
	if err := repository.InitDB(); err != nil {
		slog.Error("[启动] 数据库初始化失败", "err", err)
		return
	}
	service.StartAccountStatusMonitor()
	router.RunServer()
}
