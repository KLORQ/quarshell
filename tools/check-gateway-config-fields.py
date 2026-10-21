#!/usr/bin/env python3
"""Audit the gateway TOML schema against the pinned OpenShell structs.

Missing fields are errors: a valid upstream config would be rejected. Extra
fields are warnings by default because they are legacy debt that must be
removed before M8; --strict turns them into errors too.
"""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
UPSTREAM = ROOT / "tools/upstream/openshell-v0.1.4/crates"
SCHEMA = ROOT / "quarshell-gateway/internal/gatewayconfig/schema.go"

RUST_SOURCES = [
    UPSTREAM / "openshell-server/src/config_file.rs",
    UPSTREAM / "openshell-core/src/config.rs",
]

# Rust struct -> Go struct.
MAPPING = {
    "ConfigFile": "File",
    "OpenShellRoot": "Root",
    "GatewayFileSection": "Gateway",
    "GatewayTlsFileConfig": "TLS",
    "OcsfLogConfig": "OCSFLog",
    "OtlpConfig": "OTLP",
    "SupervisorFileSection": "Supervisor",
    "MiddlewareServiceFileConfig": "Middleware",
    "OidcConfig": "OIDC",
    "MtlsAuthConfig": "MTLSAuth",
    "GatewayAuthConfig": "Auth",
    "GatewayInterceptorConfig": "Interceptor",
    "GatewayInterceptorBindingOverride": "Binding",
    "GatewayJwtConfig": "JWT",
}

# Provider profile sources are an enum in Rust and a tagged struct in Go.
PROVIDER_SOURCE_FIELDS = {"type", "name"}


def rust_structs(paths: list[Path]) -> dict[str, set[str]]:
    structs: dict[str, set[str]] = {}
    for path in paths:
        if not path.exists():
            raise SystemExit(f"missing pinned source: {path}")
        text = path.read_text()
        for match in re.finditer(r"pub struct (\w+)\s*\{(.*?)\n\}", text, re.S):
            fields: set[str] = set()
            for line in match.group(2).splitlines():
                stripped = line.strip()
                if stripped.startswith("pub ") and ":" in stripped:
                    name = stripped[4:].split(":", 1)[0].strip()
                    if name:
                        fields.add(name)
            structs[match.group(1)] = fields
    return structs


def go_structs(path: Path) -> dict[str, set[str]]:
    text = path.read_text()
    structs: dict[str, set[str]] = {}
    for match in re.finditer(r"type (\w+) struct \{(.*?)\n\}", text, re.S):
        fields: set[str] = set()
        for line in match.group(2).splitlines():
            tag = re.search(r'toml:"([^"]+)"', line)
            if tag:
                fields.add(tag.group(1).split(",")[0])
        structs[match.group(1)] = fields
    return structs


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--strict", action="store_true", help="treat legacy extra fields as errors")
    args = parser.parse_args()

    rust = rust_structs(RUST_SOURCES)
    go = go_structs(SCHEMA)
    missing: list[str] = []
    extra: list[str] = []

    for rust_name, go_name in MAPPING.items():
        if rust_name not in rust:
            raise SystemExit(f"upstream struct not found: {rust_name}")
        if go_name not in go:
            missing.append(f"{rust_name}: Go struct {go_name} is missing")
            continue
        for field in sorted(rust[rust_name] - go[go_name]):
            missing.append(f"{rust_name}->{go_name}: missing field {field!r}")
        for field in sorted(go[go_name] - rust[rust_name]):
            extra.append(f"{rust_name}->{go_name}: extra legacy field {field!r}")

    provider = go.get("ProfileSource", set())
    if not PROVIDER_SOURCE_FIELDS.issubset(provider):
        missing.append("GatewayProviderProfileSourceConfig: ProfileSource lacks type/name")

    for problem in missing:
        print(f"error: {problem}", file=sys.stderr)
    for problem in extra:
        print(f"warning: {problem}", file=sys.stderr if args.strict else sys.stdout)

    fields = sum(len(values) for values in rust.values())
    print(f"gateway config schema: {len(MAPPING)} structs, {fields} pinned fields, {len(missing)} missing, {len(extra)} legacy")
    if missing or (args.strict and extra):
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
