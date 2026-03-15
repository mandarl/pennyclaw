package main

import (
"flag"
"fmt"
"log"
"os"
"os/signal"
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

// Create agent (initializes LLM, memory, sandbox, skills internally)
ag, err := agent.New(cfg)
if err != nil {
log.Fatalf("Failed to initialize agent: %v", err)
}

// Start web server
srv := web.NewServer(cfg.Server.Host, cfg.Server.Port, ag.HandleMessage)
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
srv.Stop()
log.Println("Goodbye!")
}
