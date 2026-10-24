package applicationgrpc

//go:generate go run ./cmd/genconnectadapter -out connect_adapter_generated.go

import (
	"context"
	"errors"
	"net/http"

	"connectrpc.com/connect"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type connectGRPCStream[Request, Response any] struct {
	ctx            context.Context
	conn           connect.StreamingHandlerConn
	closedResponse *Response
}

func (s *connectGRPCStream[Request, Response]) Send(message *Response) error {
	return s.conn.Send(message)
}

func (s *connectGRPCStream[Request, Response]) Recv() (*Request, error) {
	message := new(Request)
	if err := s.conn.Receive(message); err != nil {
		return nil, err
	}
	return message, nil
}

func (s *connectGRPCStream[Request, Response]) SendAndClose(message *Response) error {
	s.closedResponse = message
	return nil
}

func (s *connectGRPCStream[Request, Response]) SetHeader(values metadata.MD) error {
	copyMetadata(s.conn.ResponseHeader(), values)
	return nil
}

func (s *connectGRPCStream[Request, Response]) SendHeader(values metadata.MD) error {
	copyMetadata(s.conn.ResponseHeader(), values)
	return nil
}

func (s *connectGRPCStream[Request, Response]) SetTrailer(values metadata.MD) {
	copyMetadata(s.conn.ResponseTrailer(), values)
}

func (s *connectGRPCStream[Request, Response]) Context() context.Context { return s.ctx }

func (s *connectGRPCStream[Request, Response]) SendMsg(message any) error {
	return s.conn.Send(message)
}

func (s *connectGRPCStream[Request, Response]) RecvMsg(message any) error {
	return s.conn.Receive(message)
}

var _ grpc.BidiStreamingServer[struct{}, struct{}] = (*connectGRPCStream[struct{}, struct{}])(nil)
var _ grpc.ClientStreamingServer[struct{}, struct{}] = (*connectGRPCStream[struct{}, struct{}])(nil)
var _ grpc.ServerStreamingServer[struct{}] = (*connectGRPCStream[struct{}, struct{}])(nil)

func copyMetadata(destination http.Header, values metadata.MD) {
	for key, entries := range values {
		for _, value := range entries {
			destination.Add(key, value)
		}
	}
}

func connectError(err error) error {
	if err == nil {
		return nil
	}
	code := connect.CodeInternal
	switch status.Code(err) {
	case codes.Canceled:
		code = connect.CodeCanceled
	case codes.InvalidArgument:
		code = connect.CodeInvalidArgument
	case codes.DeadlineExceeded:
		code = connect.CodeDeadlineExceeded
	case codes.NotFound:
		code = connect.CodeNotFound
	case codes.AlreadyExists:
		code = connect.CodeAlreadyExists
	case codes.PermissionDenied:
		code = connect.CodePermissionDenied
	case codes.ResourceExhausted:
		code = connect.CodeResourceExhausted
	case codes.FailedPrecondition:
		code = connect.CodeFailedPrecondition
	case codes.Aborted:
		code = connect.CodeAborted
	case codes.OutOfRange:
		code = connect.CodeOutOfRange
	case codes.Unimplemented:
		code = connect.CodeUnimplemented
	case codes.Unavailable:
		code = connect.CodeUnavailable
	case codes.Unauthenticated:
		code = connect.CodeUnauthenticated
	}
	message := status.Convert(err).Message()
	if message == "" {
		message = "gateway request failed"
	}
	return connect.NewError(code, errors.New(message))
}

func connectMissingStreamResponse(method string) error {
	return connect.NewError(connect.CodeInternal, errors.New(method+" completed without a response"))
}
