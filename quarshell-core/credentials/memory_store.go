package credentials

import (
	"context"
	"sync"
	"time"

	"github.com/KLORQ/quarshell-core/domain"
)

type memoryEntry struct {
	secret     Secret
	nonce      []byte
	ciphertext []byte
}

// MemoryStore is an AES-GCM encrypted in-memory store. It is intended for
// tests and single-process development; durable backends live in M4 drivers.
type MemoryStore struct {
	mu      sync.Mutex
	box     *cipherBox
	entries map[domain.CredentialID]memoryEntry
}

// NewMemoryStore returns an encrypted store. key must be 16, 24, or 32 bytes.
func NewMemoryStore(key []byte) (*MemoryStore, error) {
	box, err := newCipherBox(key)
	if err != nil {
		return nil, err
	}
	return &MemoryStore{box: box, entries: map[domain.CredentialID]memoryEntry{}}, nil
}

func (s *MemoryStore) Put(ctx context.Context, secret Secret) (uint64, error) {
	if err := ctx.Err(); err != nil {
		return 0, domain.Wrap(classifyContextError(err), "credentials.store", "put canceled")
	}
	if secret.ID == "" || secret.Provider == "" {
		return 0, domain.Invalidf("credentials.store", "credential id and provider are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.entries[secret.ID]; exists {
		return 0, domain.New(domain.CodeConflict, "credentials.store", "credential already exists")
	}
	nonce, ciphertext, err := s.box.seal(secret.Material)
	if err != nil {
		return 0, err
	}
	secret.Material = nil
	secret.Revision = 1
	s.entries[secret.ID] = memoryEntry{secret: secret, nonce: nonce, ciphertext: ciphertext}
	return 1, nil
}

func (s *MemoryStore) Get(ctx context.Context, id domain.CredentialID) (Secret, error) {
	if err := ctx.Err(); err != nil {
		return Secret{}, domain.Wrap(classifyContextError(err), "credentials.store", "get canceled")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, exists := s.entries[id]
	if !exists {
		return Secret{}, domain.Wrap(domain.ErrNotFound, "credentials.store", "credential not found")
	}
	material, err := s.box.open(entry.nonce, entry.ciphertext)
	if err != nil {
		return Secret{}, err
	}
	secret := entry.secret
	secret.Material = material
	return secret, nil
}

func (s *MemoryStore) Rotate(ctx context.Context, id domain.CredentialID, material []byte, expiresAt time.Time) (Secret, error) {
	if err := ctx.Err(); err != nil {
		return Secret{}, domain.Wrap(classifyContextError(err), "credentials.store", "rotate canceled")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, exists := s.entries[id]
	if !exists {
		return Secret{}, domain.Wrap(domain.ErrNotFound, "credentials.store", "credential not found")
	}
	nonce, ciphertext, err := s.box.seal(material)
	if err != nil {
		return Secret{}, err
	}
	entry.nonce = nonce
	entry.ciphertext = ciphertext
	entry.secret.Revision++
	entry.secret.ExpiresAt = expiresAt
	entry.secret.Material = nil
	s.entries[id] = entry
	secret := entry.secret
	secret.Material = append([]byte(nil), material...)
	return secret, nil
}

func (s *MemoryStore) Delete(ctx context.Context, id domain.CredentialID) error {
	if err := ctx.Err(); err != nil {
		return domain.Wrap(classifyContextError(err), "credentials.store", "delete canceled")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.entries[id]; !exists {
		return domain.Wrap(domain.ErrNotFound, "credentials.store", "credential not found")
	}
	delete(s.entries, id)
	return nil
}
