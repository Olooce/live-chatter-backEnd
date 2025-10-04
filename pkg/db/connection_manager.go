package db

//WORST CM EVER. ( XD )Interesting waste of time though

import (
	"context"
	"database/sql"
	"fmt"
	"live-chatter/internal/config"
	"log"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	conn      *gorm.DB
	connMutex sync.RWMutex
	debugMode bool
	dbConfig  *config.APIConfig
)

const (
	PingInterval         = 30 * time.Second
	MaxReconnectAttempts = 5
	InitialBackoff       = 1 * time.Second
	MaxBackoff           = 32 * time.Second
	HealthCheckTimeout   = 5 * time.Second
	IdleThresholdForSkip = 5 * time.Minute
)

var lastActivityTime time.Time
var activityMutex sync.RWMutex

func debugLog(context, message string, args ...interface{}) {
	if debugMode {
		timestamp := time.Now().Format("2006-01-02 15:04:05.000")
		prefix := fmt.Sprintf("[DB-DEBUG][%s][%s] ", timestamp, context)
		log.Printf(prefix+message, args...)
	}
}

func InitDBFromConfig(cfg *config.APIConfig) error {
	connMutex.Lock()
	dbConfig = cfg

	debugMode = cfg.Context.Mode != gin.ReleaseMode // TODO: Can we have this to be completely stand alone?
	connMutex.Unlock()
	log.SetFlags(0)
	debugLog("InitDBFromConfig", "Starting database initialization")
	debugLog("InitDBFromConfig", "Debug mode is: %v", debugMode)

	fmt.Println("\nCONNECTION MANAGER - CHATTER_SERVER-CM")
	fmt.Println("-------------[Creating Connection Pool...]-----------")
	fmt.Printf(" Database Server       : %s\n", cfg.DB.Server)
	fmt.Printf(" Database Driver       : %s\n", cfg.DB.Driver)
	fmt.Printf(" Database Name         : %s\n", cfg.DB.Names.LIVECHAT)
	fmt.Printf(" Database Username     : %s\n", cfg.DB.Username)
	fmt.Printf(" Database Port         : %d\n", cfg.DB.Port)
	fmt.Printf(" Debug Mode            : %v\n", debugMode)

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=%s",
		cfg.DB.Host,
		cfg.DB.Username,
		cfg.DB.Password.Value,
		cfg.DB.Names.LIVECHAT,
		cfg.DB.Port,
		cfg.DB.SSLMode,
		cfg.Context.TimeZone,
	)

	debugLog("InitDBFromConfig", "Attempting to open database connection")

	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}

	newConn, err := gorm.Open(postgres.Open(dsn), gormConfig)
	if err != nil {
		debugLog("InitDBFromConfig", "FAILED to open database connection: %v", err)
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	debugLog("InitDBFromConfig", "Successfully opened connection")

	sqlDB, err := newConn.DB()
	if err != nil {
		debugLog("InitDBFromConfig", "FAILED to get underlying sql.DB: %v", err)
		return fmt.Errorf("failed to get database connection: %w", err)
	}

	debugLog("InitDBFromConfig", "Configuring connection pool with MaxOpen=%d, MaxIdle=%d, MaxLifetime=%ds",
		cfg.DB.Pool.MaxOpenConns, cfg.DB.Pool.MaxIdleConns, cfg.DB.Pool.ConnMaxLifetime)

	sqlDB.SetMaxOpenConns(cfg.DB.Pool.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.DB.Pool.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.DB.Pool.ConnMaxLifetime) * time.Second)

	debugLog("InitDBFromConfig", "Connection pool configured successfully")

	//Pre-warm connections
	for i := 0; i < dbConfig.DB.Pool.MaxIdleConns; i++ {
		go func(db *gorm.DB) {
			sqlDB, _ := db.DB()
			_ = sqlDB.Ping()
		}(newConn)
	}

	debugLog("InitDBFromConfig", "Performing initial health check (ping)")
	pingStart := time.Now()
	if err := sqlDB.Ping(); err != nil {
		debugLog("InitDBFromConfig", "Initial ping FAILED after %v: %v", time.Since(pingStart), err)
		return fmt.Errorf("database ping failed: %w", err)
	}
	pingDuration := time.Since(pingStart)
	debugLog("InitDBFromConfig", "Initial ping SUCCESS in %v", pingDuration)

	connMutex.Lock()
	conn = newConn
	lastActivityTime = time.Now()
	connMutex.Unlock()

	debugLog("InitDBFromConfig", "Global connection assigned")

	debugLog("InitDBFromConfig", "Starting background health monitor goroutine")
	go monitorConnectionPool()

	printConnectionPoolStats(sqlDB, cfg)

	fmt.Println("\nConnection Pool Initialized Successfully!")
	debugLog("InitDBFromConfig", "Initialization complete")

	startDebugPoolLogger(5 * time.Minute)

	return nil
}

func monitorConnectionPool() {
	debugLog("monitorConnectionPool", "Health monitor started with interval=%v", PingInterval)
	ticker := time.NewTicker(PingInterval)
	defer ticker.Stop()

	for range ticker.C {
		activityMutex.RLock()
		timeSinceActivity := time.Since(lastActivityTime)
		activityMutex.RUnlock()

		if timeSinceActivity > IdleThresholdForSkip {
			debugLog("monitorConnectionPool", "Skipping health check - DB idle for %v (threshold: %v)",
				timeSinceActivity, IdleThresholdForSkip)
			continue
		}

		debugLog("monitorConnectionPool", "Performing scheduled health check (last activity: %v ago)",
			timeSinceActivity)

		db := GetDB()
		if db == nil {
			debugLog("monitorConnectionPool", "WARNING: Global connection is nil, triggering reconnect")
			ReconnectDB("monitorConnectionPool - nil connection")
			continue
		}

		sqlDB, err := db.DB()
		if err != nil {
			debugLog("monitorConnectionPool", "ERROR: Failed to get sql.DB: %v, triggering reconnect", err)
			ReconnectDB("monitorConnectionPool - failed to get sql.DB")
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), HealthCheckTimeout)
		pingStart := time.Now()
		err = sqlDB.PingContext(ctx)
		pingDuration := time.Since(pingStart)
		cancel()

		if err != nil {
			debugLog("monitorConnectionPool", "Health check FAILED after %v: %v", pingDuration, err)
			log.Printf("Database connection unhealthy: %v\n", err)

			debugLog("monitorConnectionPool", "Closing unhealthy connection before reconnect")
			err := sqlDB.Close()
			if err != nil {
				debugLog("monitorConnectionPool", "Error closing unhealthy connection", err)
				return
			}

			ReconnectDB("monitorConnectionPool - unhealthy connection")
		} else {
			debugLog("monitorConnectionPool", "Health check SUCCESS in %v", pingDuration)

			if debugMode {
				stats := sqlDB.Stats()
				debugLog("monitorConnectionPool", "Pool stats: Open=%d, InUse=%d, Idle=%d",
					stats.OpenConnections, stats.InUse, stats.Idle)
			}
		}
	}
}

func ReconnectDB(callerContext string) {
	debugLog("ReconnectDB", "Reconnection requested by: %s", callerContext)
	log.Printf("Reconnecting database (triggered by: %s)...\n", callerContext)

	connMutex.Lock()
	defer connMutex.Unlock()

	if dbConfig == nil {
		debugLog("ReconnectDB", "ERROR: dbConfig is nil, cannot reconnect")
		log.Println("Failed to reconnect: configuration not available")
		return
	}

	backoff := InitialBackoff
	var lastErr error

	for attempt := 1; attempt <= MaxReconnectAttempts; attempt++ {
		debugLog("ReconnectDB", "Reconnection attempt %d/%d (backoff: %v)",
			attempt, MaxReconnectAttempts, backoff)

		dsn := fmt.Sprintf(
			"host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=%s",
			dbConfig.DB.Host,
			dbConfig.DB.Username,
			dbConfig.DB.Password.Value,
			dbConfig.DB.Names.LIVECHAT,
			dbConfig.DB.Port,
			dbConfig.DB.SSLMode,
			dbConfig.Context.TimeZone,
		)

		debugLog("ReconnectDB", "Opening new connection (attempt %d)", attempt)

		gormConfig := &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		}

		newConn, err := gorm.Open(postgres.Open(dsn), gormConfig)
		if err != nil {
			lastErr = err
			debugLog("ReconnectDB", "Attempt %d FAILED to open connection: %v", attempt, err)
			log.Printf("Reconnection attempt %d failed: %v\n", attempt, err)

			if attempt < MaxReconnectAttempts {
				debugLog("ReconnectDB", "Sleeping for %v before retry", backoff)
				time.Sleep(backoff)

				backoff *= 2
				if backoff > MaxBackoff {
					backoff = MaxBackoff
				}
			}
			continue
		}

		debugLog("ReconnectDB", "Successfully opened new connection on attempt %d", attempt)

		sqlDB, err := newConn.DB()
		if err != nil {
			lastErr = err
			debugLog("ReconnectDB", "Attempt %d FAILED to get sql.DB: %v", attempt, err)
			continue
		}

		debugLog("ReconnectDB", "Reconfiguring connection pool settings")
		sqlDB.SetMaxOpenConns(dbConfig.DB.Pool.MaxOpenConns)
		sqlDB.SetMaxIdleConns(dbConfig.DB.Pool.MaxIdleConns)
		sqlDB.SetConnMaxLifetime(time.Duration(dbConfig.DB.Pool.ConnMaxLifetime) * time.Second)

		debugLog("ReconnectDB", "Testing new connection with ping")
		pingStart := time.Now()
		if err := sqlDB.Ping(); err != nil {
			lastErr = err
			debugLog("ReconnectDB", "Ping FAILED on attempt %d after %v: %v",
				attempt, time.Since(pingStart), err)
			continue
		}
		pingDuration := time.Since(pingStart)
		debugLog("ReconnectDB", "Ping SUCCESS in %v", pingDuration)

		if conn != nil {
			debugLog("ReconnectDB", "Closing old connection")
			if oldDB, err := conn.DB(); err == nil {
				err := oldDB.Close()
				if err != nil {
					log.Printf("Error closing old connection: %v", err)
					return
				}
				debugLog("ReconnectDB", "Old connection closed successfully")
			}
		}

		conn = newConn
		lastActivityTime = time.Now()
		debugLog("ReconnectDB", "New global connection assigned")

		log.Printf("Database reconnected successfully on attempt %d\n", attempt)
		debugLog("ReconnectDB", "Reconnection complete")

		return
	}

	debugLog("ReconnectDB", "CRITICAL: All %d reconnection attempts FAILED. Last error: %v",
		MaxReconnectAttempts, lastErr)
	log.Printf("CRITICAL: Failed to reconnect after %d attempts. Last error: %v\n",
		MaxReconnectAttempts, lastErr)
}

func GetDB() *gorm.DB {
	connMutex.RLock()
	defer connMutex.RUnlock()

	debugLog("GetDB", "Accessing database connection (read-lock acquired)")

	activityMutex.Lock()
	lastActivityTime = time.Now()
	activityMutex.Unlock()

	debugLog("GetDB", "Last activity time updated")

	return conn
}

func PrintPoolStats() {
	if !debugMode {
		return
	}

	debugLog("PrintPoolStats", "Retrieving current pool statistics")

	db := GetDB()
	if db == nil {
		debugLog("PrintPoolStats", "Cannot print stats - connection is nil")
		log.Println("Cannot print pool stats: connection is nil")
		return
	}

	sqlDB, err := db.DB()
	if err != nil {
		debugLog("PrintPoolStats", "Failed to get sql.DB: %v", err)
		log.Printf("Cannot print pool stats: %v\n", err)
		return
	}

	stats := sqlDB.Stats()

	fmt.Println("\n-------------[Real-Time Pool Status]-------------")
	fmt.Printf(" Timestamp                     : %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf(" Open Connections              : %d\n", stats.OpenConnections)
	fmt.Printf(" In Use Connections            : %d\n", stats.InUse)
	fmt.Printf(" Idle Connections              : %d\n", stats.Idle)
	fmt.Printf(" Wait Count                    : %d\n", stats.WaitCount)
	fmt.Printf(" Wait Duration (ms)            : %d\n", stats.WaitDuration.Milliseconds())
	fmt.Printf(" Max Idle Closed               : %d\n", stats.MaxIdleClosed)
	fmt.Printf(" Max Lifetime Closed           : %d\n", stats.MaxLifetimeClosed)
	fmt.Println("------------------------------------------------")

	debugLog("PrintPoolStats", "Stats printed: Open=%d, InUse=%d, Idle=%d",
		stats.OpenConnections, stats.InUse, stats.Idle)
}

func printConnectionPoolStats(db *sql.DB, cfg *config.APIConfig) {
	stats := db.Stats()

	fmt.Println("\n-------------[Connection Pool Details]-------------")
	fmt.Printf(" Max Open Connections          : %d\n", cfg.DB.Pool.MaxOpenConns)
	fmt.Printf(" Max Idle Connections          : %d\n", cfg.DB.Pool.MaxIdleConns)
	fmt.Printf(" Connection Max Lifetime       : %d seconds\n", cfg.DB.Pool.ConnMaxLifetime)
	fmt.Println("\n-------------[Current Pool Status]-------------")
	fmt.Printf(" Open Connections              : %d\n", stats.OpenConnections)
	fmt.Printf(" In Use Connections            : %d\n", stats.InUse)
	fmt.Printf(" Idle Connections              : %d\n", stats.Idle)
	fmt.Printf(" Wait Count                    : %d\n", stats.WaitCount)
	fmt.Printf(" Wait Duration (ms)            : %d\n", stats.WaitDuration.Milliseconds())
	fmt.Printf(" Max Idle Closed               : %d\n", stats.MaxIdleClosed)
	fmt.Printf(" Max Lifetime Closed           : %d\n", stats.MaxLifetimeClosed)
	fmt.Println("\n------------------------------------------------")

	debugLog("printConnectionPoolStats", "Initial stats: Open=%d, InUse=%d, Idle=%d",
		stats.OpenConnections, stats.InUse, stats.Idle)
}

func startDebugPoolLogger(interval time.Duration) {
	if !debugMode {
		return
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			PrintPoolStats()
		}
	}()
}

func SetDebugMode(enabled bool) {
	connMutex.Lock()
	defer connMutex.Unlock()

	oldMode := debugMode
	debugMode = enabled

	debugLog("SetDebugMode", "Debug mode changed from %v to %v", oldMode, enabled)
	log.Printf("Database debug mode: %v\n", enabled)
}

func GetDebugMode() bool {
	connMutex.RLock()
	defer connMutex.RUnlock()
	return debugMode
}

func CloseDB() error {
	debugLog("CloseDB", "Shutdown requested - closing database connection")

	connMutex.Lock()
	defer connMutex.Unlock()

	if conn == nil {
		debugLog("CloseDB", "Connection already nil, nothing to close")
		return nil
	}

	sqlDB, err := conn.DB()
	if err != nil {
		debugLog("CloseDB", "Failed to get sql.DB during shutdown: %v", err)
		return err
	}

	err = sqlDB.Close()
	if err != nil {
		debugLog("CloseDB", "Error closing connection: %v", err)
		return err
	}

	conn = nil
	debugLog("CloseDB", "Connection closed successfully and global variable set to nil")
	log.Println("Database connection closed")

	return nil
}
