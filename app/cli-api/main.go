package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"plandex-cli-api/config"
	"plandex-cli-api/server"
)

var (
	configPath = flag.String("config", "", "Path to configuration file")
	port       = flag.Int("port", 8080, "Port to run API server on")
	help       = flag.Bool("help", false, "Show help message")
)

func main() {
	flag.Parse()

	if *help {
		fmt.Println("Plandex CLI API Wrapper")
		fmt.Println("Usage: plandex-cli-api [options]")
		fmt.Println("\nOptions:")
		flag.PrintDefaults()
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Override port from command line if provided
	if *port != 8080 {
		cfg.Server.Port = *port
	}

	// Initialize server
	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize server: %v", err)
	}

	// Setup graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		log.Println("Shutting down server...")
		srv.Shutdown()
		os.Exit(0)
	}()

	// Start server
	log.Printf("Starting Plandex CLI API server on port %d", cfg.Server.Port)
	if err := srv.Start(); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
