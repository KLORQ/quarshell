// Package main demonstrates sanitized gRPC call / response / stream logging.
package main

import (
	"context"

	"github.com/KLORQ/quarshell-core/slogx"
)

type CreateUserRequest struct {
	Email string
	Name  string
}

type CreateUserResponse struct {
	ID string
}

func main() {
	log := slogx.New(
		slogx.WithFormat(slogx.FormatJSON),
		slogx.WithMaskRules(
			slogx.NewMaskRules().Add("Email", slogx.MaskEmail),
		),
	)

	req := CreateUserRequest{Email: "bob@example.com", Name: "Bob"}
	md := slogx.Metadata{
		"authorization": {"Bearer grpc-secret"},
		"x-request-id":  {"req-grpc-1"},
	}

	ctx := context.Background()
	log.InfoContext(ctx, "grpc unary call",
		slogx.GRPCCallWith(log, "grpc", "/users.UserService/Create", req, md),
	)

	resp := CreateUserResponse{ID: "user-99"}
	log.InfoContext(ctx, "grpc unary response",
		slogx.GRPCResponseWith(log, "grpc", resp),
	)

	log.InfoContext(ctx, "grpc stream event",
		slogx.GRPCStreamEventWith(log, "grpc", "/users.UserService/Watch", "recv",
			map[string]any{"event": "created"}, md),
	)
}
