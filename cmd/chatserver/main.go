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
	"live-chatter/internal/controller"
	"live-chatter/internal/repository"
	"live-chatter/internal/server"
	"live-chatter/internal/service"
	"live-chatter/pkg"
	"live-chatter/pkg/db"
	"live-chatter/pkg/middleware"
	"live-chatter/pkg/model"

	Log "live-chatter/pkg/logger"

	"github.com/gin-gonic/gin"
)

var (
	clientsManager = &pkg.ClientManager{
		Broadcast:   make(chan pkg.BroadcastMessage),
		Register:    make(chan *pkg.Client),
		Unregister:  make(chan *pkg.Client),
		Clients:     make(map[*pkg.Client]bool),
		Rooms:       make(map[string]map[*pkg.Client]bool),
		UserClients: make(map[string]*pkg.Client),
	}
)

func main() {
	Log.SetupLogging("logs", true)
	Log.Info("Gin WebSocket server starting...")

	cfg := loadConfig("config.xml")
	initDatabase(cfg)
	initAuth(cfg)

	// Auto-migrate database models
	if err := autoMigrate(); err != nil {
		Log.Error("Database migration failed: %v", err)
		os.Exit(1)
	}

	go clientsManager.Start()

	r := initRouter(cfg)
	setupRoutes(r)

	runServer(cfg, r)
}

func autoMigrate() error {
	return db.GetDB().AutoMigrate(
		&model.User{},
		&model.Room{},
		&model.Message{},
	)
}

func setupRoutes(router *gin.Engine) {
	// Initialize repositories and services
	userRepo := repository.NewUserRepository()
	roomRepo := repository.NewRoomRepository()
	messageRepo := repository.NewMessageRepository()

	authService := service.NewAuthService(userRepo)
	chatService := service.NewChatService(messageRepo, roomRepo, userRepo)

	authController := controller.NewAuthController(authService)
	chatController := controller.NewChatController(chatService)

	// WebSocket endpoint
	router.GET("/ws", middleware.WebSocketAuthMiddleware(), func(c *gin.Context) {
		server.WebSocket(c.Writer, c.Request, clientsManager)
	})

	// API routes
	api := router.Group("/api/v1")
	{
		// Auth routes
		auth := api.Group("/auth")
		{
			auth.POST("/register", authController.Register)
			auth.POST("/login", authController.Login)
			auth.POST("/refresh", authController.Refresh)
		}

		// Chat routes
		chat := api.Group("/chat")
		chat.Use(middleware.AuthMiddleware())
		{
			chat.GET("/rooms", chatController.GetRooms)
			chat.POST("/rooms", chatController.CreateRoom)
			chat.GET("/rooms/:roomId/messages", chatController.GetRoomMessages)
			chat.POST("/rooms/:roomId/join", chatController.JoinRoom)
			chat.POST("/rooms/:roomId/leave", chatController.LeaveRoom)
			chat.GET("/users/online", chatController.GetOnlineUsers)
		}
	}

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})
}

func runServer(cfg *config.APIConfig, router *gin.Engine) {
	addr := fmt.Sprintf("%s:%d", cfg.Context.Host, cfg.Context.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	Log.Info("Server starting on %s", addr)

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			Log.Error("Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	Log.Info("Shutting down server...")

	// Give outstanding requests 30 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

	router.Use(
		middleware.CORSMiddleware(),
		middleware.RateLimitMiddleware(),
		gin.Logger(),
		gin.Recovery(),
	)

	return router
}

func loadConfig(path string) *config.APIConfig {
	cfg, err := config.LoadConfig(path)
	if err != nil {
		Log.Error("Error loading config:", err)
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
