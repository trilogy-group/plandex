package main

import (
	"flag"
	"fmt"
	"log"

	"plandex-cli-api/config"
	"plandex-cli-api/server"
)

func main() {
	var configPath = flag.String("config", "plandex-api.json", "Path to configuration file")
	var port = flag.Int("port", 8080, "Port to run API server on")
	var help = flag.Bool("help", false, "Show help message")
	flag.Parse()

	if *help {
		fmt.Println("Plandex CLI API Wrapper")
		fmt.Println("Usage: plandex-cli-api [options]")
		fmt.Println()
		fmt.Println("Options:")
		flag.PrintDefaults()
		fmt.Println()
		fmt.Println("Example config file:")
		fmt.Println(`{
  "server": {
    "port": 8080,
    "host": "0.0.0.0"
  },
  "auth": {
    "api_keys": ["your-api-key-here"],
    "require_auth": true
  },
  "cli": {
    "auto_detect_stl": true,
    "api_keys": {
      "OPENAI_API_KEY": "your-openai-api-key-here"
    }
  },
  "webhooks": {
    "enabled": true
  }
}`)
		return
	}

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// Override port if specified
	if *port != 8080 {
		cfg.Server.Port = *port
	}

	log.Printf("Starting Plandex CLI API server on port %d", cfg.Server.Port)
	log.Printf("Working directory: %s", cfg.CLI.WorkingDir)
	if cfg.CLI.AutoDetectSTL {
		log.Printf("STL auto-detection enabled")
	}

	// Create and start server
	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("Error creating server: %v", err)
	}

	if err := srv.Start(); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
