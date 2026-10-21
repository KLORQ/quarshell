#!/usr/bin/env python3
"""Audit Docker/Podman gateway config fields against the pinned Rust structs."""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
UPSTREAM = ROOT / "tools/upstream/openshell-v0.1.4/crates"
MATRIX = ROOT / "quarshell-driver/internal/configcompat/testdata/openshell-v0.1.4-driver-config.json"
PIN_FILE = ROOT / "tools/upstream/UPSTREAM_SHA"
VERSION_FILE = ROOT / "tools/upstream/UPSTREAM_VERSION"

BACKENDS = {
    "docker": {
        "upstream_file": UPSTREAM / "openshell-driver-docker/src/lib.rs",
        "struct": "DockerComputeConfig",
        "parser_file": ROOT / "quarshell-driver/internal/docker/config.go",
        "test_refs": [
            "quarshell-driver/internal/docker/config_test.go",
            "quarshell-driver/internal/docker/sdk_contract_test.go",
        ],
    },
    "podman": {
        "upstream_file": UPSTREAM / "openshell-driver-podman/src/config.rs",
        "struct": "PodmanComputeConfig",
        "parser_file": ROOT / "quarshell-driver/podman/config.go",
        "test_refs": [
            "quarshell-driver/podman/podman_test.go",
            "quarshell-driver/podman/libpod_create_test.go",
        ],
    },
}

SECURITY_REJECTED = {"guest_tls_cert", "guest_tls_key"}
RUNTIME_IMAGE_GAPS = {
    "sandbox_runtime_image": "The pinned backend extracts a trusted sandbox binary from this image. The local runtime uses Gateway-provided helper binaries and has no equivalent image extraction/cache contract.",
    "supervisor_image": "The pinned backend runs a separately managed supervisor container from this image. The local backend currently starts its supervisor in the workload container, which changes its isolation and lifecycle boundary.",
    "supervisor_bin": "The pinned Docker setting supplies a host supervisor binary path. Local helper binaries are selected and staged by the Gateway, so accepting this path would have no effect.",
}

# This corpus separates parser inventory from effective behavior. Keep entries
# narrow: fields without source + executable evidence remain explicitly open.
VALUE_BEHAVIOR = {
    ("docker", "guest_tls_ca"): {
        "status": "partial",
        "omitted": "Both leave guest TLS disabled when the CA option is absent.",
        "explicit_empty": "Local rejects the empty path. Upstream stores Some(empty), enables TLS, then fails while loading the CA material.",
        "explicit_zero": "Wrong TOML type; rejected by both path/string decoders.",
        "runtime_effect": "A valid CA enables gateway TLS verification; local and upstream path/readability checks differ in timing.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-driver-docker/src/lib.rs", "quarshell-driver/internal/docker/config.go", "quarshell-driver/internal/docker/config_test.go"],
    },
    ("podman", "guest_tls_ca"): {
        "status": "partial",
        "omitted": "Both leave guest TLS disabled when the CA option is absent.",
        "explicit_empty": "Local rejects the empty path. Upstream stores Some(empty), enables TLS, then fails while loading the CA material.",
        "explicit_zero": "Wrong TOML type; rejected by both path/string decoders.",
        "runtime_effect": "A valid CA enables gateway TLS verification; local and upstream path/readability checks differ in timing.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-driver-podman/src/config.rs", "quarshell-driver/podman/config.go", "quarshell-driver/podman/podman_test.go"],
    },
    ("docker", "guest_tls_cert"): {
        "status": "partial",
        "omitted": "Both default to no client certificate.",
        "explicit_empty": "Rejected locally during decode; upstream deserializes Some(empty) and rejects it during driver startup.",
        "explicit_zero": "Wrong TOML type; rejected by both path/string decoders.",
        "runtime_effect": "Any configured client certificate is unsupported because sandbox identity uses bearer tokens; failure stage differs.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-driver-docker/src/lib.rs", "quarshell-driver/internal/docker/config.go", "quarshell-driver/internal/docker/config_test.go"],
    },
    ("docker", "guest_tls_key"): {
        "status": "partial",
        "omitted": "Both default to no client key.",
        "explicit_empty": "Rejected locally during decode; upstream deserializes Some(empty) and rejects it during driver startup.",
        "explicit_zero": "Wrong TOML type; rejected by both path/string decoders.",
        "runtime_effect": "Any configured client key is unsupported because sandbox identity uses bearer tokens; failure stage differs.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-driver-docker/src/lib.rs", "quarshell-driver/internal/docker/config.go", "quarshell-driver/internal/docker/config_test.go"],
    },
    ("podman", "guest_tls_cert"): {
        "status": "partial",
        "omitted": "Both default to no client certificate.",
        "explicit_empty": "Rejected locally during decode; upstream deserializes Some(empty) and rejects it during driver startup.",
        "explicit_zero": "Wrong TOML type; rejected by both path/string decoders.",
        "runtime_effect": "Any configured client certificate is unsupported because sandbox identity uses bearer tokens; failure stage differs.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-driver-podman/src/config.rs", "quarshell-driver/podman/config.go", "quarshell-driver/podman/podman_test.go"],
    },
    ("podman", "guest_tls_key"): {
        "status": "partial",
        "omitted": "Both default to no client key.",
        "explicit_empty": "Rejected locally during decode; upstream deserializes Some(empty) and rejects it during driver startup.",
        "explicit_zero": "Wrong TOML type; rejected by both path/string decoders.",
        "runtime_effect": "Any configured client key is unsupported because sandbox identity uses bearer tokens; failure stage differs.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-driver-podman/src/config.rs", "quarshell-driver/podman/config.go", "quarshell-driver/podman/podman_test.go"],
    },
    ("docker", "provider_spiffe_workload_api_socket"): {
        "status": "partial",
        "omitted": "Both omit the projected Workload API socket.",
        "explicit_empty": "Rejected locally and by upstream path validation.",
        "explicit_zero": "Wrong TOML type; rejected by both path/string decoders.",
        "runtime_effect": "A valid host Unix socket is projected read-only to the supervisor; host socket availability and permission behavior still require target-host acceptance.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-core/src/driver_utils.rs", "quarshell-driver/internal/docker/config.go", "quarshell-driver/internal/docker/sidecar.go"],
    },
    ("podman", "provider_spiffe_workload_api_socket"): {
        "status": "partial",
        "omitted": "Both omit the projected Workload API socket.",
        "explicit_empty": "Rejected locally and by upstream path validation.",
        "explicit_zero": "Wrong TOML type; rejected by both path/string decoders.",
        "runtime_effect": "A valid host Unix socket is projected read-only to the supervisor; host socket availability and permission behavior still require target-host acceptance.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-core/src/driver_utils.rs", "quarshell-driver/podman/config.go", "quarshell-driver/podman/podman.go"],
    },
    ("docker", "app_armor_profile"): {
        "status": "partial",
        "omitted": "Both omit an override and preserve the container runtime default.",
        "explicit_empty": "Rejected by both enum parsers.",
        "explicit_zero": "Wrong TOML type; rejected by both enum/string decoders.",
        "runtime_effect": "Supported profiles are host-checked before mutations and translated to Engine security options; host-specific AppArmor availability still requires acceptance.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-driver-docker/src/lib.rs", "quarshell-driver/internal/docker/app_armor_test.go", "quarshell-driver/internal/docker/config_test.go"],
    },
    ("podman", "app_armor_profile"): {
        "status": "partial",
        "omitted": "Both omit an override and preserve the container runtime default.",
        "explicit_empty": "Rejected by both enum parsers.",
        "explicit_zero": "Wrong TOML type; rejected by both enum/string decoders.",
        "runtime_effect": "Supported profiles are host-checked before mutations and translated to the native Libpod create request; host-specific AppArmor availability still requires acceptance.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-driver-podman/src/config.rs", "quarshell-driver/podman/app_armor_test.go", "quarshell-driver/podman/libpod_create_test.go"],
    },
    ("docker", "sandbox_runtime_image"): {
        "status": "partial",
        "omitted": "Upstream leaves the optional image unset; local helper binaries are selected and staged by the Gateway.",
        "explicit_empty": "Upstream accepts an empty optional string but local rejects the field as unsupported.",
        "explicit_zero": "Wrong TOML type; rejected by the upstream string option and local unknown-field gate.",
        "runtime_effect": "The local runtime does not extract or cache the binary from an OCI image, so explicit configuration fails closed.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-driver-docker/src/lib.rs", "quarshell-driver/internal/docker/config.go"],
    },
    ("docker", "supervisor_bin"): {
        "status": "partial",
        "omitted": "Upstream leaves the optional host binary path unset; local helper binaries are selected and staged by the Gateway.",
        "explicit_empty": "Upstream accepts an empty optional path but local rejects the field as unsupported.",
        "explicit_zero": "Wrong TOML type; rejected by the upstream path option and local unknown-field gate.",
        "runtime_effect": "The local Gateway owns binary selection and staging, so explicit host paths fail closed.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-driver-docker/src/lib.rs", "quarshell-driver/internal/docker/config.go"],
    },
    ("docker", "supervisor_image"): {
        "status": "partial",
        "omitted": "Upstream leaves the optional supervisor image unset; local helper binaries are selected and staged by the Gateway.",
        "explicit_empty": "Upstream accepts an empty optional string but local rejects the field as unsupported.",
        "explicit_zero": "Wrong TOML type; rejected by the upstream string option and local unknown-field gate.",
        "runtime_effect": "The local runtime does not create a separately managed supervisor container from this image.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-driver-docker/src/lib.rs", "quarshell-driver/internal/docker/config.go"],
    },
    ("podman", "sandbox_runtime_image"): {
        "status": "partial",
        "omitted": "Upstream selects an environment or compiled default OCI image; local helper binaries are selected and staged by the Gateway.",
        "explicit_empty": "Upstream accepts an empty optional string but local rejects the field as unsupported.",
        "explicit_zero": "Wrong TOML type; rejected by the upstream string option and local unknown-field gate.",
        "runtime_effect": "The local runtime does not extract or cache the binary from an OCI image, so explicit configuration fails closed.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-driver-podman/src/config.rs", "quarshell-driver/podman/config.go"],
    },
    ("podman", "supervisor_image"): {
        "status": "partial",
        "omitted": "Upstream selects an environment or compiled default OCI image; local helper binaries are selected and staged by the Gateway.",
        "explicit_empty": "Upstream accepts an empty optional string but local rejects the field as unsupported.",
        "explicit_zero": "Wrong TOML type; rejected by the upstream string option and local unknown-field gate.",
        "runtime_effect": "The local runtime does not create a separately managed supervisor container from this image.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-driver-podman/src/config.rs", "quarshell-driver/podman/config.go"],
    },
    ("docker", "allow_driver_config"): {
        "status": "verified",
        "omitted": "Upstream and local defaults disable caller-supplied driver_config.",
        "explicit_empty": "Wrong TOML type; rejected by both typed decoders and the local boolean decoder.",
        "explicit_zero": "Wrong TOML type; the option is boolean. false is accepted and preserves the disabled default.",
        "runtime_effect": "true is effective only with the operator-owned resource admission policy; false is rejected before Engine mutation.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-driver-docker/src/lib.rs", "quarshell-driver/internal/docker/config_test.go", "quarshell-driver/internal/docker/resource_admission_test.go"],
    },
    ("podman", "allow_driver_config"): {
        "status": "verified",
        "omitted": "Upstream and local defaults disable caller-supplied driver_config.",
        "explicit_empty": "Wrong TOML type; rejected by both typed decoders and the local boolean decoder.",
        "explicit_zero": "Wrong TOML type; the option is boolean. false is accepted and preserves the disabled default.",
        "runtime_effect": "true is effective only with the operator-owned resource admission policy; false is rejected before Engine mutation.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-driver-podman/src/config.rs", "quarshell-driver/podman/podman_test.go", "quarshell-driver/internal/docker/resource_admission_test.go"],
    },
    ("docker", "resource_admission"): {
        "status": "verified",
        "omitted": "Both implementations enable admission with the pinned shared-approval and workspace labels.",
        "explicit_empty": "An empty table selects the same defaults; an empty required_labels table is rejected while enabled.",
        "explicit_zero": "not-applicable",
        "runtime_effect": "The policy is validated before caller mounts; named volumes require matching approval/workspace labels and identity is checked again on restart.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-core/src/resource_admission.rs", "quarshell-core/resourceadmission/admission_test.go", "quarshell-driver/internal/docker/resource_admission_test.go"],
    },
    ("podman", "resource_admission"): {
        "status": "verified",
        "omitted": "Both implementations enable admission with the pinned shared-approval and workspace labels.",
        "explicit_empty": "An empty table selects the same defaults; an empty required_labels table is rejected while enabled.",
        "explicit_zero": "not-applicable",
        "runtime_effect": "The policy is validated before caller mounts; named volumes require matching approval/workspace labels and identity is checked again on restart.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-core/src/resource_admission.rs", "quarshell-core/resourceadmission/admission_test.go", "quarshell-driver/internal/docker/resource_admission_test.go"],
    },
    ("docker", "socket_path"): {
        "status": "partial",
        "omitted": "Upstream probes the configured Docker environment and falls back to /var/run/docker.sock; the local Engine adapter uses Docker client environment settings.",
        "explicit_empty": "Rejected locally. Upstream retains an explicitly empty PathBuf and fails when creating its Docker client.",
        "explicit_zero": "not-applicable",
        "runtime_effect": "Both select a Docker API endpoint. A local mutual-TLS Engine test confirms DOCKER_HOST, DOCKER_CERT_PATH, DOCKER_TLS_VERIFY, server validation, and client-certificate authentication; socket discovery and a real external daemon remain target-host acceptance.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-driver-docker/src/lib.rs", "quarshell-driver/internal/docker/register.go", "quarshell-driver/internal/docker/config_test.go", "quarshell-driver/internal/docker/tls_engine_test.go"],
    },
    ("podman", "socket_path"): {
        "status": "partial",
        "omitted": "Both discover a responsive Podman socket; local discovery also honors Docker-compatible environment variables and Quarshell/Cautem candidate paths.",
        "explicit_empty": "Rejected locally. Upstream retains an explicitly empty PathBuf and attempts to use it instead of discovery.",
        "explicit_zero": "not-applicable",
        "runtime_effect": "Both connect to the selected Podman API socket; discovery order differs and needs host acceptance.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-driver-podman/src/driver.rs", "quarshell-driver/podman/podman.go", "quarshell-driver/podman/podman_test.go"],
    },
    ("docker", "sandbox_label"): {
        "status": "partial",
        "omitted": "Both use the value default.",
        "explicit_empty": "Local rejects whitespace-only labels; upstream stores the empty string as a Docker label value.",
        "explicit_zero": "Wrong TOML type; rejected by both decoders.",
        "runtime_effect": "The value scopes sandbox labels; empty-value behavior intentionally fails earlier locally and should remain documented.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-driver-docker/src/lib.rs", "quarshell-driver/internal/docker/config.go", "quarshell-driver/internal/docker/config_test.go"],
    },
    ("docker", "grpc_endpoint"): {
        "status": "partial",
        "omitted": "Both default to an empty string. The local Gateway rewrites a configured grpc_endpoint; no implicit endpoint is added when the key is absent.",
        "explicit_empty": "Accepted and has the same value as omission in the local config; upstream also stores an empty String.",
        "explicit_zero": "Wrong TOML type; rejected by both decoders.",
        "runtime_effect": "Endpoint discovery and TLS choice are supplied by the local Gateway/runtime descriptor and are not yet cross-checked against the upstream Docker host path.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-driver-docker/src/lib.rs", "quarshell-driver/internal/docker/config.go", "quarshell-driver/internal/docker/sdk_contract_test.go"],
    },
    ("podman", "grpc_endpoint"): {
        "status": "partial",
        "omitted": "Both default to an empty string; upstream falls back through gateway_port and platform host discovery. The local Gateway rewrites a configured grpc_endpoint but does not add a missing key.",
        "explicit_empty": "Accepted and has the same value as omission in the local config; upstream also stores an empty String.",
        "explicit_zero": "Wrong TOML type; rejected by both decoders.",
        "runtime_effect": "Linux and Podman Machine endpoint selection are not yet cross-checked against Gateway runtime injection on each target host.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-driver-podman/src/config.rs", "quarshell-driver/podman/config.go", "quarshell-driver/podman/podman_test.go"],
    },
    ("podman", "gateway_port"): {
        "status": "partial",
        "omitted": "Upstream defaults to 17670; the local decoder leaves zero and no implicit gateway_port injection is present in registry construction.",
        "explicit_empty": "Wrong TOML type; rejected by both integer decoders.",
        "explicit_zero": "Upstream accepts zero as u16; local accepts zero and treats it as no override.",
        "runtime_effect": "The field controls Podman Machine/host endpoint fallback; direct standalone default parity differs and remains open.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-driver-podman/src/config.rs", "quarshell-driver/podman/config.go", "quarshell-driver/podman/podman.go"],
    },
    ("podman", "network_name"): {
        "status": "partial",
        "omitted": "Upstream defaults to openshell; local Engine-backed resource naming uses its Cautem network prefix when this value is empty.",
        "explicit_empty": "Accepted locally and selects the local fallback; upstream stores an empty String and passes it into network creation.",
        "explicit_zero": "Wrong TOML type; rejected by both string decoders.",
        "runtime_effect": "Explicit non-empty values select the network prefix; default resource naming differs and remains a compatibility decision.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-driver-podman/src/config.rs", "quarshell-driver/podman/config.go", "quarshell-driver/internal/docker/create.go"],
    },
    ("podman", "host_gateway_ip"): {
        "status": "partial",
        "omitted": "Upstream selects a Podman Machine host IP on macOS and localhost on Linux; local empty value delegates to Docker-compatible host-gateway aliases.",
        "explicit_empty": "Accepted locally and treated as omission; upstream also stores an empty String and resolves its platform default.",
        "explicit_zero": "Wrong TOML type; rejected by both decoders.",
        "runtime_effect": "The resolved address affects sandbox-to-host proxy routing; macOS and Linux host lanes remain required.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-driver-podman/src/config.rs", "quarshell-driver/podman/config.go", "quarshell-driver/internal/docker/runtime_config.go"],
    },
    ("docker", "enable_bind_mounts"): {
        "status": "partial",
        "omitted": "Both default to false.",
        "explicit_empty": "Wrong TOML type; rejected by both boolean decoders.",
        "explicit_zero": "Wrong TOML type; false is accepted and keeps bind mounts disabled.",
        "runtime_effect": "true enables caller bind-mount parsing after operator admission; target-host source pinning and race acceptance remain open.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-driver-docker/src/lib.rs", "quarshell-driver/internal/docker/config_test.go", "quarshell-driver/internal/docker/driver_config_test.go"],
    },
    ("podman", "enable_bind_mounts"): {
        "status": "partial",
        "omitted": "Both default to false.",
        "explicit_empty": "Wrong TOML type; rejected by both boolean decoders.",
        "explicit_zero": "Wrong TOML type; false is accepted and keeps bind mounts disabled.",
        "runtime_effect": "true enables caller bind-mount parsing after operator admission; target-host source pinning and race acceptance remain open.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-driver-podman/src/config.rs", "quarshell-driver/podman/podman_test.go", "quarshell-driver/internal/docker/driver_config_test.go"],
    },
    ("docker", "default_image"): {
        "status": "partial",
        "omitted": "Pinned Rust default is nvcr.io/nvidia/base/ubuntu:24.04; local ConfigFromMap leaves the value empty and the driver chooses a Cautem image.",
        "explicit_empty": "Rejected locally to prevent empty input from silently selecting the product image; upstream accepts the String shape and fails later when creating with the empty image.",
        "explicit_zero": "not-applicable",
        "runtime_effect": "The default image differs by design; an explicit non-empty image is passed to the workload image selection path.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-driver-docker/src/lib.rs", "quarshell-driver/internal/docker/sdk_contract_test.go"],
    },
    ("podman", "default_image"): {
        "status": "partial",
        "omitted": "Pinned Rust default is nvcr.io/nvidia/base/ubuntu:24.04; local ConfigFromMap leaves the value empty and the driver chooses a Cautem image.",
        "explicit_empty": "Rejected locally to prevent empty input from silently selecting the product image; upstream accepts the String shape and fails later when creating with the empty image.",
        "explicit_zero": "not-applicable",
        "runtime_effect": "The default image differs by design; an explicit non-empty image is passed to the workload image selection path.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-driver-podman/src/config.rs", "quarshell-driver/internal/docker/sdk_contract_test.go"],
    },
    ("docker", "sandbox_pids_limit"): {
        "status": "partial",
        "omitted": "Upstream and local runtime default to 2048. Local CAUTEM_SANDBOX_PIDS_LIMIT can override that default, including opting out with zero.",
        "explicit_empty": "Wrong TOML type; rejected by the upstream typed decoder and local integer decoder.",
        "explicit_zero": "Rejected by both typed decoders because the field is NonZeroI64.",
        "runtime_effect": "Positive values set the Engine PIDs limit. The local environment override means omission is not invariant across deployments.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-core/src/config.rs", "quarshell-driver/internal/docker/resources_test.go", "quarshell-driver/internal/docker/config_test.go"],
    },
    ("podman", "sandbox_pids_limit"): {
        "status": "partial",
        "omitted": "Upstream and local runtime default to 2048. Local CAUTEM_SANDBOX_PIDS_LIMIT can override that default, including opting out with zero.",
        "explicit_empty": "Wrong TOML type; rejected by the upstream typed decoder and local integer decoder.",
        "explicit_zero": "Rejected by both typed decoders because the field is NonZeroI64.",
        "runtime_effect": "Positive values set the native Libpod PIDs limit. The local environment override means omission is not invariant across deployments.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-core/src/config.rs", "quarshell-driver/internal/docker/resources_test.go", "quarshell-driver/podman/podman_test.go"],
    },
    ("docker", "image_pull_policy"): {
        "status": "partial",
        "omitted": "Upstream defaults to if_not_present. Local empty policy follows the if-not-present path for ordinary images, with a product-image pull environment override.",
        "explicit_empty": "Rejected locally; upstream enum deserialization rejects an empty variant.",
        "explicit_zero": "not-applicable",
        "runtime_effect": "Canonical policies are translated to image presence/pull behavior. Local aliases extend the upstream vocabulary; newer is rejected for Docker.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-core/src/config.rs", "quarshell-driver/internal/docker/config_test.go", "quarshell-driver/internal/docker/images.go"],
    },
    ("podman", "image_pull_policy"): {
        "status": "partial",
        "omitted": "Upstream defaults to if_not_present. Local empty policy follows the if-not-present path for ordinary images, with a product-image pull environment override.",
        "explicit_empty": "Rejected locally; upstream enum deserialization rejects an empty variant.",
        "explicit_zero": "not-applicable",
        "runtime_effect": "Canonical policies are translated to Libpod values. Local aliases extend the upstream vocabulary.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-core/src/config.rs", "quarshell-driver/podman/podman_test.go", "quarshell-driver/podman/libpod_create.go"],
    },
    ("podman", "stop_timeout_secs"): {
        "status": "verified",
        "omitted": "Both configs default to 45 seconds.",
        "explicit_empty": "Wrong TOML type; rejected by both typed decoders and the local integer decoder.",
        "explicit_zero": "Accepted and passed through as zero; the local configured bit prevents zero from being replaced by the 10-second Docker adapter fallback.",
        "runtime_effect": "Native Podman receives the configured stop timeout in seconds.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-driver-podman/src/config.rs", "quarshell-driver/podman/podman_test.go", "quarshell-driver/internal/docker/runtime_config_test.go"],
    },
    ("podman", "health_check_interval_secs"): {
        "status": "partial",
        "omitted": "Upstream leaves the supervisor health check disabled while retaining a 10-second workload readiness check. Local combined-container mode applies a 10-second readiness check.",
        "explicit_empty": "Wrong TOML type; rejected by both typed decoders and the local integer decoder.",
        "explicit_zero": "Rejected by both typed decoders because the field is NonZeroU64.",
        "runtime_effect": "Configured intervals update the local container health check; upstream configures a separate supervisor health check. The lifecycle boundary differs.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-driver-podman/src/config.rs", "tools/upstream/openshell-v0.1.4/crates/openshell-driver-podman/src/container.rs", "quarshell-driver/podman/podman_test.go", "quarshell-driver/internal/docker/sdk_contract_test.go"],
    },
    ("podman", "userns"): {
        "status": "partial",
        "omitted": "Both configs leave the engine user namespace at its default.",
        "explicit_empty": "Rejected during userns canonicalization by both implementations.",
        "explicit_zero": "not-applicable",
        "runtime_effect": "Supported modes and ID mappings are applied; local compatibility aliases and engine-version/host support still need the full host matrix.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-driver-podman/src/config.rs", "quarshell-driver/podman/podman_test.go", "quarshell-driver/podman/libpod_create_test.go"],
    },
}

for backend in BACKENDS:
    if backend == "docker":
        upstream_driver = "tools/upstream/openshell-v0.1.4/crates/openshell-driver-docker/src/lib.rs"
        local_parser = "quarshell-driver/internal/docker/config.go"
        local_tests = ["quarshell-driver/internal/docker/config_test.go", "quarshell-driver/internal/docker/sdk_contract_test.go"]
    else:
        upstream_driver = "tools/upstream/openshell-v0.1.4/crates/openshell-driver-podman/src/config.rs"
        local_parser = "quarshell-driver/podman/config.go"
        local_tests = ["quarshell-driver/podman/podman_test.go", "quarshell-driver/podman/libpod_create_test.go"]

    VALUE_BEHAVIOR[(backend, "ssh_socket_path")] = {
        "status": "partial",
        "omitted": "Upstream defaults to /run/openshell/ssh.sock; local runtime defaults to its Cautem guest SSH socket path.",
        "explicit_empty": "Local rejects an empty path instead of silently selecting its product default; upstream accepts the empty String shape and the resulting guest SSH path is unusable.",
        "explicit_zero": "Wrong TOML type; rejected by both string decoders.",
        "runtime_effect": "The guest SSH socket path is shared with the relay and health/readiness checks; the path differs while the relay behavior is covered locally.",
        "evidence_refs": [upstream_driver, "tools/upstream/openshell-v0.1.4/crates/openshell-core/src/container_paths.rs", local_parser, "quarshell-driver/internal/docker/runtime_config.go", *local_tests],
    }

    for field in ("https_proxy", "no_proxy", "proxy_auth_file", "proxy_auth_allow_insecure", "proxy_connect_by_hostname", "proxy_ca_bundle"):
        if field in ("https_proxy", "no_proxy"):
            empty = "Explicit empty strings are rejected locally; upstream deserializes the Option and rejects the empty value during shared proxy validation."
            effect = "The proxy URL or bypass list is validated and passed only to the supervisor; no proxy credential is exposed in sandbox environment values."
        elif field in ("proxy_auth_file", "proxy_ca_bundle"):
            empty = "Explicit empty paths/strings are rejected locally; upstream accepts the option shape and rejects an empty or unusable path during validation."
            effect = "The path is validated and mounted for the supervisor; proxy credentials use a secret mount and CA material extends trust."
        elif field == "proxy_auth_allow_insecure":
            empty = "Wrong TOML type. Explicit false is retained as presence and rejected without proxy_auth_file, matching upstream Option<bool> validation."
            effect = "true acknowledges cleartext Basic auth only when a proxy auth file is configured; false has no effect with a secure proxy and auth file."
        else:
            empty = "Wrong TOML type. Explicit false is retained as presence and rejected without https_proxy, matching upstream Option<bool> validation."
            effect = "The boolean selects hostname CONNECT behavior only when a corporate proxy is configured; host/network acceptance remains required."
        VALUE_BEHAVIOR[(backend, field)] = {
            "status": "partial",
            "omitted": "Both omit the optional setting; false/absent proxy options do not alter the runtime.",
            "explicit_empty": empty,
            "explicit_zero": "Wrong TOML type for this string/path/boolean field; rejected by both decoders.",
            "runtime_effect": effect + " Cross-backend egress behavior still needs the remaining target-host lanes.",
            "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-core/src/config.rs", upstream_driver, local_parser, *local_tests],
        }

for field in ("uidmap", "gidmap"):
    VALUE_BEHAVIOR[("podman", field)] = {
        "status": "partial",
        "omitted": "Both default to an empty mapping list and apply no explicit UID/GID ranges.",
        "explicit_empty": "An explicit empty array is accepted by both decoders and applies no mappings.",
        "explicit_zero": "A numeric zero is not a mapping string and is rejected; each mapping string also requires size greater than zero.",
        "runtime_effect": "Non-empty ranges require userns=private, are parsed as uint32 container:host:size tuples and reach native Libpod IDMappings; rootless/rootful host acceptance remains open.",
        "evidence_refs": ["tools/upstream/openshell-v0.1.4/crates/openshell-driver-podman/src/config.rs", "tools/upstream/openshell-v0.1.4/crates/openshell-driver-podman/src/container.rs", "quarshell-driver/podman/config.go", "quarshell-driver/podman/podman_test.go", "quarshell-driver/podman/libpod_create_test.go"],
    }


def pin_value(path: Path) -> str:
    return path.read_text(encoding="utf-8").strip()


def parse_struct(text: str, name: str) -> list[tuple[str, str, str]]:
    start = text.index(f"pub struct {name} {{")
    end = text.index("\n}", start)
    body = text[start:end]
    fields: list[tuple[str, str, str]] = []
    attributes: list[str] = []
    for line in body.splitlines()[1:]:
        stripped = line.strip()
        if stripped.startswith("#["):
            attributes.append(stripped)
            continue
        match = re.match(r"pub\s+([a-z_][a-z0-9_]*)\s*:\s*(.+),$", stripped)
        if match:
            fields.append((match.group(1), match.group(2).strip(), " ".join(attributes)))
        attributes = []
    return fields


def upstream_fields(backend: str) -> list[tuple[str, str]]:
    spec = BACKENDS[backend]
    text = spec["upstream_file"].read_text(encoding="utf-8")
    fields = parse_struct(text, spec["struct"])
    result: list[tuple[str, str]] = []
    for field, kind, attributes in fields:
        if "serde(skip)" in attributes:
            continue
        if field == "upstream_proxy":
            core = (UPSTREAM / "openshell-core/src/config.rs").read_text(encoding="utf-8")
            result.extend((name, value_type) for name, value_type, _ in parse_struct(core, "UpstreamProxyConfig"))
        else:
            result.append((field, kind))
    return result


def parser_cases(path: Path) -> dict[str, str]:
    source = path.read_text(encoding="utf-8")
    start = source.index("func ConfigFromMap(")
    end = source.index("\nfunc ", start + 1)
    body = source[start:end]
    cases: dict[str, str] = {}
    lines = body.splitlines()
    for index, line in enumerate(lines):
        match = re.search(r'case\s+(.+):$', line.strip())
        if not match:
            continue
        names = re.findall(r'"([a-z_][a-z0-9_]*)"', match.group(1))
        end = index + 1
        while end < len(lines) and not re.match(r"\s*(?:case\s+|default\s*:)", lines[end]):
            end += 1
        block = "\n".join(lines[index + 1 : end])
        target = re.search(r"cfg\.([A-Z][A-Za-z0-9_]*)\s*(?:,\s*err\s*=|=)", block)
        if target:
            binding = target.group(1)
        elif "socket_path" in names and path == BACKENDS["docker"]["parser_file"]:
            binding = "daemon endpoint returned by ConfigFromMap"
        else:
            binding = "validated by field-specific decoder"
        for name in names:
            cases[name] = binding
    return cases


def row(backend: str, field: str, kind: str, cases: dict[str, str]) -> dict[str, object]:
    if field in SECURITY_REJECTED:
        decoder = "decodes-then-rejects"
        binding = None
        status = "intentional-rejection"
        behavior = "The pinned v0.1.4 config accepts the optional field syntactically but rejects it during driver startup because sandbox identity uses bearer tokens."
    elif field in cases:
        decoder = "accepted-and-validated"
        binding = cases[field]
        status = "local-path-present"
        behavior = f"ConfigFromMap maps this field into {binding}; local runtime effects and exact upstream defaults are tracked separately from this parser inventory."
    else:
        decoder = "unknown-field-rejected"
        binding = None
        status = "unsupported"
        behavior = RUNTIME_IMAGE_GAPS.get(
            field,
            "The local Engine API backend rejects this field before backend construction; no runtime effect is claimed.",
        )

    value_behavior = VALUE_BEHAVIOR.get(
        (backend, field),
        {
            "status": "open",
            "omitted": "not-audited",
            "explicit_empty": "not-audited",
            "explicit_zero": "not-audited",
            "runtime_effect": "not-audited",
            "evidence_refs": [],
        },
    )
    test_refs = list(BACKENDS[backend]["test_refs"])
    if backend == "docker" and field == "socket_path":
        test_refs.append("quarshell-driver/internal/docker/tls_engine_test.go")
    return {
        "backend": backend,
        "field": field,
        "upstream_type": kind,
        "local_decoder": decoder,
        "local_binding": binding,
        "runtime_or_parity_note": behavior,
        "value_behavior": value_behavior,
        "null_value_corpus": "TOML has no null token; Go decoder rejects a present nil value. The backend config tests enumerate supported and unsupported null cases.",
        "status": status,
        "test_refs": test_refs,
    }


def expected_matrix() -> dict[str, object]:
    return {
        "schema_version": 2,
        "upstream_version": pin_value(VERSION_FILE),
        "upstream_commit": pin_value(PIN_FILE),
        "scope": "Every externally deserializable field in pinned DockerComputeConfig and PodmanComputeConfig, including flattened proxy fields. This inventory is not a claim of full config behavior parity.",
        "fields": [
            row(backend, field, kind, parser_cases(BACKENDS[backend]["parser_file"]))
            for backend in BACKENDS
            for field, kind in upstream_fields(backend)
        ],
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--write", action="store_true", help="regenerate the field inventory from the pinned Rust structs")
    args = parser.parse_args()
    expected = expected_matrix()
    if args.write:
        MATRIX.parent.mkdir(parents=True, exist_ok=True)
        MATRIX.write_text(json.dumps(expected, indent=2) + "\n", encoding="utf-8")
        print(f"driver config matrix: wrote {len(expected['fields'])} fields to {MATRIX.relative_to(ROOT)}")
        return 0

    try:
        actual = json.loads(MATRIX.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as error:
        print(f"driver config matrix: cannot read {MATRIX}: {error}", file=sys.stderr)
        return 1
    if actual != expected:
        expected_keys = {(item["backend"], item["field"]) for item in expected["fields"]}
        actual_keys = {(item.get("backend"), item.get("field")) for item in actual.get("fields", [])}
        missing = sorted(expected_keys - actual_keys)
        extra = sorted(actual_keys - expected_keys)
        print(f"driver config matrix: stale or incorrect; missing={missing}, extra={extra}", file=sys.stderr)
        print("Regenerate only after reviewing the pinned source diff: python3 tools/check-driver-config-matrix.py --write", file=sys.stderr)
        return 1
    for item in actual["fields"]:
        for path in item["test_refs"]:
            if not (ROOT / path).is_file():
                print(f"driver config matrix: missing test reference {path}", file=sys.stderr)
                return 1
    behavior_counts: dict[str, int] = {}
    for item in actual["fields"]:
        behavior = item.get("value_behavior", {})
        required_behavior = {"status", "omitted", "explicit_empty", "explicit_zero", "runtime_effect", "evidence_refs"}
        if not required_behavior.issubset(behavior):
            print(f"driver config matrix: incomplete value behavior for {item['backend']}.{item['field']}", file=sys.stderr)
            return 1
        behavior_status = behavior["status"]
        if behavior_status not in {"open", "partial", "verified"}:
            print(f"driver config matrix: invalid value behavior status {behavior_status!r}", file=sys.stderr)
            return 1
        if behavior_status != "open" and not behavior["evidence_refs"]:
            print(f"driver config matrix: missing value behavior evidence for {item['backend']}.{item['field']}", file=sys.stderr)
            return 1
        behavior_counts[behavior_status] = behavior_counts.get(behavior_status, 0) + 1
        for path in behavior["evidence_refs"]:
            if not (ROOT / path).is_file():
                print(f"driver config matrix: missing value-behavior evidence {path}", file=sys.stderr)
                return 1
    summary = ", ".join(f"{count} {status}" for status, count in sorted(behavior_counts.items()))
    print(f"driver config matrix: PASS ({len(actual['fields'])} pinned backend fields; value behavior: {summary})")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
