package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"grok-bot/bot"
)

func main() {
	// Parse command line flags
	var configPath string
	flag.StringVar(&configPath, "config", "", "Path to configuration file")
	flag.Parse()

	// Load configuration
	config, err := bot.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Print config file being used
	if configFile := bot.GetConfigPath(); configFile != "" {
		fmt.Printf("Using configuration file: %s\n", configFile)
	} else {
		fmt.Println("Using default configuration with environment variables")
	}

	// Create a context that can be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// WaitGroup to wait for all goroutines to finish
	var wg sync.WaitGroup

	// Start the Discord bot in a goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("Starting Discord bot...")
		bot.RunWithConfigAsync(ctx, config)
		log.Println("Discord bot stopped")
	}()

	// Start the web server in a goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()

		// Check if server is enabled
		if !config.Server.Enabled {
			log.Println("Web server is disabled in configuration")
			return
		}

		log.Printf("Starting web server on port %s...", config.Server.Port)

		// Setup HTTP routes
		http.HandleFunc("/", handleRoot)
		http.HandleFunc("/health", handleHealth)
		http.HandleFunc("/status", handleStatus)

		server := &http.Server{
			Addr:    ":" + config.Server.Port,
			Handler: nil,
		}

		// Start server in a goroutine
		go func() {
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("Web server error: %v", err)
			}
		}()

		// Wait for context cancellation
		<-ctx.Done()

		// Gracefully shutdown the server
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
		log.Println("Web server stopped")
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Both Discord bot and web server are running. Press Ctrl+C to stop.")

	// Wait for signal
	<-sigChan
	log.Println("Shutdown signal received, stopping services...")

	// Cancel context to stop all services
	cancel()

	// Wait for all goroutines to finish (with timeout)
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("All services stopped gracefully")
	case <-time.After(10 * time.Second):
		log.Println("Timeout waiting for services to stop")
	}
}

// HTTP handlers
func handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <title>Grok Bot Status</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .status { color: green; font-weight: bold; }
        .info { background: #f0f0f0; padding: 20px; border-radius: 5px; }
    </style>
</head>
<body>
    <h1>Grok Discord Bot</h1>
    <p class="status"> Bot is running</p>
    <div class="info">
        <h3>Available endpoints:</h3>
        <ul>
            <li><a href="/health">/health</a> - Health check</li>
            <li><a href="/status">/status</a> - Detailed status</li>
        </ul>
    </div>
</body>
</html>
`)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status": "healthy", "timestamp": "%s"}`, time.Now().Format(time.RFC3339))
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{
		"bot": "running",
		"server": "running",
		"timestamp": "%s",
		"uptime": "%s"
	}`, time.Now().Format(time.RFC3339), time.Since(time.Now()).String())
}
