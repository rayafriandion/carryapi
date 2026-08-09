package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"carryapi/internal/config"
	"carryapi/internal/db"
	"carryapi/internal/server"
	"carryapi/internal/settings"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	d, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	defer d.Close()
	if err := db.Migrate(d); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	srv := server.New(cfg, d, settings.New(d))

	// 信号处理
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		fmt.Println("\nshutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}()

	if err := srv.ListenAndServe(); err != nil && err.Error() != "http: Server closed" {
		log.Fatalf("serve: %v", err)
	}
}
