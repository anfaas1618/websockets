package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	githubevents "github.com/anfaas/websockets/internal/github"
)

// Broadcaster is implemented by the hub.
type Broadcaster interface {
	Broadcast([]byte)
}

// Handler receives GitHub webhook POSTs, verifies the HMAC signature (when a
// secret is configured), and broadcasts the event to all WebSocket clients.
type Handler struct {
	secret      []byte
	broadcaster Broadcaster
}

func New(secret string, broadcaster Broadcaster) *Handler {
	return &Handler{
		secret:      []byte(secret),
		broadcaster: broadcaster,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20)) // 10 MB max
	if err != nil {
		http.Error(w, "failed to read body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	// Verify HMAC signature when a secret is set.
	if len(h.secret) > 0 {
		sig := r.Header.Get("X-Hub-Signature-256")
		if !verifySignature(h.secret, body, sig) {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
	}

	eventType := r.Header.Get("X-GitHub-Event")
	delivery := r.Header.Get("X-GitHub-Delivery")

	// Decode the raw payload.
	// GitHub sends JSON directly for application/json, or as a "payload" form
	// field for application/x-www-form-urlencoded.
	// Restore the body so ParseForm can read it.
	jsonBody := body
	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "application/x-www-form-urlencoded") {
		r.Body = io.NopCloser(bytes.NewReader(body))
		if err := r.ParseForm(); err != nil {
			http.Error(w, "failed to parse form", http.StatusBadRequest)
			return
		}
		encoded := r.FormValue("payload")
		if encoded == "" {
			http.Error(w, "missing payload field", http.StatusBadRequest)
			return
		}
		jsonBody = []byte(encoded)
	}

	var payload any
	if err := json.Unmarshal(jsonBody, &payload); err != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}

	envelope := githubevents.Event{
		EventType: eventType,
		Delivery:  delivery,
		Payload:   payload,
	}

	data, err := json.Marshal(envelope)
	if err != nil {
		http.Error(w, "failed to marshal event", http.StatusInternalServerError)
		return
	}

	h.broadcaster.Broadcast(data)
	log.Printf("[webhook] event=%s delivery=%s broadcasted", eventType, delivery)

	w.WriteHeader(http.StatusNoContent)
}

func verifySignature(secret, body []byte, sigHeader string) bool {
	const prefix = "sha256="
	if !strings.HasPrefix(sigHeader, prefix) {
		return false
	}
	gotHex := strings.TrimPrefix(sigHeader, prefix)
	got, err := hex.DecodeString(gotHex)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	expected := mac.Sum(nil)
	if !hmac.Equal(expected, got) {
		return false
	}
	return true
}

// PingResponse is sent back to GitHub's ping event.
func PingResponse(w http.ResponseWriter) {
	fmt.Fprintln(w, `{"ok":true}`)
}
