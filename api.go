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
}

func NewAPIServer(submitter CommandSubmitter) (*APIServer, error) {
	if submitter == nil {
		return nil, fmt.Errorf("submitter cannot be nil")
	}

	return &APIServer{
		submitter: submitter,
	}, nil
}

func (s *APIServer) submitCommand(w http.ResponseWriter, r *http.Request) {
	var cmd Command

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // Limit request body to 1MB

	decoder := json.NewDecoder(r.Body)

	if err := decoder.Decode(&cmd); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) && err != http.ErrBodyReadAfterClose {
		http.Error(w, "request must contain a single JSON object", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	if err := s.submitter.Submit(ctx, cmd); err != nil {
		http.Error(w, "command could not be accepted: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (s *APIServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /commands", s.submitCommand)
	return mux
}
