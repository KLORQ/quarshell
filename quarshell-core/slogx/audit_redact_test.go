package slogx

import (
	"strings"
	"testing"
)

func TestAuditTextRedactsInlineSecrets(t *testing.T) {
	input := "request /path?session=abc123 denied: authorization=secret Bearer abc123"
	got := RedactAuditText(input)
	for _, secret := range []string{"abc123", "secret"} {
		if strings.Contains(got, secret) {
			t.Fatalf("audit text leaked %q: %s", secret, got)
		}
	}
	if !strings.Contains(got, "denied") {
		t.Fatalf("audit reason lost: %s", got)
	}
}

func TestTokenFingerprintIsOptIn(t *testing.T) {
	secret := "long-sensitive-token-material"
	if got := (&CorporateMasker{}).Mask(secret, MaskToken); got != "[TOKEN]" {
		t.Fatalf("default token masking: %v", got)
	}
	if got := (&CorporateMasker{Fingerprint: true}).Mask(secret, MaskToken); !strings.HasPrefix(got.(string), "[TOKEN:") {
		t.Fatalf("explicit fingerprint missing: %v", got)
	}
}
