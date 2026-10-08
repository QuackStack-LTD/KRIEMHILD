package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"kriemhild/internal/httpapi"
	"kriemhild/internal/storage"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
func databaseURL() (string, error) {
	value, file := os.Getenv("DATABASE_URL"), os.Getenv("DATABASE_URL_FILE")
	if value != "" && file != "" {
		return "", errors.New("configure DATABASE_URL or DATABASE_URL_FILE, not both")
	}
	if file != "" {
		b, err := os.ReadFile(file)
		if err != nil {
			return "", errors.New("cannot read DATABASE_URL_FILE")
		}
		value = strings.TrimSpace(string(b))
		if value == "" {
			return "", errors.New("DATABASE_URL_FILE is empty")
		}
	}
	return value, nil
}
func healthcheck(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	client := http.Client{Timeout: 3 * time.Second}
	response, err := client.Get("http://" + net.JoinHostPort(host, port) + "/api/ready")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return fmt.Errorf("readiness returned HTTP %d", response.StatusCode)
	}
	return nil
}
func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
func run() error {
	addr := flag.String("addr", env("KRIEMHILD_ADDR", "127.0.0.1:8124"), "HTTP listen address")
	dist := flag.String("dist", env("KRIEMHILD_DIST", "dist"), "React build directory")
	data := flag.String("data", env("KRIEMHILD_DATA_DIR", "data"), "Writable project database and cache directory")
	check := flag.Bool("healthcheck", false, "Check the running server readiness and exit")
	flag.Parse()
	if *check {
		return healthcheck(*addr)
	}
	if _, err := os.Stat(filepath.Join(*dist, "index.html")); err != nil {
		return errors.New("React build missing; run npm run build or configure KRIEMHILD_DIST")
	}
	dsn, err := databaseURL()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	store, err := storage.Open(ctx, *data, dsn)
	cancel()
	if err != nil {
		return err
	}
	defer store.Close()
	cache := filepath.Join(*data, "cache")
	if err = os.MkdirAll(cache, 0700); err != nil {
		return fmt.Errorf("project cache directory is not writable: %w", err)
	}
	app := httpapi.NewWithOptions(*dist, httpapi.Options{Projects: store, CacheDir: cache})
	server := &http.Server{Addr: *addr, Handler: app, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 2 * time.Minute, WriteTimeout: 5 * time.Minute, IdleTimeout: 60 * time.Second}
	stop, release := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer release()
	finished := make(chan struct{})
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		select {
		case <-stop.Done():
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := server.Shutdown(ctx); err != nil {
				server.Close()
			}
		case <-finished:
		}
	}()
	log.Printf("KRIEMHILD listening on %s; project database: %s", *addr, store.Driver())
	err = server.ListenAndServe()
	close(finished)
	<-shutdownDone
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
