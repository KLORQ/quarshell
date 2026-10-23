// SPDX-FileCopyrightText: Copyright (c) 2025-2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package openshell

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
)

// EndpointStatusIdentity is the canonical, non-secret identity of one
// configured tool-server endpoint.
type EndpointStatusIdentity struct {
	ID    string
	Host  string
	Path  string
	Ports []uint32
}

// NewEndpointStatusIdentity derives the pinned OpenShell endpoint identity from
// the canonical host, effective sorted port set and path selector. Only these
// configured values contribute; request URLs, headers and credentials never do.
func NewEndpointStatusIdentity(host string, ports []uint32, port uint32, path string) EndpointStatusIdentity {
	canonicalHost := strings.ToLower(host)
	canonicalPath := path
	switch path {
	case "", "**", "/**":
		canonicalPath = "/**"
	}
	effective := append([]uint32(nil), ports...)
	if len(effective) == 0 && port != 0 {
		effective = []uint32{port}
	}
	sort.Slice(effective, func(i, j int) bool { return effective[i] < effective[j] })
	deduped := effective[:0]
	for _, item := range effective {
		if len(deduped) == 0 || deduped[len(deduped)-1] != item {
			deduped = append(deduped, item)
		}
	}
	hash := sha256.New()
	_, _ = hash.Write([]byte("openshell:endpoint:v1"))
	hashIdentityValue(hash, []byte(canonicalHost))
	hashIdentityValue(hash, []byte(canonicalPath))
	var count [8]byte
	binary.BigEndian.PutUint64(count[:], uint64(len(deduped)))
	hashIdentityValue(hash, count[:])
	for _, item := range deduped {
		var encoded [4]byte
		binary.BigEndian.PutUint32(encoded[:], item)
		hashIdentityValue(hash, encoded[:])
	}
	return EndpointStatusIdentity{
		ID:    "endpoint:v1:" + fmt.Sprintf("%x", hash.Sum(nil)),
		Host:  canonicalHost,
		Path:  canonicalPath,
		Ports: append([]uint32(nil), deduped...),
	}
}

func hashIdentityValue(hash interface{ Write([]byte) (int, error) }, value []byte) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = hash.Write(length[:])
	_, _ = hash.Write(value)
}
