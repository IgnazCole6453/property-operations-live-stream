package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/example/property-ops-stream/internal/infrai"
	"github.com/example/property-ops-stream/internal/property"
)

const channel = "property-operations"

type dashboardPayload struct {
	Operations property.Snapshot `json:"operations"`
	Metrics    json.RawMessage   `json:"metrics"`
}

type server struct {
	infrai *infrai.Client
}

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	client := infrai.New(key)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.CreateChannel(ctx, channel); err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	s := &server{infrai: client}
	mux.HandleFunc("POST /operations", s.operations)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	log.Println("property operations stream listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func (s *server) operations(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var input property.Operations
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		http.Error(w, "invalid operations payload", http.StatusBadRequest)
		return
	}
	if input.AsOf.IsZero() {
		http.Error(w, "as_of is required", http.StatusBadRequest)
		return
	}

	metrics, err := s.infrai.QueryMetrics(r.Context())
	if err != nil {
		writeInfraiError(w, err)
		return
	}
	payload := dashboardPayload{Operations: property.Summarize(input), Metrics: metrics}
	requestID, err := newRequestID()
	if err != nil {
		http.Error(w, "could not create request id", http.StatusInternalServerError)
		return
	}
	if err := s.infrai.Publish(r.Context(), channel, "operations.snapshot", requestID, payload); err != nil {
		writeInfraiError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(payload)
}

func writeInfraiError(w http.ResponseWriter, err error) {
	var apiErr *infrai.APIError
	if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
		http.Error(w, apiErr.Message, apiErr.HTTPStatus)
		return
	}
	http.Error(w, "upstream request failed", http.StatusBadGateway)
}

func newRequestID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
