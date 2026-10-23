package credentials

import (
	"bytes"
	"context"
	"testing"
)

func TestMemoryStoreEncryptsAtRest(t *testing.T) {
	store, err := NewMemoryStore([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("NewMemoryStore: %v", err)
	}
	if _, err := store.Put(context.Background(), Secret{ID: "cred-1", Provider: "test", Material: []byte("super-secret-token")}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	entry := store.entries["cred-1"]
	if bytes.Contains(entry.ciphertext, []byte("super-secret-token")) {
		t.Fatalf("ciphertext contains plaintext")
	}
	secret, err := store.Get(context.Background(), "cred-1")
	if err != nil || string(secret.Material) != "super-secret-token" {
		t.Fatalf("Get = %q %v", secret.Material, err)
	}
}
