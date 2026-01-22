// Command aquitar-server runs the Aquitar reverse tunnel proxy server.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/atelier-bm/aquitar/server"
)

// configFile represents the JSON configuration file format.
type configFile struct {
	Domain          string `json:"domain"`
	PSK             string `json:"psk"`
	ControlPort     int    `json:"control_port"`
	DisallowedPorts []int  `json:"disallowed_ports"`
	CertCache       string `json:"cert_cache"`
}

func main() {
	configPath := flag.String("config", "", "Path to configuration file")
	domain := flag.String("domain", "", "Domain name for TLS certificates")
	psk := flag.String("psk", "", "Pre-shared key for authentication")
	port := flag.Int("port", 8443, "Control port")
	flag.Parse()

	var cfg server.ServerConfig

	// Load config from file if specified
	if *configPath != "" {
		data, err := os.ReadFile(*configPath)
		if err != nil {
			log.Fatalf("Failed to read config file: %v", err)
		}

		var fileCfg configFile
		if err := json.Unmarshal(data, &fileCfg); err != nil {
			log.Fatalf("Failed to parse config file: %v", err)
		}

		cfg = server.ServerConfig{
			Domain:          fileCfg.Domain,
			PSK:             fileCfg.PSK,
			ControlPort:     fileCfg.ControlPort,
			DisallowedPorts: fileCfg.DisallowedPorts,
			CertCacheDir:    fileCfg.CertCache,
		}
	}

	// Command-line flags override config file
	if *domain != "" {
		cfg.Domain = *domain
	}
	if *psk != "" {
		cfg.PSK = *psk
	}
	if *port != 8443 || cfg.ControlPort == 0 {
		cfg.ControlPort = *port
	}

	// Validate
	if cfg.PSK == "" {
		log.Fatal("PSK is required (use -psk or config file)")
	}

	// Default disallowed ports if not set
	if len(cfg.DisallowedPorts) == 0 {
		cfg.DisallowedPorts = []int{cfg.ControlPort}
	}

	// Create server
	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("Shutting down...")
		cancel()
	}()

	// Start server
	log.Printf("Starting Aquitar server on port %d", cfg.ControlPort)
	if cfg.Domain != "" {
		log.Printf("TLS enabled for domain: %s", cfg.Domain)
	}

	if err := srv.ListenAndServe(ctx); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
