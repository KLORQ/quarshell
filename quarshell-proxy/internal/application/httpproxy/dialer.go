package httpproxy

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/KLORQ/quarshell-core/domain"
	"github.com/KLORQ/quarshell-proxy/internal/application/mediation"
)

// HTTPDialer performs the upstream request after mediation. Redirects are not
// followed so a credential or body cannot be replayed to a new origin.
type HTTPDialer struct {
	Client      *http.Client
	MaxResponse int64
}

// Do performs one upstream HTTP request.
func (d *HTTPDialer) Do(ctx context.Context, request mediation.Request) (mediation.Response, error) {
	client := d.Client
	if client == nil {
		client = &http.Client{
			Timeout:       30 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}
	}
	scheme := request.Scheme
	if scheme == "" {
		scheme = "http"
	}
	target := &url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(request.Host, strconv.Itoa(int(request.Port))),
		Path:   request.Path,
	}
	if len(request.Query) > 0 {
		values := target.Query()
		for key, value := range request.Query {
			values.Set(key, value)
		}
		target.RawQuery = values.Encode()
	}
	httpRequest, err := http.NewRequestWithContext(ctx, request.Method, target.String(), bytes.NewReader(request.Body))
	if err != nil {
		return mediation.Response{}, domain.Wrap(err, "httpproxy.dial", "build upstream request")
	}
	for key, value := range request.Headers {
		httpRequest.Header.Set(key, value)
	}
	response, err := client.Do(httpRequest)
	if err != nil {
		return mediation.Response{}, domain.Wrap(err, "httpproxy.dial", "upstream request failed")
	}
	defer response.Body.Close()
	maxResponse := d.MaxResponse
	if maxResponse <= 0 {
		maxResponse = 8 << 20
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponse+1))
	if err != nil {
		return mediation.Response{}, domain.Wrap(err, "httpproxy.dial", "read upstream response")
	}
	if int64(len(body)) > maxResponse {
		return mediation.Response{}, domain.New(domain.CodeUnavailable, "httpproxy.dial", "upstream response too large")
	}
	headers := make(map[string]string, len(response.Header))
	for key, values := range response.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}
	return mediation.Response{Status: response.StatusCode, Body: body, Headers: headers}, nil
}
