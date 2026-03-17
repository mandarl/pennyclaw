package main

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/mandarl/pennyclaw/internal/agent"
	"github.com/mandarl/pennyclaw/internal/channels/web"
	"github.com/mandarl/pennyclaw/internal/config"
)

var (
	version   = "dev"
	buildDate = "unknown"
)

func main() {
	configPath := flag.String("config", "config.json", "path to configuration file")
	showVersion := flag.Bool("version", false, "print version and exit")
	insecure := flag.Bool("insecure", false, "allow running without authentication (NOT recommended)")
	flag.Parse()

	if *showVersion {
		fmt.Printf("PennyClaw %s (built %s)\n", version, buildDate)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Secure-by-default: require PENNYCLAW_AUTH_TOKEN unless --insecure is passed
	authToken := os.Getenv("PENNYCLAW_AUTH_TOKEN")
	if authToken == "" && !*insecure {
		// Auto-generate a token and set it
		token, err := generateToken()
		if err != nil {
			log.Fatalf("Failed to generate auth token: %v", err)
		}
		os.Setenv("PENNYCLAW_AUTH_TOKEN", token)
		log.Println("═══════════════════════════════════════════════════════")
		log.Println("  No PENNYCLAW_AUTH_TOKEN set — generated one for you:")
		log.Printf("  %s", token)
		log.Println("")
		log.Println("  Save this token! You'll need it to access the web UI.")
		log.Println("  To set permanently: export PENNYCLAW_AUTH_TOKEN=<token>")
		log.Println("═══════════════════════════════════════════════════════")
	} else if authToken == "" && *insecure {
		log.Println("WARNING: Running in --insecure mode. Web UI is open to anyone on the network!")
		log.Println("WARNING: Do NOT use --insecure in production or on a public-facing server.")
	}

	// Determine data directory (same parent as the DB path)
	dataDir := filepath.Dir(cfg.Memory.DBPath)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// Set agent version from build-time ldflags
	agent.Version = version

	// Create agent (initializes LLM, memory, sandbox, skills, workspace, cron)
	ag, err := agent.New(cfg, dataDir)
	if err != nil {
		log.Fatalf("Failed to initialize agent: %v", err)
	}

	// Start web server
	srv := web.NewServer(cfg.Server.Host, cfg.Server.Port, ag.HandleMessage, cfg, *configPath,
		ag.Memory(), version, ag.Workspace(), ag.Scheduler())
	go func() {
		log.Printf("PennyClaw %s starting on %s:%d", version, cfg.Server.Host, cfg.Server.Port)
		if err := srv.Start(); err != nil {
			log.Fatalf("Web server error: %v", err)
		}
	}()

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down PennyClaw...")
	ag.Stop()
	srv.Stop()
	log.Println("Goodbye!")
}

// generateToken creates a cryptographically random 32-byte hex token.
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
