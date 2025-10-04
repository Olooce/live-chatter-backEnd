package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
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

	"github.com/common-nighthawk/go-figure"
	"github.com/gin-gonic/gin"
	"golang.org/x/term"
)

func main() {
	printStartUpBanner()

	cfg := loadConfig("config.xml")

	debugMode := cfg.Context.Mode != gin.ReleaseMode
	Log.SetupLogging("logs", debugMode)

	initDatabase(cfg)
	initAuth(cfg)

	// Auto-migrate database models
	if err := autoMigrate(); err != nil {
		Log.Error("Database migration failed: %v", err)
		os.Exit(1)
	}

	userRepo, roomRepo, messageRepo := initializeRepos()

	clientsManager := &pkg.ClientManager{
		Broadcast:   make(chan pkg.BroadcastMessage),
		Register:    make(chan *pkg.Client),
		Unregister:  make(chan *pkg.Client),
		Clients:     make(map[*pkg.Client]bool),
		Rooms:       make(map[string]map[*pkg.Client]bool),
		UserClients: make(map[string]*pkg.Client),
		RoomRepo:    roomRepo,
		MessageRepo: messageRepo,
	}

	go clientsManager.Start()

	r := initRouter(cfg)
	setupRoutes(r, clientsManager, userRepo)

	runServer(cfg, r)
}

func printStartUpBanner() {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width < 60 {
		width = 80
	}

	myFigure := figure.NewFigure("CHATTER", "slant", true)
	lines := strings.Split(myFigure.String(), "\n")

	blue := "\033[34m"
	reset := "\033[0m"

	for _, line := range lines {
		spaces := (width - len(line)) / 2
		if spaces < 0 {
			spaces = 0
		}
		fmt.Printf("%s%s%s\n", strings.Repeat(" ", spaces), blue, line)
	}
	fmt.Print(reset)

	sep := strings.Repeat("=", width)
	fmt.Println(sep)

	banner := fmt.Sprintf("CHATTER SERVER (v%s)", "0.0.0-LiveChatter")
	spaces := (width - len(banner)) / 2
	if spaces < 0 {
		spaces = 0
	}
	fmt.Printf("%s%s\n\n", strings.Repeat(" ", spaces), banner)
}

func initializeRepos() (repository.UserRepository, repository.RoomRepository, repository.MessageRepository) {
	userRepo := repository.NewUserRepository()
	roomRepo := repository.NewRoomRepository()
	messageRepo := repository.NewMessageRepository()
	return userRepo, roomRepo, messageRepo
}

func autoMigrate() error {
	return db.GetDB().AutoMigrate(
		&model.User{},
		&model.Room{},
		&model.Message{},
		&model.UserRoom{},
		&model.PrivateMessage{},
		&model.UserSession{},
		&model.ActivityLog{})
}

func setupRoutes(router *gin.Engine, clientsManager *pkg.ClientManager, userRepo repository.UserRepository) {
	roomRepo := clientsManager.RoomRepo
	messageRepo := clientsManager.MessageRepo

	authService := service.NewAuthService(userRepo)
	chatService := service.NewChatService(messageRepo, roomRepo, userRepo, clientsManager)

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
	Log.FlushLogs()
}

func initRouter(cfg *config.APIConfig) *gin.Engine {
	gin.SetMode(cfg.Context.Mode)
	router := gin.New()

	if err := router.SetTrustedProxies(cfg.Context.TrustedProxies.Proxies); err != nil {
		Log.Error("Failed to set trusted proxies: %v", err)
	}

	middlewares := []gin.HandlerFunc{
		middleware.CORSMiddleware(),
		middleware.RateLimitMiddleware(),
		gin.Recovery(),
	}

	if cfg.RequestDump {
		middlewares = append(middlewares, middleware.RequestDumpMiddleware())
	}

	if cfg.Context.Mode != gin.ReleaseMode {
		middlewares = append(middlewares, gin.Logger())
	}

	router.Use(middlewares...)

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
