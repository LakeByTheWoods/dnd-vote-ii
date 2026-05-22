package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

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
	fmt.Printf("Serving D&D vote app on http://%s\n", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("serve: %v", err)
	}
}
