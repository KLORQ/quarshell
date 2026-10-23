// SPDX-FileCopyrightText: Copyright (c) 2025-2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package openshell

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
)

// ErrInvalidSandboxSessionID is returned for nil or noncanonical session IDs.
var ErrInvalidSandboxSessionID = errors.New("sandbox session ID must be a non-nil canonical lowercase UUID")

// SandboxSessionID identifies one create or start-from-stopped sandbox launch.
// Retries of the same durable launch reuse the ID; a later launch receives a
// new ID even if the compute backend reuses its resource identifier.
type SandboxSessionID [16]byte

// NewSandboxSessionID creates a cryptographically random UUIDv4 launch ID.
func NewSandboxSessionID() (SandboxSessionID, error) {
	var id SandboxSessionID
	if _, err := rand.Read(id[:]); err != nil {
		return SandboxSessionID{}, fmt.Errorf("generate sandbox session ID: %w", err)
	}
	id[6] = (id[6] & 0x0f) | 0x40
	id[8] = (id[8] & 0x3f) | 0x80
	return id, nil
}

// ParseSandboxSessionID accepts only a non-nil, lowercase, hyphenated UUID.
func ParseSandboxSessionID(value string) (SandboxSessionID, error) {
	var id SandboxSessionID
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return id, ErrInvalidSandboxSessionID
	}
	compact := make([]byte, 0, 32)
	for i := range value {
		if i != 8 && i != 13 && i != 18 && i != 23 {
			compact = append(compact, value[i])
		}
	}
	if _, err := hex.Decode(id[:], compact); err != nil || id == (SandboxSessionID{}) || id.String() != value {
		return SandboxSessionID{}, ErrInvalidSandboxSessionID
	}
	return id, nil
}

// String returns the canonical lowercase UUID representation.
func (id SandboxSessionID) String() string {
	var encoded [32]byte
	hex.Encode(encoded[:], id[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", encoded[0:8], encoded[8:12], encoded[12:16], encoded[16:20], encoded[20:32])
}

// MarshalText implements encoding.TextMarshaler.
func (id SandboxSessionID) MarshalText() ([]byte, error) {
	if id == (SandboxSessionID{}) {
		return nil, ErrInvalidSandboxSessionID
	}
	return []byte(id.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (id *SandboxSessionID) UnmarshalText(value []byte) error {
	if id == nil {
		return ErrInvalidSandboxSessionID
	}
	parsed, err := ParseSandboxSessionID(string(value))
	if err != nil {
		return err
	}
	*id = parsed
	return nil
}
