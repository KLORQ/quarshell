package httpproxy

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/KLORQ/quarshell-core/domain"
)

// Authorizer authenticates a proxy client before policy evaluation.
type Authorizer interface {
	Authorize(context.Context, *http.Request) error
}

// StaticToken checks a bearer token sent as Proxy-Authorization or
// X-Quarshell-Proxy-Token. An empty token rejects every request.
type StaticToken struct {
	Token string
}

// Authorize compares the supplied token in constant time.
func (s StaticToken) Authorize(ctx context.Context, request *http.Request) error {
	if err := ctx.Err(); err != nil {
		return domain.Wrap(domain.ErrCanceled, "httpproxy.auth", "authorization canceled")
	}
	if s.Token == "" {
		return domain.Wrap(domain.ErrUnauthorized, "httpproxy.auth", "proxy token is not configured")
	}
	value := request.Header.Get("Proxy-Authorization")
	if value == "" {
		value = request.Header.Get("X-Quarshell-Proxy-Token")
	}
	value = strings.TrimSpace(value)
	if len(value) > len("Bearer ") && strings.EqualFold(value[:len("Bearer ")], "Bearer ") {
		value = strings.TrimSpace(value[len("Bearer "):])
	}
	if subtle.ConstantTimeCompare([]byte(value), []byte(s.Token)) != 1 {
		return domain.Wrap(domain.ErrUnauthorized, "httpproxy.auth", "invalid proxy token")
	}
	return nil
}
