package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"g360-ventas-api/internal/auth"
	"g360-ventas-api/internal/config"
	"g360-ventas-api/internal/db"
	"g360-ventas-api/internal/handlers"
)

func main() {
	cfg := config.Load()

	store, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("no se pudo abrir %s: %v", cfg.DBPath, err)
	}
	defer store.Close()

	if err := store.Ping(context.Background()); err != nil {
		log.Printf("ADVERTENCIA: db no responde aun (%v) — el API arranca y reporta 503 hasta que exista", err)
	} else {
		log.Printf("db: %s (%d objetos permitidos)", cfg.DBPath, len(store.Allowed()))
	}

	if cfg.User == "" || cfg.Pass == "" {
		log.Print("ADVERTENCIA: sin credenciales intranet (G360_INTRANET_USER/PASS o config.json) — login rechazara todo")
	} else {
		log.Printf("login: credenciales intranet cargadas (usuario %q)", cfg.User)
	}

	am := auth.New(cfg.Secret, cfg.User, cfg.Pass, cfg.TokenTTL)
	srv := handlers.New(cfg, store, am)

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("g360-ventas-api escuchando en http://%s", cfg.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Print("cerrando...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	httpServer.Shutdown(ctx)
}
