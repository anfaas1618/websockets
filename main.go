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
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func main() {
	addr := getEnv("ADDR", ":443")
	secret := getEnv("GITHUB_WEBHOOK_SECRET", "")
	certFile := getEnv("TLS_CERT", "")
	keyFile := getEnv("TLS_KEY", "")

	h := hub.New()
	go h.Run()

	whHandler := webhook.New(secret, h)

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

	// GET /          — dashboard UI
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	useTLS := certFile != "" && keyFile != ""
	scheme := "http"
	if useTLS {
		scheme = "https"
	}
	log.Printf("[server] dashboard  : %s://0.0.0.0%s/", scheme, addr)
	log.Printf("[server] webhook    : POST /webhook")
	log.Printf("[server] websocket  : %s://... /ws", map[bool]string{true: "wss", false: "ws"}[useTLS])
	if useTLS {
		log.Printf("[server] cert       : %s", certFile)
		if err := http.ListenAndServeTLS(addr, certFile, keyFile, mux); err != nil {
			log.Fatalf("[server] fatal: %v", err)
		}
	} else {
		log.Printf("[server] TLS        : disabled (set TLS_CERT and TLS_KEY to enable)")
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Fatalf("[server] fatal: %v", err)
		}
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
