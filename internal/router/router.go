package router

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"claude2api/internal/adapter"
	"claude2api/internal/config"
	"claude2api/internal/handler"
	"claude2api/internal/middleware"
	"claude2api/web"

	"github.com/gin-gonic/gin"
)

// NewMainEngine 构造主站引擎。
func NewMainEngine() *gin.Engine {
	r := gin.Default()

	admin := r.Group("/api")
	admin.Use(middleware.AdminAuth())
	admin.POST("/login", handler.AdminLogin)
	admin.POST("/logout", handler.AdminLogout)

	admin.GET("/accounts", handler.AdminAccounts)
	admin.POST("/accounts/import", handler.AdminImportAccounts)
	admin.POST("/accounts/refresh", handler.AdminRefreshAccount)
	admin.GET("/config", handler.AdminGetConfig)
	admin.POST("/config", handler.AdminUpdateConfig)
	admin.POST("/delete", handler.AdminDeleteAccount)
	admin.POST("/delete-expired", handler.AdminDeleteExpired)

	admin.GET("/keys", handler.AdminListKeys)
	admin.POST("/keys", handler.AdminCreateKey)
	admin.POST("/keys/delete", handler.AdminDeleteKey)

	admin.GET("/logs", handler.AdminListLogs)
	admin.GET("/logs/:id", handler.AdminGetLog)
	admin.POST("/logs/delete", handler.AdminDeleteLogs)

	pool := r.Group("/pool")
	pool.Use(middleware.PoolAuth())
	pool.GET("/api/list", handler.PoolList)
	pool.GET("/api/select", handler.PoolSelect)

	// 2api 必须早于 NoRoute 反代。
	api := r.Group("/v1")
	api.Use(middleware.APIKey())
	api.Use(func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, adapter.MaxAPIBodyBytes)
		c.Next()
	})
	api.GET("/models", adapter.ListModels)
	api.POST("/chat/completions", adapter.OpenAIChat)
	api.POST("/responses", adapter.OpenAIResponses)
	api.POST("/messages", adapter.AnthropicMessages)

	r.StaticFS("/static", http.FS(web.FS))
	r.GET("/", page("index.html"))

	r.NoRoute(handler.ServeMainProxy)
	return r
}

func newSinglePortHandler() http.Handler {
	mainEngine := NewMainEngine()
	ucEngine := gin.Default()
	ucEngine.NoRoute(handler.ServeUCProxy)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if handler.IsUCHost(r.Host) {
			ucEngine.ServeHTTP(w, r)
			return
		}
		mainEngine.ServeHTTP(w, r)
	})
}

func page(name string) gin.HandlerFunc {
	data, err := web.FS.ReadFile(name)
	if err != nil {
		panic(err)
	}
	return func(c *gin.Context) { c.Data(http.StatusOK, "text/html; charset=utf-8", data) }
}

// RunServer 启动 Web 服务。
func RunServer() {
	s := config.Get()
	gin.SetMode(gin.ReleaseMode)

	addr := fmt.Sprintf("%s:%d", s.WebHost, s.WebPort)
	fmt.Printf("[web] 镜像 + 管理已启动: http://%s\n", addr)
	fmt.Printf("[web]   管理与号池入口:        http://%s/\n", addr)
	fmt.Printf("[web]   镜像站(claude.ai):     选择账号后进入\n")
	fmt.Printf("[web]   artifact 沙箱与主站共用端口\n")
	fmt.Println("[web] 按 Ctrl+C 停止")
	server := http.Server{Addr: addr, Handler: newSinglePortHandler(), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 2 * time.Minute}
	if err := server.ListenAndServe(); err != nil {
		slog.Error("[web] 主站启动失败", "err", err)
	}
}
