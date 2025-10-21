package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

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

	// Run the bot with the loaded configuration
	bot.RunWithConfig(config)

	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Printf("Error starting server: %s\n", err)
	}
}
