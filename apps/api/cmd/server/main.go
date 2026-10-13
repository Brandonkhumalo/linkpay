package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"linkpay/api/internal/platform"
)

func main() {
	ctx := context.Background()
	a, err := platform.New(ctx, os.Getenv("DATABASE_URL"), os.Getenv("PUBLIC_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer a.Close()
	server := &http.Server{Addr: env("PORT", "8080"), Handler: a.Routes(""), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	log.Printf("LinkPay API listening on %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}
func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return ":" + v
	}
	return ":" + fallback
}
