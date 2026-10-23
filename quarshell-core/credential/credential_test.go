package credential

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/KLORQ/quarshell-core/domain"
)

func TestSecretValueIsRedacted(t *testing.T) {
	secret := NewSecretValue([]byte("canary-secret"))
	if secret.String() != "[REDACTED]" {
		t.Fatalf("String leaked secret: %q", secret.String())
	}
	if _, err := json.Marshal(secret); err == nil {
		t.Fatalf("secret must not be JSON-serializable")
	}
	if got := string(secret.Reveal()); got != "canary-secret" {
		t.Fatalf("Reveal = %q", got)
	}
	message := Redact("Authorization: Bearer abc123 api_key=xyz password=hunter2")
	if message != "Authorization: Bearer [REDACTED] api_key=[REDACTED] password=[REDACTED]" {
		t.Fatalf("Redact = %q", message)
	}
}

func TestCredentialLeaseValidity(t *testing.T) {
	now := time.Now()
	lease := CredentialLease{
		Handle: "h", Workspace: "ws", Sandbox: "sb", Instance: "in",
		Provider: "openai", Credential: "api_key", AttachmentEpoch: "1",
		PolicyGeneration: 1, ExpiresAt: now.Add(time.Minute),
	}
	if !lease.Valid(now) {
		t.Fatalf("valid lease rejected")
	}
	if lease.Valid(now.Add(2 * time.Minute)) {
		t.Fatalf("expired lease accepted")
	}
	if (CredentialLease{ExpiresAt: now.Add(time.Minute)}).Valid(now) {
		t.Fatalf("incomplete lease accepted")
	}
	_ = domain.Generation(1)
}
