package credentials

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync"
	"time"

	"github.com/KLORQ/quarshell-core/domain"
)

// Handle is an opaque credential reference. The material is never embedded in
// the handle.
type Handle string

// HandleInfo is the non-secret metadata bound to a handle.
type HandleInfo struct {
	Handle     Handle
	Credential domain.CredentialID
	Provider   domain.ProviderID
	Sandbox    domain.SandboxRef
	Endpoint   EndpointBinding
	ExpiresAt  time.Time
	Revision   uint64
	StableKey  string
}

// IssueRequest binds one credential to a sandbox endpoint.
type IssueRequest struct {
	Credential domain.CredentialID
	Sandbox    domain.SandboxRef
	Endpoint   EndpointBinding
	StableKey  string
	TTL        time.Duration
}

// Broker issues and resolves endpoint-bound handles.
type Broker struct {
	store Store
	now   func() time.Time

	mu      sync.Mutex
	handles map[Handle]HandleInfo
	byKey   map[string]Handle
}

// NewBroker returns a broker over store.
func NewBroker(store Store, now func() time.Time) (*Broker, error) {
	if store == nil {
		return nil, domain.Invalidf("credentials.broker", "store is required")
	}
	if now == nil {
		now = time.Now
	}
	return &Broker{store: store, now: now, handles: map[Handle]HandleInfo{}, byKey: map[string]Handle{}}, nil
}

// Issue returns a stable handle when StableKey is reusable, otherwise a new
// handle bound to the sandbox and endpoint.
func (b *Broker) Issue(ctx context.Context, request IssueRequest) (Handle, HandleInfo, error) {
	if err := ctx.Err(); err != nil {
		return "", HandleInfo{}, domain.Wrap(classifyContextError(err), "credentials.broker", "issue canceled")
	}
	if request.Credential == "" {
		return "", HandleInfo{}, domain.Invalidf("credentials.broker", "credential is required")
	}
	if !request.Sandbox.Valid() {
		return "", HandleInfo{}, domain.Invalidf("credentials.broker", "sandbox reference is invalid")
	}
	if strings.TrimSpace(request.Endpoint.Host) == "" || request.Endpoint.Port == 0 {
		return "", HandleInfo{}, domain.Invalidf("credentials.broker", "endpoint host and port are required")
	}
	secret, err := b.store.Get(ctx, request.Credential)
	if err != nil {
		return "", HandleInfo{}, err
	}
	now := b.now()
	expiresAt := secret.ExpiresAt
	if request.TTL > 0 {
		candidate := now.Add(request.TTL)
		if expiresAt.IsZero() || candidate.Before(expiresAt) {
			expiresAt = candidate
		}
	}
	if !expiresAt.After(now) {
		return "", HandleInfo{}, domain.Wrap(domain.ErrUnauthorized, "credentials.broker", "credential is expired")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if request.StableKey != "" {
		if existing, ok := b.byKey[request.StableKey]; ok {
			info, exists := b.handles[existing]
			if exists && info.Revision == secret.Revision && info.ExpiresAt.After(now) && sameBinding(info.Endpoint, request.Endpoint) {
				return existing, info, nil
			}
			delete(b.handles, existing)
			delete(b.byKey, request.StableKey)
		}
	}
	handle, err := newHandle()
	if err != nil {
		return "", HandleInfo{}, err
	}
	info := HandleInfo{
		Handle: handle, Credential: request.Credential, Provider: secret.Provider,
		Sandbox: request.Sandbox, Endpoint: request.Endpoint, ExpiresAt: expiresAt,
		Revision: secret.Revision, StableKey: request.StableKey,
	}
	b.handles[handle] = info
	if request.StableKey != "" {
		b.byKey[request.StableKey] = handle
	}
	return handle, info, nil
}

// Resolve returns material only for the exact bound endpoint and a current
// credential revision.
func (b *Broker) Resolve(ctx context.Context, handle Handle, endpoint EndpointBinding) ([]byte, HandleInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, HandleInfo{}, domain.Wrap(classifyContextError(err), "credentials.broker", "resolve canceled")
	}
	b.mu.Lock()
	info, exists := b.handles[handle]
	b.mu.Unlock()
	if !exists {
		return nil, HandleInfo{}, domain.Wrap(domain.ErrNotFound, "credentials.broker", "credential handle not found")
	}
	if !endpointMatches(info.Endpoint, endpoint) {
		return nil, HandleInfo{}, domain.Wrap(domain.ErrPermissionDenied, "credentials.broker", "credential is not bound to this endpoint")
	}
	if !info.ExpiresAt.After(b.now()) {
		b.Revoke(ctx, handle)
		return nil, HandleInfo{}, domain.Wrap(domain.ErrNotFound, "credentials.broker", "credential handle expired")
	}
	secret, err := b.store.Get(ctx, info.Credential)
	if err != nil {
		return nil, HandleInfo{}, err
	}
	if secret.Revision != info.Revision {
		return nil, HandleInfo{}, domain.New(domain.CodeConflict, "credentials.broker", "credential rotated; issue a new handle")
	}
	if !secret.ExpiresAt.IsZero() && !secret.ExpiresAt.After(b.now()) {
		b.Revoke(ctx, handle)
		return nil, HandleInfo{}, domain.Wrap(domain.ErrNotFound, "credentials.broker", "credential expired")
	}
	return append([]byte(nil), secret.Material...), info, nil
}

// Revoke removes a handle. It is idempotent.
func (b *Broker) Revoke(_ context.Context, handle Handle) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	info, exists := b.handles[handle]
	if !exists {
		return nil
	}
	delete(b.handles, handle)
	if info.StableKey != "" && b.byKey[info.StableKey] == handle {
		delete(b.byKey, info.StableKey)
	}
	return nil
}

// Rotate replaces material and invalidates every handle for the credential.
func (b *Broker) Rotate(ctx context.Context, id domain.CredentialID, material []byte, expiresAt time.Time) (uint64, error) {
	secret, err := b.store.Rotate(ctx, id, material, expiresAt)
	if err != nil {
		return 0, err
	}
	b.mu.Lock()
	for handle, info := range b.handles {
		if info.Credential == id {
			delete(b.handles, handle)
			if info.StableKey != "" && b.byKey[info.StableKey] == handle {
				delete(b.byKey, info.StableKey)
			}
		}
	}
	b.mu.Unlock()
	return secret.Revision, nil
}

func newHandle() (Handle, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", domain.Wrap(classifyContextError(err), "credentials.broker", "generate handle")
	}
	return Handle("ch_" + hex.EncodeToString(raw)), nil
}

func sameBinding(left, right EndpointBinding) bool {
	return strings.EqualFold(left.Scheme, right.Scheme) && strings.EqualFold(left.Host, right.Host) &&
		left.Port == right.Port && strings.EqualFold(left.Method, right.Method) && left.PathPrefix == right.PathPrefix
}

func endpointMatches(bound, requested EndpointBinding) bool {
	if bound.Scheme != "" && !strings.EqualFold(bound.Scheme, requested.Scheme) {
		return false
	}
	if bound.Port != 0 && bound.Port != requested.Port {
		return false
	}
	if bound.Method != "" && !strings.EqualFold(bound.Method, requested.Method) {
		return false
	}
	if bound.PathPrefix != "" && !strings.HasPrefix(requested.PathPrefix, bound.PathPrefix) {
		return false
	}
	return hostMatches(bound.Host, requested.Host)
}

func hostMatches(bound, requested string) bool {
	bound = strings.ToLower(bound)
	requested = strings.ToLower(requested)
	switch {
	case bound == requested:
		return true
	case strings.HasPrefix(bound, "**."):
		return strings.HasSuffix(requested, bound[2:]) && len(requested) > len(bound)-2
	case strings.HasPrefix(bound, "*."):
		suffix := bound[1:]
		return strings.HasSuffix(requested, suffix) && !strings.Contains(strings.TrimSuffix(requested, suffix), ".")
	case bound == "*":
		return true
	default:
		return false
	}
}
