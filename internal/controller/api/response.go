package api

import (
	"encoding/json"
	"fmt"
	"net/http"
)

const requestIDHeader = "X-Request-ID"

// envelope wraps a successful JSON response.
type envelope struct {
	Data any            `json:"data"`
	Meta map[string]any `json:"meta"`
}

// problem is an RFC 9457 Problem Details object.
type problem struct {
	Type      string `json:"type"`
	Title     string `json:"title"`
	Status    int    `json:"status"`
	Detail    string `json:"detail,omitempty"`
	Instance  string `json:"instance,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

// writeJSON serialises data as a JSON envelope and writes it to w.
func writeJSON(w http.ResponseWriter, r *http.Request, status int, data any) {
	reqID := r.Header.Get(requestIDHeader)
	env := envelope{
		Data: data,
		Meta: map[string]any{"request_id": reqID},
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(env)
}

// writeProblem writes an RFC 9457 problem details response.
func writeProblem(w http.ResponseWriter, r *http.Request, status int, title, detail string) {
	reqID := r.Header.Get(requestIDHeader)
	p := problem{
		Type:      fmt.Sprintf("https://distencoder.dev/errors/%s", problemSlug(status)),
		Title:     title,
		Status:    status,
		Detail:    detail,
		Instance:  r.URL.Path,
		RequestID: reqID,
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(p)
}

func problemSlug(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not-found"
	case http.StatusUnprocessableEntity:
		return "validation"
	case http.StatusBadRequest:
		return "bad-request"
	default:
		return "internal"
	}
}
