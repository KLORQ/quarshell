// SPDX-FileCopyrightText: Copyright (c) 2025-2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package openshell contains domain contracts ported from the pinned OpenShell
// source. It has no dependency on gateway or backend implementations.
package openshell

import (
	"fmt"
	"strings"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/protoadapt"
	"google.golang.org/protobuf/types/known/durationpb"
)

// ErrorDomain is the stable ErrorInfo domain used by gateway-owned failures.
const ErrorDomain = "openshell.nvidia.com"

// RPCErrorDetails contains the supported structured details decoded from a
// gRPC status. Unrecognized or malformed details are ignored.
type RPCErrorDetails struct {
	BadRequest *errdetails.BadRequest
	ErrorInfo  *errdetails.ErrorInfo
	RetryInfo  *errdetails.RetryInfo
}

// DecodeRPCErrorDetails rejects inconsistent status envelopes and invalid retry
// delays. The original status remains available to callers when details fail to
// decode.
func DecodeRPCErrorDetails(st *status.Status) *RPCErrorDetails {
	if st == nil {
		return nil
	}
	envelope := st.Proto()
	if envelope == nil || envelope.Code != int32(st.Code()) || envelope.Message != st.Message() {
		return nil
	}
	out := &RPCErrorDetails{}
	for _, detail := range envelope.Details {
		if detail == nil {
			continue
		}
		switch {
		case strings.HasSuffix(detail.TypeUrl, "/google.rpc.BadRequest"):
			value := new(errdetails.BadRequest)
			if detail.UnmarshalTo(value) == nil {
				if out.BadRequest == nil {
					out.BadRequest = new(errdetails.BadRequest)
				}
				out.BadRequest.FieldViolations = append(out.BadRequest.FieldViolations, value.FieldViolations...)
			}
		case strings.HasSuffix(detail.TypeUrl, "/google.rpc.ErrorInfo"):
			value := new(errdetails.ErrorInfo)
			if detail.UnmarshalTo(value) == nil {
				out.ErrorInfo = value
			}
		case strings.HasSuffix(detail.TypeUrl, "/google.rpc.RetryInfo"):
			value := new(errdetails.RetryInfo)
			if detail.UnmarshalTo(value) != nil || value.RetryDelay == nil || value.RetryDelay.CheckValid() != nil {
				continue
			}
			if value.RetryDelay.Seconds < 0 || value.RetryDelay.Nanos < 0 {
				continue
			}
			out.RetryInfo = value
		}
	}
	return out
}

// ComputeDriverErrorKind identifies backend-independent compute failures.
type ComputeDriverErrorKind uint8

const (
	DriverAlreadyExists ComputeDriverErrorKind = iota + 1
	DriverNotFound
	DriverInvalidArgument
	DriverPrecondition
	DriverFailure
)

// ComputeDriverError is the shared driver error contract. Backend-specific
// diagnostics stay in Message while the Kind determines the public gRPC code.
type ComputeDriverError struct {
	Kind    ComputeDriverErrorKind
	Message string
}

func (e *ComputeDriverError) Error() string {
	if e == nil {
		return "compute driver error"
	}
	switch e.Kind {
	case DriverAlreadyExists:
		return "sandbox already exists"
	case DriverNotFound:
		return "sandbox not found"
	default:
		return e.Message
	}
}

// GRPCStatus maps the shared driver error into the status code expected by the
// gateway's public API.
func (e *ComputeDriverError) GRPCStatus() *status.Status {
	if e == nil {
		return status.New(codes.Internal, "compute driver error")
	}
	switch e.Kind {
	case DriverAlreadyExists:
		return status.New(codes.AlreadyExists, "sandbox already exists")
	case DriverNotFound:
		return status.New(codes.NotFound, "sandbox not found")
	case DriverInvalidArgument:
		return status.New(codes.InvalidArgument, e.Message)
	case DriverPrecondition:
		return status.New(codes.FailedPrecondition, e.Message)
	default:
		return status.New(codes.Internal, e.Message)
	}
}

// InvalidArgument returns a field-specific validation status without including
// the rejected field value.
func InvalidArgument(field, message string) error {
	return withDetails(codes.InvalidArgument, message,
		&errdetails.BadRequest{FieldViolations: []*errdetails.BadRequest_FieldViolation{{Field: field, Description: message}}},
		&errdetails.ErrorInfo{Reason: "INVALID_ARGUMENT", Domain: ErrorDomain},
	)
}

// FailedPrecondition returns a failure with a stable machine-readable reason.
func FailedPrecondition(reason, message string) error {
	return withDetails(codes.FailedPrecondition, message,
		&errdetails.ErrorInfo{Reason: reason, Domain: ErrorDomain},
	)
}

// ResourceVersionConflict requires callers to refresh state before a new write.
func ResourceVersionConflict(message string, currentVersion *uint64) error {
	metadata := map[string]string{"recovery": "REFRESH_STATE"}
	if currentVersion != nil {
		metadata["current_resource_version"] = fmt.Sprint(*currentVersion)
	}
	return withDetails(codes.Aborted, message,
		&errdetails.ErrorInfo{Reason: "RESOURCE_VERSION_CONFLICT", Domain: ErrorDomain, Metadata: metadata},
	)
}

// Unavailable returns a transient failure and retry guidance. The caller remains
// responsible for deciding whether its operation is safe to retry.
func Unavailable(reason, message string, delay time.Duration) error {
	if delay < 0 {
		delay = 0
	}
	return withDetails(codes.Unavailable, message,
		&errdetails.ErrorInfo{Reason: reason, Domain: ErrorDomain},
		&errdetails.RetryInfo{RetryDelay: durationpb.New(delay)},
	)
}

func withDetails(code codes.Code, message string, details ...protoadapt.MessageV1) error {
	st := status.New(code, message)
	for _, detail := range details {
		var err error
		st, err = st.WithDetails(detail)
		if err != nil {
			return status.Error(codes.Internal, "failed to attach structured RPC error details")
		}
	}
	return st.Err()
}
