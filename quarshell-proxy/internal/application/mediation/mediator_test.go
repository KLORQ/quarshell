package mediation

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/KLORQ/quarshell-core/credentials"
	"github.com/KLORQ/quarshell-core/domain"
	"github.com/KLORQ/quarshell-policy"
)

const mediationPolicy = `version: 1
network_policies:
  api:
    endpoints:
      - host: api.example.com
        port: 443
        protocol: rest
        rules:
          - allow:
              method: POST
              path: /v1/*
      - host: other.example.com
        port: 443
        protocol: rest
        rules:
          - allow:
              method: POST
              path: /v1/*
`

type staticPolicy struct{ snapshot *policy.Snapshot }

func (s staticPolicy) Current(context.Context) (*policy.Snapshot, error) { return s.snapshot, nil }

type recordingDialer struct {
	calls int
	last  Request
	err   error
}

func (d *recordingDialer) Do(_ context.Context, request Request) (Response, error) {
	d.calls++
	d.last = request
	if d.err != nil {
		return Response{}, d.err
	}
	return Response{Status: 200, Body: []byte("ok")}, nil
}

func snapshot(t *testing.T) *policy.Snapshot {
	t.Helper()
	document, err := policy.Parse(strings.NewReader(mediationPolicy), policy.DefaultLimits())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	snapshot, err := document.Snapshot(1, time.Now())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	return snapshot
}

func TestMediatorDefaultDeny(t *testing.T) {
	dialer := &recordingDialer{}
	mediator := &Mediator{Policies: staticPolicy{snapshot: snapshot(t)}, Dialer: dialer}
	_, err := mediator.Do(context.Background(), Request{Protocol: "rest", Host: "evil.example.com", Port: 443, Method: "POST", Path: "/v1/x"})
	if !errors.Is(err, domain.ErrPermissionDenied) {
		t.Fatalf("deny error = %v", err)
	}
	if dialer.calls != 0 {
		t.Fatalf("dialer called on deny")
	}
}

func TestMediatorAllowsAndRewritesCredential(t *testing.T) {
	store, err := credentials.NewMemoryStore([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("NewMemoryStore: %v", err)
	}
	secret := credentials.Secret{
		ID: "cred", Provider: "api", Material: []byte("token-123"),
		Endpoint:  credentials.EndpointBinding{Scheme: "rest", Host: "api.example.com", Port: 443, Method: "POST", PathPrefix: "/v1"},
		ExpiresAt: time.Now().Add(time.Hour),
	}
	if _, err := store.Put(context.Background(), secret); err != nil {
		t.Fatalf("Put: %v", err)
	}
	broker, _ := credentials.NewBroker(store, nil)
	handle, _, err := broker.Issue(context.Background(), credentials.IssueRequest{
		Credential: "cred", Sandbox: domain.SandboxRef{Workspace: "ws", Sandbox: "sb"}, Endpoint: secret.Endpoint,
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	dialer := &recordingDialer{}
	mediator := &Mediator{
		Policies: staticPolicy{snapshot: snapshot(t)},
		Rewriter: &credentials.Rewriter{Broker: broker},
		Dialer:   dialer,
	}
	response, err := mediator.Do(context.Background(), Request{
		Protocol: "rest", Host: "api.example.com", Port: 443, Method: "POST", Path: "/v1/x",
		Body: []byte("authorization=" + credentials.Placeholder), Credential: handle,
	})
	if err != nil || response.Status != 200 {
		t.Fatalf("Do = %+v %v", response, err)
	}
	if !strings.Contains(string(dialer.last.Body), "token-123") || strings.Contains(string(dialer.last.Body), credentials.Placeholder) {
		t.Fatalf("body = %q", dialer.last.Body)
	}
}

func TestMediatorRejectsWrongCredentialBinding(t *testing.T) {
	store, _ := credentials.NewMemoryStore([]byte("0123456789abcdef0123456789abcdef"))
	secret := credentials.Secret{
		ID: "cred", Provider: "api", Material: []byte("token"),
		Endpoint:  credentials.EndpointBinding{Scheme: "rest", Host: "api.example.com", Port: 443, Method: "POST"},
		ExpiresAt: time.Now().Add(time.Hour),
	}
	store.Put(context.Background(), secret)
	broker, _ := credentials.NewBroker(store, nil)
	handle, _, _ := broker.Issue(context.Background(), credentials.IssueRequest{
		Credential: "cred", Sandbox: domain.SandboxRef{Workspace: "ws", Sandbox: "sb"}, Endpoint: secret.Endpoint,
	})
	mediator := &Mediator{
		Policies: staticPolicy{snapshot: snapshot(t)},
		Rewriter: &credentials.Rewriter{Broker: broker},
		Dialer:   &recordingDialer{},
	}
	_, err := mediator.Do(context.Background(), Request{
		Protocol: "rest", Host: "other.example.com", Port: 443, Method: "POST", Path: "/v1/x",
		Body: []byte(credentials.Placeholder), Credential: handle,
	})
	if !errors.Is(err, domain.ErrPermissionDenied) {
		t.Fatalf("wrong binding = %v", err)
	}
}

func TestMediatorBoundsAndCancellation(t *testing.T) {
	mediator := &Mediator{Policies: staticPolicy{snapshot: snapshot(t)}, Dialer: &recordingDialer{}, MaxBodyBytes: 4}
	if _, err := mediator.Do(context.Background(), Request{Protocol: "rest", Host: "api.example.com", Port: 443, Method: "POST", Path: "/v1/x", Body: []byte("12345")}); err == nil {
		t.Fatalf("oversized body accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := mediator.Do(ctx, Request{Protocol: "rest", Host: "api.example.com", Port: 443, Method: "POST", Path: "/v1/x"}); !errors.Is(err, domain.ErrCanceled) {
		t.Fatalf("canceled = %v", err)
	}
}

func TestM4CreateAttachRequestRotateDetachDeny(t *testing.T) {
	store, err := credentials.NewMemoryStore([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("NewMemoryStore: %v", err)
	}
	endpoint := credentials.EndpointBinding{Scheme: "rest", Host: "api.example.com", Port: 443, Method: "POST", PathPrefix: "/v1"}
	secret := credentials.Secret{ID: "cred", Provider: "api", Material: []byte("v1-token"), Endpoint: endpoint, ExpiresAt: time.Now().Add(time.Hour)}
	if _, err := store.Put(context.Background(), secret); err != nil {
		t.Fatalf("Put: %v", err)
	}
	broker, _ := credentials.NewBroker(store, nil)
	dialer := &recordingDialer{}
	mediator := &Mediator{Policies: staticPolicy{snapshot: snapshot(t)}, Rewriter: &credentials.Rewriter{Broker: broker}, Dialer: dialer}
	ref := domain.SandboxRef{Workspace: "ws", Sandbox: "sb"}

	// attach
	handle, _, err := broker.Issue(context.Background(), credentials.IssueRequest{Credential: "cred", Sandbox: ref, Endpoint: endpoint})
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	request := Request{Protocol: "rest", Host: "api.example.com", Port: 443, Method: "POST", Path: "/v1/x", Body: []byte("token=" + credentials.Placeholder), Credential: handle}
	if _, err := mediator.Do(context.Background(), request); err != nil || !strings.Contains(string(dialer.last.Body), "v1-token") {
		t.Fatalf("request = %v body=%q", err, dialer.last.Body)
	}

	// rotate invalidates the old handle
	if _, err := broker.Rotate(context.Background(), "cred", []byte("v2-token"), time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if _, err := mediator.Do(context.Background(), request); err == nil {
		t.Fatalf("old handle survived rotation")
	}
	newHandle, _, err := broker.Issue(context.Background(), credentials.IssueRequest{Credential: "cred", Sandbox: ref, Endpoint: endpoint})
	if err != nil {
		t.Fatalf("re-attach: %v", err)
	}
	request.Credential = newHandle
	if _, err := mediator.Do(context.Background(), request); err != nil || !strings.Contains(string(dialer.last.Body), "v2-token") {
		t.Fatalf("post-rotate request = %v body=%q", err, dialer.last.Body)
	}

	// detach
	if err := broker.Revoke(context.Background(), newHandle); err != nil {
		t.Fatalf("detach: %v", err)
	}
	if _, err := mediator.Do(context.Background(), request); err == nil {
		t.Fatalf("revoked handle accepted")
	}

	// default deny
	if _, err := mediator.Do(context.Background(), Request{Protocol: "rest", Host: "evil.example.com", Port: 443, Method: "POST", Path: "/v1/x"}); !errors.Is(err, domain.ErrPermissionDenied) {
		t.Fatalf("default deny = %v", err)
	}
}
