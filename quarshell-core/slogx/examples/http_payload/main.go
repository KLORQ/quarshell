// Package main demonstrates sanitized HTTP request/response payload logging.
package main

import (
	"context"
	"net/http"
	"strings"

	"github.com/KLORQ/quarshell-core/slogx"
)

func main() {
	log := slogx.New(
		slogx.WithFormat(slogx.FormatJSON),
		slogx.WithDumpLimits(5, 64),
	)

	req, _ := http.NewRequest(
		http.MethodPost,
		"https://api.example.com/v1/login",
		strings.NewReader(`{"email":"alice@example.com","password":"secret"}`),
	)
	req.Header.Set("Authorization", "Bearer sk-live-secret")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", "req-100")

	ctx := context.Background()

	// Sensitive headers (Authorization, Cookie, ...) are redacted automatically.
	// Body is read up to a limit and Request.Body is restored for downstream use.
	log.InfoContext(ctx, "incoming http request",
		slogx.HTTPRequestWith(log, "request", req),
	)

	resp := &http.Response{
		Status: "200 OK",
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"Set-Cookie":   []string{"session=abc; HttpOnly"},
		},
		Body: http.NoBody,
	}
	log.InfoContext(ctx, "outgoing http response",
		slogx.HTTPResponseWith(log, "response", resp),
	)
}
