package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

type CommandSubmitter interface {
	Submit(context.Context, Command) error
}

var _ CommandSubmitter = (*Dispatcher)(nil)

type APIServer struct {
	submitter CommandSubmitter
	ready     ReadinessFunc
}

func NewAPIServer(submitter CommandSubmitter, ready ReadinessFunc) (*APIServer, error) {
	if submitter == nil {
		return nil, fmt.Errorf("submitter cannot be nil")
	}

	if ready == nil {
		return nil, fmt.Errorf("ready function cannot be nil")
	}

	return &APIServer{
		submitter: submitter,
		ready:     ready,
	}, nil
}

func (s *APIServer) submitCommand(w http.ResponseWriter, r *http.Request) {
	var cmd Command

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // Limit request body to 1MB

	decoder := json.NewDecoder(r.Body)

	if err := decoder.Decode(&cmd); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "invalid command: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		http.Error(w, "request must contain a single JSON object", http.StatusBadRequest)
		return
	}

	if err := s.submitter.Submit(r.Context(), cmd); err != nil {
		http.Error(w, "command could not be accepted", http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (s *APIServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /commands", s.submitCommand)
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /readyz", s.readiness)
	return mux
}

type ReadinessFunc func() bool

func (s *APIServer) readiness(w http.ResponseWriter, r *http.Request) {
	if s.ready() {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("not ready"))
	}
}

func (s *APIServer) health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("healthy"))
}
