package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"

	"dnd-vote-ii/internal/app"
)

func main() {
	cfg, err := app.LoadConfig()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	store, err := app.OpenStore(context.Background(), cfg.DatabasePath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer store.Close()

	server := app.NewHTTPServer(cfg, app.NewRouter(cfg, store))
	fmt.Printf("Serving D&D vote app on %s\n", browserURL(cfg.Host, cfg.Port))
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("serve: %v", err)
	}
}

func browserURL(host string, port string) string {
	displayHost := host
	if host == "" || host == "0.0.0.0" || host == "::" {
		displayHost = "localhost"
	}
	if strings.Contains(displayHost, ":") && !strings.HasPrefix(displayHost, "[") {
		displayHost = "[" + displayHost + "]"
	}
	return "http://" + net.JoinHostPort(displayHost, port)
}
