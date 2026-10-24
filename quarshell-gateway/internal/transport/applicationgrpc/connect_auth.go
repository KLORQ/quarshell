package applicationgrpc

import (
	"context"
	"errors"
	"net/http"

	"connectrpc.com/connect"
	tokenapp "github.com/KLORQ/quarshell-gateway/internal/application/token"
	gatewayv1connect "github.com/KLORQ/quarshell-proto/gen/go/openshell/gateway/v1/gatewayv1connect"
	"google.golang.org/grpc/metadata"
)

// ConnectAuthInterceptor authenticates Connect and gRPC-Web requests using
// the same operator bearer and sandbox-token rules as the native gRPC API.
func ConnectAuthInterceptor(operatorToken, operatorSubject string, tokens *tokenapp.Service) connect.Interceptor {
	return connectStreamingAuth{operatorToken: operatorToken, operatorSubject: operatorSubject, tokens: tokens}
}

type connectStreamingAuth struct {
	operatorToken   string
	operatorSubject string
	tokens          *tokenapp.Service
}

func (i connectStreamingAuth) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
		if request.Spec().Procedure == gatewayv1connect.OpenShellHealthProcedure {
			return next(ctx, request)
		}
		ctx = connectMetadataContext(ctx, request.Header())
		authenticated, err := withAuthenticatedPrincipal(ctx, i.operatorToken, i.operatorSubject)
		if err != nil && i.tokens != nil && isSandboxUnaryMethod(request.Spec().Procedure) {
			authenticated, err = withSandboxProtocolPrincipal(ctx, i.tokens)
		}
		if err != nil {
			return nil, connectAuthError()
		}
		return next(authenticated, request)
	}
}
func (i connectStreamingAuth) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}
func (i connectStreamingAuth) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, stream connect.StreamingHandlerConn) error {
		if stream.Spec().Procedure == gatewayv1connect.OpenShellWatchSandboxProcedure {
			ctx = connectMetadataContext(ctx, stream.RequestHeader())
			if _, err := withAuthenticatedPrincipal(ctx, i.operatorToken, i.operatorSubject); err == nil {
				return next(ctx, stream)
			}
			return connectAuthError()
		}
		ctx = connectMetadataContext(ctx, stream.RequestHeader())
		if _, err := withAuthenticatedPrincipal(ctx, i.operatorToken, i.operatorSubject); err == nil {
			return next(ctx, stream)
		}
		procedure := stream.Spec().Procedure
		if i.tokens != nil && (procedure == gatewayv1connect.OpenShellConnectSupervisorProcedure || procedure == gatewayv1connect.OpenShellRelayStreamProcedure || procedure == gatewayv1connect.OpenShellPushSandboxLogsProcedure) {
			if authenticated, err := withSandboxProtocolPrincipal(ctx, i.tokens); err == nil {
				return next(authenticated, stream)
			}
		}
		return connectAuthError()
	}
}

// ConnectAuthInterceptors returns unary and streaming authentication handlers
// as one Connect interceptor.
func connectMetadataContext(ctx context.Context, header http.Header) context.Context {
	md := metadata.MD{}
	if values := header.Values("Authorization"); len(values) > 0 {
		md["authorization"] = append([]string(nil), values...)
	}
	if values := header.Values("X-Quarshell-Token"); len(values) > 0 {
		md["x-quarshell-token"] = append([]string(nil), values...)
	}
	return metadata.NewIncomingContext(ctx, md)
}

func connectAuthError() error {
	return connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
}
