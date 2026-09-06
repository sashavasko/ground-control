package main

import (
	"context"
	"encoding/json"
	"fmt"
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

func (s *APIServer) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var cmd Command
		if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
			http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
			return
		}

		ctx := r.Context()
		if err := s.submitter.Submit(ctx, cmd); err != nil {
			http.Error(w, "failed to submit command: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusAccepted)
	})
}
