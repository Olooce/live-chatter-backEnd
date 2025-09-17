package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"live-chatter/internal/config"
	"live-chatter/internal/server"
	"live-chatter/pkg"
	"live-chatter/pkg/db"
	"live-chatter/pkg/middleware"

	Log "live-chatter/pkg/logger"

	"github.com/gin-gonic/gin"
)

var (
	clientsManager = &pkg.ClientManager{
		Broadcast:  make(chan []byte),
		Register:   make(chan *pkg.Client),
		Unregister: make(chan *pkg.Client),
		Clients:    make(map[*pkg.Client]bool),
	}
)

func main() {
	Log.SetupLogging("logs")
	fmt.Println("Gin WebSocket server starting...")

	cfg := loadConfig("config.xml")
	initDatabase(cfg)
	initAuth(cfg)

	go clientsManager.Start()

	r := initRouter(cfg)
	r.GET("/", func(c *gin.Context) {
		server.WebSocket(c.Writer, c.Request, clientsManager)
	})

	runServer(cfg, r)
}

func runServer(cfg *config.APIConfig, router *gin.Engine) {
	addr := fmt.Sprintf("%s:%d", cfg.Context.Host, cfg.Context.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	// Start server in a goroutine so it doesn't block
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			Log.Error("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	Log.Info("Shutting down server...")

	// Give outstanding requests 5 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		Log.Error("Server forced to shutdown: %v", err)
	}

	Log.Info("Server exiting")
}

func initRouter(cfg *config.APIConfig) *gin.Engine {
	gin.SetMode(cfg.Context.Mode)
	router := gin.Default()
	if err := router.SetTrustedProxies(cfg.Context.TrustedProxies.Proxies); err != nil {
		Log.Error("Failed to set trusted proxies: %v", err)
	}

	router.Use(middleware.CORSMiddleware(), middleware.AuthMiddleware(), middleware.RateLimitMiddleware())
	return router
}

func loadConfig(path string) *config.APIConfig {
	cfg, err := config.LoadConfig(path)
	if err != nil {

		fmt.Println("Error loading config:", err)
		os.Exit(1)
	}
	return cfg
}

func initDatabase(cfg *config.APIConfig) {
	db.InitDBFromConfig(cfg)
}

func initAuth(cfg *config.APIConfig) {
	middleware.InitAuthConfig(cfg)
}
