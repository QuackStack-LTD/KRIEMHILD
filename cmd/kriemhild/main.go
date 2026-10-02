package main

import (
	"context"
	"flag"
	"fmt"
	"kriemhild/internal/httpapi"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func main() {
	data := flag.String("data", "worlds-dev", "local project library directory")
	web := flag.String("web", "apps/web/out-dev", "compiled Next static export directory")
	addr := flag.String("addr", "127.0.0.1:4784", "loopback address and port")
	origin := flag.String("origin", "", "explicit HTTPS origin to enable hosted authentication behind a reverse proxy")
	flag.Parse()
	host, _, e := net.SplitHostPort(*addr)
	if e != nil || net.ParseIP(host) == nil || (*origin == "" && !net.ParseIP(host).IsLoopback()) {
		log.Fatal("-addr must be a loopback IP and port, e.g. 127.0.0.1:4780")
	}
	if _, e = os.Stat(filepath.Join(*web, "index.html")); e != nil {
		log.Fatal("frontend missing: run npm --prefix apps/web run build, or pass -web")
	}
	app, e := httpapi.New(*data, *web)
	if e != nil {
		log.Fatal(e)
	}
	defer app.Close()
	if *origin != "" {
		if e := app.EnableHosting(*origin, os.Getenv("KRIEMHILD_ADMIN_PASSWORD")); e != nil {
			log.Fatal(e)
		}
	}
	server := &http.Server{Addr: *addr, Handler: app, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 120 * time.Second, IdleTimeout: 60 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdown)
	}()
	fmt.Printf("KRIEMHILD development\nOpen http://%s\nProject library: %s\nCtrl+C to stop.\n", *addr, app.Data)
	if e = server.ListenAndServe(); e != nil && e != http.ErrServerClosed {
		log.Fatal(e)
	}
}
