package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/NEDA-LABS/stablenode/config"
	"github.com/NEDA-LABS/stablenode/routers"
	"github.com/NEDA-LABS/stablenode/services"
	"github.com/NEDA-LABS/stablenode/storage"
	"github.com/NEDA-LABS/stablenode/tasks"
	"github.com/NEDA-LABS/stablenode/utils/logger"
	"github.com/spf13/viper"
)

func main() {
	// Set timezone with fallback options
	conf := config.ServerConfig()
	loc, err := time.LoadLocation(conf.Timezone)
	if err != nil {
		// Try fallback timezones if the configured one fails
		fallbackTimezones := []string{"UTC", "GMT", "America/New_York"}

		for _, fallback := range fallbackTimezones {
			if fallbackLoc, fallbackErr := time.LoadLocation(fallback); fallbackErr == nil {
				logger.Warnf("Failed to load configured timezone %s, using fallback: %s", conf.Timezone, fallback)
				loc = fallbackLoc
				break
			}
		}

		// If all fallbacks fail, use UTC as last resort
		if loc == nil {
			logger.Errorf("All timezone fallbacks failed, using UTC as last resort. Error: %v", err)
			loc = time.UTC
		}
	}

	time.Local = loc

	// Connect to the database
	DSN := config.DBConfig()
	if err := storage.DBConnection(DSN); err != nil {
		logger.Fatalf("database DBConnection: %s", err)
	}
	defer storage.GetClient().Close()

	// Fix database mishap
	// err := tasks.FixDatabaseMishap()
	// if err != nil {
	// 	logger.Errorf("FixDatabaseMishap: %v", err)
	// }

	// Fetch provider balances
	err = tasks.FetchProviderBalances()
	if err != nil {
		logger.Errorf("Failed to fetch provider balances: %v", err)
	}

	// Initialize Redis
	if err := storage.InitializeRedis(); err != nil {
		log.Println(err)
		logger.Fatalf("Redis initialization: %v", err)
	}

	// Setup gateway webhooks for all EVM networks
	serviceManager := services.NewServiceManager()
	logger.Infof("Using blockchain service: %s", serviceManager.GetActiveService())
	
	// Only create webhooks if using Thirdweb (Alchemy webhooks handled differently)
	if serviceManager.GetActiveService() == "Thirdweb Engine" {
		err = serviceManager.GetEngineService().CreateGatewayWebhook()
		if err != nil {
			logger.Errorf("Failed to create gateway webhooks: %v", err)
		}
	} else {
		logger.Infof("Alchemy service active - webhook setup handled separately")
	}

	// Subscribe to Redis keyspace events
	tasks.SubscribeToRedisKeyspaceEvents()

	// Start cron jobs
	tasks.StartCronJobs()

	// Start polling service if enabled (fallback for webhook failures)
	var pollingService *services.PollingService
	if viper.GetBool("ENABLE_POLLING_FALLBACK") {
		pollingInterval := viper.GetDuration("POLLING_INTERVAL")
		if pollingInterval == 0 {
			pollingInterval = 1 * time.Minute // Default: 1 minute
		}

		pollingService = services.NewPollingService(pollingInterval)
		
		// Start in background
		ctx := context.Background()
		go pollingService.Start(ctx)

		logger.WithFields(logger.Fields{
			"interval":    pollingInterval,
			"minOrderAge": viper.GetDuration("POLLING_MIN_AGE"),
		}).Infof("✅ Polling service started (fallback mode)")
	} else {
		logger.Infof("⏭️  Polling service disabled (webhook-only mode)")
	}

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		sig := <-sigChan
		logger.Infof("Received signal: %v, shutting down gracefully...", sig)
		
		// Stop polling service
		if pollingService != nil {
			pollingService.Stop()
			logger.Infof("Polling service stopped")
		}
		
		// Close database connection
		storage.GetClient().Close()
		logger.Infof("Database connection closed")
		
		os.Exit(0)
	}()

	// Run the server
	router := routers.Routes()

	appServer := fmt.Sprintf("%s:%s", conf.Host, conf.Port)
	logger.Infof("Server Running at :%v", appServer)

	logger.Fatalf("%v", router.Run(appServer))
}
