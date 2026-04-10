package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/anfaas/websockets/internal/hub"
	"github.com/anfaas/websockets/internal/webhook"
	"github.com/gorilla/websocket"
)

//go:embed static
var staticFiles embed.FS

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins; restrict in production.
	CheckOrigin: func(r *http.Request) bool { return true },
}

func main() {
	addr := getEnv("ADDR", ":8080")
	secret := getEnv("GITHUB_WEBHOOK_SECRET", "")

	h := hub.New()
	go h.Run()

	whHandler := webhook.New(secret, h)

	// Serve static/ as the root file system.
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("failed to sub static fs: %v", err)
	}

	mux := http.NewServeMux()

	// POST /webhook  — GitHub sends events here
	mux.Handle("/webhook", whHandler)

	// GET /ws        — WebSocket clients connect here
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("[ws] upgrade error: %v", err)
			return
		}
		h.ServeClient(conn)
	})

	// GET /healthz   — liveness probe
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// GET /          — dashboard UI (embedded static files)
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	log.Printf("[server] listening on %s", addr)
	log.Printf("[server] dashboard  : GET  /")
	log.Printf("[server] webhook    : POST /webhook")
	log.Printf("[server] websocket  : GET  /ws")

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("[server] fatal: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
