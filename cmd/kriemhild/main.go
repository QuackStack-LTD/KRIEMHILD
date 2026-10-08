package main

import (
	"context"
	"flag"
	"kriemhild/internal/httpapi"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8124", "HTTP listen address")
	dist := flag.String("dist", "dist", "React build directory")
	flag.Parse()
	server := &http.Server{Addr: *addr, Handler: httpapi.New(*dist), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 60 * time.Second}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	go func() {
		<-stop
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()
	log.Printf("KRIEMHILD is running at http://%s", *addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
