package credentials

import (
	"bytes"
	"context"
	"regexp"
	"strings"

	"github.com/KLORQ/quarshell-core/domain"
)

// Placeholder is the only token replaced by credential material.
const Placeholder = "{{credential}}"

// RewriteTarget selects the header or body safety rules.
type RewriteTarget int

const (
	RewriteBody RewriteTarget = iota
	RewriteHeader
)

// Rewriter substitutes a bound credential placeholder into a request.
type Rewriter struct {
	Broker       *Broker
	MaxBodyBytes int
}

// Rewrite resolves the handle for endpoint and replaces Placeholder. It rejects
// header injection and never echoes material in an error.
func (r *Rewriter) Rewrite(ctx context.Context, handle Handle, endpoint EndpointBinding, input []byte, target RewriteTarget) ([]byte, error) {
	if r.Broker == nil {
		return nil, domain.Invalidf("credentials.rewrite", "broker is required")
	}
	maxBytes := r.MaxBodyBytes
	if maxBytes <= 0 {
		maxBytes = 1 << 20
	}
	if len(input) > maxBytes {
		return nil, domain.Invalidf("credentials.rewrite", "request exceeds %d bytes", maxBytes)
	}
	if !bytes.Contains(input, []byte(Placeholder)) {
		return nil, domain.Invalidf("credentials.rewrite", "placeholder is required")
	}
	material, _, err := r.Broker.Resolve(ctx, handle, endpoint)
	if err != nil {
		return nil, err
	}
	if target == RewriteHeader && (bytes.ContainsAny(material, "\r\n\x00")) {
		return nil, domain.Invalidf("credentials.rewrite", "credential material is unsafe in a header")
	}
	if target == RewriteBody && bytes.IndexByte(material, 0) >= 0 {
		return nil, domain.Invalidf("credentials.rewrite", "credential material contains NUL")
	}
	return bytes.ReplaceAll(input, []byte(Placeholder), material), nil
}

// HasPlaceholder reports whether input contains the credential placeholder.
func HasPlaceholder(input []byte) bool {
	return bytes.Contains(input, []byte(Placeholder))
}

// Redact replaces any placeholder and known material with a stable marker. It
// is for logs and diagnostics only.
func Redact(input []byte) []byte {
	return []byte(strings.ReplaceAll(string(input), Placeholder, "[credential]"))
}

var credentialPattern = regexp.MustCompile(regexp.QuoteMeta(Placeholder))

// CountPlaceholders returns the number of placeholders in input.
func CountPlaceholders(input []byte) int {
	return len(credentialPattern.FindAllIndex(input, -1))
}
