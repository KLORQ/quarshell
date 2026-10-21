// SPDX-FileCopyrightText: Copyright (c) 2025-2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package openshell

import (
	"fmt"

	"google.golang.org/protobuf/types/known/structpb"
)

// ValidateRegexMiddlewareConfig enforces the pinned openshell/regex config
// contract. An omitted mode defaults to redact; no other fields or modes are
// accepted.
func ValidateRegexMiddlewareConfig(config *structpb.Struct) error {
	if config == nil {
		return nil
	}
	fields := config.GetFields()
	for key, value := range fields {
		if key != "mode" || value == nil || value.GetKind() == nil {
			return fmt.Errorf("invalid %s config: unsupported field %q; only mode: redact is supported", RegexMiddlewareName, key)
		}
		mode, ok := value.GetKind().(*structpb.Value_StringValue)
		if !ok || mode.StringValue != "redact" {
			return fmt.Errorf("invalid %s config: only mode: redact is supported", RegexMiddlewareName)
		}
	}
	return nil
}
