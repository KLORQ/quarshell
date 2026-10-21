package klorq

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"strings"

	gatewayv1 "github.com/KLORQ/quarshell-proto/gen/go/openshell/gateway/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

type gatewayCredentials struct {
	token     func() string
	edgeToken string
}

func (c gatewayCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	if c.token != nil {
		if token := c.token(); token != "" {
			return map[string]string{"authorization": "Bearer " + token}, nil
		}
	}
	if c.edgeToken != "" {
		return map[string]string{
			"cf-access-jwt-assertion": c.edgeToken,
			"cookie":                  "CF_Authorization=" + c.edgeToken,
		}, nil
	}
	return nil, nil
}

// gatewayConnection creates the generated OpenShell transport. The host alias
// is injected by the CLI into sandbox proxy environments as a trusted route.
func (c *Client) gatewayConnection() (*grpc.ClientConn, error) {
	c.rpcMu.Lock()
	defer c.rpcMu.Unlock()
	if c.gatewayConn != nil {
		return c.gatewayConn, nil
	}
	endpoint, err := url.Parse(c.base)
	if err != nil || endpoint.Host == "" || endpoint.Path != "" || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return nil, fmt.Errorf("gateway RPC: base URL must contain only scheme and authority")
	}
	var transport credentials.TransportCredentials
	switch strings.ToLower(endpoint.Scheme) {
	case "https":
		transport = credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: c.insecureTLS})
	case "http":
		host := strings.ToLower(endpoint.Hostname())
		if !isLoopbackHost(host) && host != "host.cautem.internal" {
			return nil, fmt.Errorf("gateway RPC: unencrypted gRPC is allowed only for local gateway routes")
		}
		transport = insecure.NewCredentials()
	default:
		return nil, fmt.Errorf("gateway RPC: unsupported URL scheme %q", endpoint.Scheme)
	}
	conn, err := grpc.NewClient(endpoint.Host,
		grpc.WithTransportCredentials(transport),
		grpc.WithPerRPCCredentials(gatewayCredentials{token: c.authToken, edgeToken: c.edgeToken}),
	)
	if err != nil {
		return nil, fmt.Errorf("gateway RPC: connect: %w", err)
	}
	c.gatewayConn = conn
	return conn, nil
}

func (c *Client) gatewayClient() (gatewayv1.OpenShellClient, error) {
	conn, err := c.gatewayConnection()
	if err != nil {
		return nil, err
	}
	return gatewayv1.NewOpenShellClient(conn), nil
}

func (gatewayCredentials) RequireTransportSecurity() bool { return false }

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
