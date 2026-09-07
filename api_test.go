package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type recordingSubmitter struct {
	submitted chan<- Command
	err       error
}

func (s *recordingSubmitter) Submit(ctx context.Context, cmd Command) error {
	if s.err != nil {
		return s.err
	}

	select {
	case s.submitted <- cmd:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestAPIServer_SubmitCommand(t *testing.T) {
	submitted := make(chan Command, 1)
	submitter := &recordingSubmitter{
		submitted: submitted,
	}
	badSubmitter := &recordingSubmitter{
		err: fmt.Errorf("submitter error"),
	}

	tests := []struct {
		name       string
		method     string
		url        string
		body       string
		wantStatus int
		submitter  *recordingSubmitter
	}{
		{
			name:       "valid command",
			method:     http.MethodPost,
			url:        "/commands",
			body:       `{"satelliteId":"SAT-1","sequence":1,"payload":"CAPTURE"}`,
			wantStatus: http.StatusAccepted,
			submitter:  submitter,
		},
		{
			name:       "invalid sequence",
			method:     http.MethodPost,
			url:        "/commands",
			body:       `{"satelliteId":"SAT-1","sequence":-1,"payload":"CAPTURE"}`,
			wantStatus: http.StatusBadRequest,
			submitter:  submitter,
		},
		{
			name:       "missing sequence",
			method:     http.MethodPost,
			url:        "/commands",
			body:       `{"satelliteId":"SAT-1","payload":"CAPTURE"}`,
			wantStatus: http.StatusBadRequest,
			submitter:  submitter,
		},
		{
			name:       "missing satelliteId",
			method:     http.MethodPost,
			url:        "/commands",
			body:       `{"sequence":1,"payload":"CAPTURE"}`,
			wantStatus: http.StatusBadRequest,
			submitter:  submitter,
		},
		{
			name:       "missing payload",
			method:     http.MethodPost,
			url:        "/commands",
			body:       `{"satelliteId":"SAT-1","sequence":1}`,
			wantStatus: http.StatusBadRequest,
			submitter:  submitter,
		},
		{
			name:       "invalid method",
			method:     http.MethodGet,
			url:        "/commands",
			body:       ``,
			wantStatus: http.StatusMethodNotAllowed,
			submitter:  submitter,
		},
		{
			name:       "malformed JSON",
			method:     http.MethodPost,
			url:        "/commands",
			body:       `foobar`,
			wantStatus: http.StatusBadRequest,
			submitter:  submitter,
		},
		{
			name:       "multiple JSON objects",
			method:     http.MethodPost,
			url:        "/commands",
			body:       `{"satelliteId":"SAT-1","sequence":1,"payload":"CAPTURE"}{"satelliteId":"SAT-1","sequence":2,"payload":"TRANSMIT"}`,
			wantStatus: http.StatusBadRequest,
			submitter:  submitter,
		},
		{
			name:       "bad url",
			method:     http.MethodPost,
			url:        "/invalid",
			body:       `{"satelliteId":"SAT-1","sequence":1,"payload":"CAPTURE"}`,
			wantStatus: http.StatusNotFound,
			submitter:  submitter,
		},
		{
			name:       "submitter error",
			method:     http.MethodPost,
			url:        "/commands",
			body:       `{"satelliteId":"SAT-666","sequence":1,"payload":"CAPTURE"}`,
			wantStatus: http.StatusServiceUnavailable,
			submitter:  badSubmitter,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(tt.method, tt.url, strings.NewReader(tt.body))
			response := httptest.NewRecorder()

			server, err := NewAPIServer(tt.submitter)
			if err != nil {
				t.Fatalf("NewAPIServer() error = %v", err)
			}

			server.Handler().ServeHTTP(response, request)

			if response.Code != tt.wantStatus {
				t.Errorf("expected status code %d, but got %d", tt.wantStatus, response.Code)
			}
			if response.Code == http.StatusAccepted {
				select {
				case cmd := <-submitted:
					if cmd.SatelliteID != "SAT-1" || cmd.Sequence != 1 || string(cmd.Payload) != "CAPTURE" {
						t.Errorf("unexpected command submitted: %+v", cmd)
					}
				default:
					t.Fatal("expected command to be submitted, but none was")
				}
			}
		})
	}
}
