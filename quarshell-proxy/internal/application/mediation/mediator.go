// Package mediation implements the default-deny egress decision path. It uses
// the authored policy snapshot and credential broker; transport adapters call
// it before dialing an upstream.
package mediation

import (
	"context"
	"strings"

	"github.com/KLORQ/quarshell-core/credentials"
	"github.com/KLORQ/quarshell-core/domain"
	"github.com/KLORQ/quarshell-policy"
)

// Request is one normalized egress request.
type Request struct {
	Scheme           string
	Protocol         string
	Host             string
	Port             uint16
	Method           string
	Path             string
	Command          string
	Query            map[string]string
	OperationType    string
	OperationName    string
	Fields           []string
	Tool             string
	Params           map[string]any
	Body             []byte
	Headers          map[string]string
	Credential       credentials.Handle
	CredentialHeader string
	CredentialPrefix string
	Rewrite          credentials.RewriteTarget
}

// Response is the upstream result.
type Response struct {
	Status  int
	Body    []byte
	Headers map[string]string
}

// PolicySource loads the active policy snapshot.
type PolicySource interface {
	Current(context.Context) (*policy.Snapshot, error)
}

// Dialer performs the upstream request after mediation.
type Dialer interface {
	Do(context.Context, Request) (Response, error)
}

// Mediator enforces default-deny and credential-safe rewriting.
type Mediator struct {
	Policies     PolicySource
	Rewriter     *credentials.Rewriter
	Dialer       Dialer
	MaxBodyBytes int
}

// Evaluate applies policy only. It is used by transports that need a decision
// before performing their own network operation (for example CONNECT tunnels).
func (m *Mediator) Evaluate(ctx context.Context, request Request) error {
	if err := ctx.Err(); err != nil {
		return domain.Wrap(domain.ErrCanceled, "mediation.evaluate", "request canceled")
	}
	if m.Policies == nil {
		return domain.Invalidf("mediation.evaluate", "policy source is required")
	}
	if strings.TrimSpace(request.Host) == "" || request.Port == 0 {
		return domain.Invalidf("mediation.evaluate", "host and port are required")
	}
	snapshot, err := m.Policies.Current(ctx)
	if err != nil {
		return err
	}
	if snapshot == nil {
		return domain.Wrap(domain.ErrPermissionDenied, "mediation.evaluate", "no active policy")
	}
	decision := snapshot.EvaluateNetwork(policy.NetworkRequest{
		Host: request.Host, Port: request.Port, Protocol: request.Protocol,
		Method: request.Method, Path: request.Path, Command: request.Command, Query: request.Query,
		OperationType: request.OperationType, OperationName: request.OperationName,
		Fields: request.Fields, Tool: request.Tool, Params: request.Params,
	})
	switch decision.Effect {
	case policy.EffectDeny:
		return domain.Wrap(domain.ErrPermissionDenied, "mediation.evaluate", "egress denied: "+decision.Reason)
	case policy.EffectUnsupported:
		return domain.Wrap(domain.ErrUnsupported, "mediation.evaluate", "egress policy cannot be evaluated")
	case policy.EffectAllow:
		return nil
	default:
		return domain.Wrap(domain.ErrUnsupported, "mediation.evaluate", "unknown policy effect")
	}
}

// Do evaluates policy, optionally rewrites a bound credential, then dials.
func (m *Mediator) Do(ctx context.Context, request Request) (Response, error) {
	if err := ctx.Err(); err != nil {
		return Response{}, domain.Wrap(domain.ErrCanceled, "mediation.do", "request canceled")
	}
	if m.Dialer == nil {
		return Response{}, domain.Invalidf("mediation.do", "dialer is required")
	}
	maxBytes := m.MaxBodyBytes
	if maxBytes <= 0 {
		maxBytes = 1 << 20
	}
	if len(request.Body) > maxBytes {
		return Response{}, domain.Invalidf("mediation.do", "request exceeds %d bytes", maxBytes)
	}
	if err := m.Evaluate(ctx, request); err != nil {
		return Response{}, err
	}
	if request.Credential != "" {
		if m.Rewriter == nil || m.Rewriter.Broker == nil {
			return Response{}, domain.Invalidf("mediation.do", "credential rewrite requested without a broker")
		}
		scheme := strings.ToLower(request.Scheme)
		if scheme == "" {
			scheme = strings.ToLower(request.Protocol)
		}
		binding := credentials.EndpointBinding{
			Scheme: scheme, Host: request.Host, Port: request.Port,
			Method: request.Method, PathPrefix: request.Path,
		}
		if request.CredentialHeader != "" {
			if strings.ContainsAny(request.CredentialHeader, ":\r\n") {
				return Response{}, domain.Invalidf("mediation.do", "credential header name is invalid")
			}
			material, _, err := m.Rewriter.Broker.Resolve(ctx, request.Credential, binding)
			if err != nil {
				return Response{}, err
			}
			prefix := request.CredentialPrefix
			if prefix == "" {
				prefix = "Bearer "
			}
			if request.Headers == nil {
				request.Headers = map[string]string{}
			}
			request.Headers[request.CredentialHeader] = prefix + string(material)
		} else {
			if len(request.Body) == 0 {
				return Response{}, domain.Invalidf("mediation.do", "credential rewrite requires a body")
			}
			body, err := m.Rewriter.Rewrite(ctx, request.Credential, binding, request.Body, request.Rewrite)
			if err != nil {
				return Response{}, err
			}
			request.Body = body
		}
	}
	return m.Dialer.Do(ctx, request)
}
