package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"tv-proxy-go/api"
	"tv-proxy-go/proxy"
	"tv-proxy-go/store"
)

func main() {
	port := envInt("PORT", 8080)
	maxConcurrent := envInt("MAX_CONCURRENT", 1000)
	proxyBase := os.Getenv("PROXY_BASE")
	if proxyBase == "" {
		proxyBase = fmt.Sprintf("http://127.0.0.1:%d", port)
	}

	ctx := context.Background()
	var dataStore *store.Store
	if strings.TrimSpace(os.Getenv("MONGODB_URI")) != "" {
		var err error
		dataStore, err = store.Open(ctx)
		if err != nil {
			log.Fatalf("mongodb store error: %v", err)
		}
	} else {
		log.Println("MONGODB_URI not set — channel data API disabled")
	}

	engine := proxy.NewEngine(maxConcurrent, proxyBase, os.Getenv("PROXY_TOKEN_SECRET"))

	mux := http.NewServeMux()
	mux.HandleFunc("/proxy", engine.HandleStream)
	if dataStore != nil {
		api.NewDataHandler(dataStore).Register(mux)
	}
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":              "ok",
			"active_streams":      engine.ActiveStreams(),
			"max_concurrent":      engine.MaxConcurrent(),
			"play_tokens_enabled": engine.PlayTokensEnabled(),
			"data_api_enabled":    dataStore != nil,
		})
	})

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           withCORS(mux),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       0,
		WriteTimeout:      0,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Printf("IPTV proxy listening on :%d (max_concurrent=%d)", port, maxConcurrent)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("shutting down gracefully...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
	engine.Shutdown()
	if dataStore != nil {
		if err := dataStore.Close(shutdownCtx); err != nil {
			log.Printf("mongodb disconnect error: %v", err)
		}
	}
	log.Println("server stopped")
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Range, Content-Type, Authorization")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Range, Accept-Ranges, Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func envInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}
