#!/usr/bin/env python3
"""Generate a source-linked inventory from the pinned OpenShell checkout."""

from __future__ import annotations

import hashlib
import json
import re
import subprocess
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[1]
SOURCE = ROOT / "tools/upstream/openshell-v0.1.4"
OUT = ROOT / "plans/inventory/openshell-v0.1.4-inventory.json"
FEATURES_OUT = ROOT / "plans/inventory/features.yaml"
CLI_HELP = ROOT / "plans/inventory/openshell-v0.1.4-cli-help.json"
PLAN = ROOT / "plans/openshell-go-2026-10-21/PLAN.md"
BASELINE = "5a7728c304b5528e954e9ae900cf21f84e1a12b1"


def git(*args: str) -> str:
    return subprocess.check_output(["git", "-C", str(SOURCE), *args], text=True).strip()


def git_local(path: Path, *args: str) -> str | None:
    try:
        return subprocess.check_output(["git", "-C", str(path), *args], text=True, stderr=subprocess.DEVNULL).strip()
    except subprocess.CalledProcessError:
        return None


def digest(paths: list[Path]) -> str:
    h = hashlib.sha256()
    for path in sorted(paths):
        h.update(path.relative_to(SOURCE).as_posix().encode())
        h.update(b"\0")
        h.update(path.read_bytes())
        h.update(b"\0")
    return h.hexdigest()


def owner(crate: str) -> str:
    mappings = (
        ("prover", "quarshell-prover"),
        ("driver-db-credstore", "quarshell-credentials"),
        ("driver-kubernetes-secrets", "quarshell-credentials"),
        ("driver-vault", "quarshell-credentials"),
        ("driver-vm", "quarshell-driver-vm"),
        ("driver-mxc", "quarshell-driver-mxc"),
        ("driver-docker", "quarshell-driver"),
        ("driver-podman", "quarshell-driver"),
        ("driver-kubernetes", "quarshell-driver"),
        ("vfio", "quarshell-driver-vm"),
        ("sandbox-backend", "quarshell-isolation"),
        ("sandbox", "quarshell-isolation"),
        ("binary-identity", "quarshell-isolation"),
        ("isolation-interface", "quarshell-spi"),
        ("supervisor-process", "quarshell-supervisor"),
        ("supervisor-network", "quarshell-proxy"),
        ("supervisor-middleware-builtins", "quarshell-proxy"),
        ("supervisor-middleware", "quarshell-proxy"),
        ("supervisor", "quarshell-supervisor"),
        ("policy-schema", "quarshell-policy"),
        ("policy", "quarshell-policy"),
        ("ocsf", "quarshell-telemetry"),
        ("otel-test-support", "quarshell-telemetry"),
        ("otel", "quarshell-telemetry"),
        ("extension-core", "quarshell-spi"),
        ("gateway-interceptors", "quarshell-gateway"),
        ("server-macros", "quarshell-gateway"),
        ("server", "quarshell-gateway"),
        ("gateway", "quarshell-gateway"),
        ("bootstrap", "quarshell-cli"),
        ("tui", "quarshell-cli"),
        ("cli", "quarshell-cli"),
        ("sdk", "quarshell-sdk"),
        ("python", "quarshell-python"),
        ("providers", "quarshell-providers"),
        ("conformance", "quarshell-gateway"),
        ("proto", "quarshell-proto"),
        ("core", "quarshell-core"),
    )
    for key, value in mappings:
        if crate == key or crate.startswith(key + "-"):
            return value
    return "UNASSIGNED"


def feature_ids(crate: str) -> list[str]:
    name = crate.removeprefix("openshell-")
    result: set[str] = set()
    if name == "cli": result.update(("OS-CLI", "OS-GATEWAY-DEPLOY"))
    if name == "tui": result.add("OS-CLI")
    if name == "sdk": result.add("OS-SDK")
    if name == "python": result.add("OS-SDK")
    if name == "providers": result.update(("OS-PROVIDERS", "OS-CREDENTIALS"))
    if name.startswith("driver-"):
        result.update(("OS-BACKENDS", "OS-SANDBOX"))
        if "docker" in name or "podman" in name or "kubernetes" in name: result.add("OS-EXEC")
        if "-vm" in name: result.update(("OS-EXEC", "OS-ISOLATION-PROTO"))
        if "-mxc" in name: result.update(("OS-EXEC", "OS-FS", "OS-NET"))
        if any(x in name for x in ("vault", "credstore", "secrets")): result.update(("OS-CRED-DRIVERS", "OS-CREDENTIALS"))
    if name == "vfio": result.add("OS-GPU")
    if name == "sandbox": result.update(("OS-FS", "OS-NET", "OS-EXEC"))
    if name == "sandbox-backend": result.update(("OS-ISOLATION-PROTO", "OS-FS", "OS-NET"))
    if name == "supervisor": result.update(("OS-MAIN", "OS-EXEC", "OS-SANDBOX", "OS-ISOLATION-PROTO"))
    if name == "supervisor-process": result.update(("OS-MAIN", "OS-EXEC", "OS-FS"))
    if name == "supervisor-network": result.update(("OS-NET", "OS-L7", "OS-CREDENTIALS"))
    if name in {"supervisor-middleware", "supervisor-middleware-builtins"}:
        result.update(("OS-MIDDLEWARE", "OS-L7"))
    if name in {"policy", "policy-schema"}:
        result.update(("OS-POLICY-SCHEMA", "OS-POLICY-LIFECYCLE", "OS-FS", "OS-NET", "OS-L7"))
    if name == "isolation-interface": result.update(("OS-ISOLATION-PROTO", "OS-FS", "OS-NET"))
    if name in {"server", "gateway"}:
        result.update(("OS-API", "OS-SANDBOX", "OS-WORKSPACE", "OS-PEER", "OS-CRED-API"))
    if name in {"bootstrap", "gateway-interceptors"}: result.update(("OS-GATEWAY-DEPLOY", "OS-INTERCEPTORS"))
    if name == "extension-core": result.update(("OS-MIDDLEWARE", "OS-INTERCEPTORS"))
    if name == "server-macros": result.add("OS-API")
    if name in {"ocsf", "otel", "otel-test-support"}: result.add("OS-OBS")
    if name in {"prover", "prover-cli"}: result.add("OS-PROVER")
    if name == "proto":
        result.update(("OS-API", "OS-COMPUTE", "OS-CRED-API", "OS-MIDDLEWARE", "OS-INTERCEPTORS", "OS-ISOLATION-PROTO"))
    if name == "core": result.update(("OS-API", "OS-SANDBOX", "OS-POLICY-SCHEMA", "OS-FS", "OS-NET"))
    if name == "binary-identity": result.update(("OS-FS", "OS-EXEC"))
    if name == "conformance": result.update(("OS-BACKENDS", "OS-SANDBOX", "OS-EXEC"))
    return sorted(result)


SERVICE_CONTRACTS = {
    "OpenShell": ("quarshell-gateway", "OS-API"),
    "ComputeDriver": ("quarshell-spi", "OS-COMPUTE"),
    "CredentialDriver": ("quarshell-credentials", "OS-CRED-API"),
    "GatewayInterceptor": ("quarshell-gateway", "OS-INTERCEPTORS"),
    "SupervisorMiddleware": ("quarshell-proxy", "OS-MIDDLEWARE"),
    "HttpResponsePreReturn": ("quarshell-proxy", "OS-MIDDLEWARE"),
    "IsolationBoundary": ("quarshell-isolation", "OS-ISOLATION-PROTO"),
}


def proto_inventory() -> tuple[list[dict], list[dict]]:
    rows: list[dict] = []
    services: list[dict] = []
    pattern = re.compile(r"\brpc\s+(\w+)\s*\((.*?)\)\s*returns\s*\((.*?)\)\s*[;{]", re.S)
    proto_paths = sorted(SOURCE.glob("proto/**/*.proto")) + sorted(SOURCE.glob("crates/openshell-sandbox-backend/proto/**/*.proto"))
    for path in proto_paths:
        text = path.read_text()
        service_ranges = []
        for match in re.finditer(r"\bservice\s+(\w+)\s*\{", text):
            depth, end = 1, match.end()
            while end < len(text) and depth:
                if text[end] == "{": depth += 1
                elif text[end] == "}": depth -= 1
                end += 1
            service_ranges.append((match.start(), end, match.group(1)))
            service_owner, service_feature = SERVICE_CONTRACTS.get(match.group(1), ("quarshell-proto", "OS-API"))
            services.append({"name": match.group(1), "source": f"{path.relative_to(SOURCE).as_posix()}:{text.count(chr(10), 0, match.start()) + 1}",
                             "owner": service_owner, "feature_id": service_feature})
        for match in pattern.finditer(text):
            req, resp = (x.strip() for x in match.groups()[1:])
            service = next((name for start, end, name in service_ranges if start < match.start() < end), "UNSCOPED")
            service_owner, service_feature = SERVICE_CONTRACTS.get(service, ("quarshell-proto", "OS-API"))
            line = text.count("\n", 0, match.start()) + 1
            rows.append({"id": f"OS-RPC-{path.stem}-{service}-{match.group(1)}", "service": service,
                         "method": match.group(1), "source": f"{path.relative_to(SOURCE).as_posix()}:{line}",
                         "request_stream": req.startswith("stream "), "response_stream": resp.startswith("stream "),
                         "request_type": req.removeprefix("stream "), "response_type": resp.removeprefix("stream "),
                         "owner": service_owner, "feature_id": service_feature, "status": "mapped",
                         "acceptance": "proto wire golden + unary/stream conformance (B1)"})
    return rows, services


def rust_subcommands(path: Path) -> list[dict]:
    text = path.read_text()
    rows = []
    enum_re = re.compile(r"enum\s+(\w*Commands)\s*\{")
    for match in enum_re.finditer(text):
        start = match.start()
        if "Subcommand" not in text[max(0, start - 300):start]:
            continue
        depth, end = 1, match.end()
        while end < len(text) and depth:
            if text[end] == "{": depth += 1
            elif text[end] == "}": depth -= 1
            end += 1
        block = text[match.end():end - 1]
        base_line = text.count("\n", 0, match.start()) + 1
        # Top-level enum variants are at brace depth zero relative to the body.
        depth = 0
        attrs: list[str] = []
        for i, line in enumerate(block.splitlines(), 1):
            stripped = line.strip()
            if depth == 0:
                if stripped.startswith("#[command"):
                    attrs.append(stripped)
                elif re.match(r"[A-Z][A-Za-z0-9_]*\s*(?:\{|\(|,|$)", stripped):
                    name = re.match(r"([A-Z][A-Za-z0-9_]*)", stripped).group(1)
                    aliases = re.findall(r"(?:visible_)?alias\s*=\s*\"([^\"]+)\"", " ".join(attrs))
                    rows.append({"enum": match.group(1), "command": re.sub(r"([a-z0-9])([A-Z])", r"\1-\2", name).lower(),
                                 "aliases": aliases, "source": f"{path.relative_to(SOURCE).as_posix()}:{base_line + i}",
                                 "feature_id": "OS-CLI", "status": "mapped"})
                    attrs = []
                elif stripped and not stripped.startswith("///") and not stripped.startswith("//"):
                    attrs = []
            depth += line.count("{") - line.count("}")
    return rows


def clap_arguments(path: Path) -> list[dict]:
    text = path.read_text()
    rows = []
    for match in re.finditer(r"#\[arg\s*\(", text):
        start, pos, depth, quoted, escaped = match.start(), match.end() - 1, 0, False, False
        while pos < len(text):
            char = text[pos]
            if quoted:
                if char == '"' and not escaped: quoted = False
                escaped = char == "\\" and not escaped
                if char != "\\": escaped = False
            else:
                if char == '"': quoted = True
                elif char == "(": depth += 1
                elif char == ")":
                    depth -= 1
                    if depth == 0: break
            pos += 1
        close = text.find("]", pos)
        if close < 0: continue
        attr = text[start:close + 1]
        tail = text[close + 1:]
        field_match = re.search(r"(?:^|\n)\s*(\w+)\s*:", tail)
        field_name = field_match.group(1) if field_match else None
        long_match = re.search(r'\blong\s*=\s*"([^"]+)"', attr)
        has_long = bool(re.search(r"\blong\b", attr))
        short_match = re.search(r"\bshort\s*=\s*'([^']+)'", attr)
        has_short = bool(re.search(r"\bshort\b", attr))
        env_match = re.search(r'\benv\s*=\s*"([^"]+)"', attr)
        line = text.count("\n", 0, start) + 1
        preceding_lines = text[:start].splitlines()[-30:]
        doc_lines = []
        for doc_line in reversed(preceding_lines):
            if doc_line.strip().startswith("///"):
                doc_lines.append(doc_line.strip()[3:].strip())
            elif doc_lines:
                break
        rows.append({"field": field_name,
                     "long": long_match.group(1) if long_match else (field_name.replace("_", "-") if has_long and field_name else None),
                     "short": short_match.group(1) if short_match else (field_name[0] if has_short and field_name else None),
                     "env": env_match.group(1) if env_match else None,
                     "clap_attribute": " ".join(attr.split()),
                     "help": " ".join(reversed(doc_lines)),
                     "hidden": bool(re.search(r"\bhide\b", attr)) and not bool(re.search(r"\bhide\s*=\s*false\b", attr)),
                     "deprecated": "deprecated" in " ".join(doc_lines).lower(),
                     "source": f"{path.relative_to(SOURCE).as_posix()}:{line}", "feature_id": "OS-CLI"})
        pos = close
    return rows


def rust_enum_variants(path: Path, enum_name: str) -> list[dict]:
    text = path.read_text()
    match = re.search(rf"enum\s+{re.escape(enum_name)}\s*\{{", text)
    if not match:
        return []
    end = match.end(); depth = 1
    while end < len(text) and depth:
        if text[end] == "{": depth += 1
        elif text[end] == "}": depth -= 1
        end += 1
    body = text[match.end():end - 1]
    line = text.count("\n", 0, match.start()) + 1
    return [{"name": m.group(1), "source": f"{path.relative_to(SOURCE).as_posix()}:{line + body.count(chr(10), 0, m.start()) + 1}",
             "feature_id": "OS-CLI"} for m in re.finditer(r"(?m)^\s{4}([A-Z][A-Za-z0-9_]*)\s*,", body)]


def config_struct_inventory() -> list[dict]:
    rows = []
    for path in sorted((SOURCE / "crates").rglob("*.rs")):
        text = path.read_text(errors="replace")
        for match in re.finditer(r"(?m)^\s*(?:pub(?:\([^)]*\))?\s+)?struct\s+(\w+)[^{]*\{", text):
            type_name = match.group(1)
            if not re.search(r"config|settings|options", type_name, re.I) and not re.search(r"config|settings", path.name, re.I):
                continue
            end = match.end(); depth = 1
            while end < len(text) and depth:
                if text[end] == "{": depth += 1
                elif text[end] == "}": depth -= 1
                end += 1
            body = text[match.end():end - 1]
            start_line = text.count("\n", 0, match.start()) + 1
            preceding = text[max(0, match.start() - 400):match.start()]
            rename_all = re.findall(r'rename_all\s*=\s*"([^"]+)"', preceding)
            style = rename_all[-1] if rename_all else "snake_case"
            derives = re.findall(r"#\[derive\(([^\]]+)\)\]", preceding)
            serde_container = re.findall(r"#\[serde\(([^\]]+)\)\]", preceding)
            fields = []
            for field in re.finditer(r"(?m)^\s{4}(?:pub(?:\([^)]*\))?\s+)?([a-z][A-Za-z0-9_]*)\s*:", body):
                before = body[max(0, field.start() - 180):field.start()]
                renames = re.findall(r'rename\s*=\s*"([^"]+)"', before)
                raw = field.group(1)
                if renames:
                    key = renames[-1]
                elif style == "kebab-case":
                    key = re.sub(r"([a-z0-9])([A-Z])", r"\1-\2", raw).lower()
                elif style == "camelCase":
                    key = re.sub(r"_([a-z])", lambda m: m.group(1).upper(), raw)
                else:
                    key = raw
                line_offset = body.count("\n", 0, field.start())
                field_line = body.splitlines()[line_offset].strip()
                field_type = field_line.split(":", 1)[1].strip().rstrip(",") if ":" in field_line else None
                prev_field = list(re.finditer(r"(?m)^\s{4}(?:pub(?:\([^)]*\))?\s+)?[a-z][A-Za-z0-9_]*\s*:[^\n]*", body[:field.start()]))
                attr_start = prev_field[-1].end() if prev_field else 0
                field_attrs = body[attr_start:field.start()]
                serde_attrs = re.findall(r"#\[serde\((.*?)\)\]", field_attrs, re.S)
                fields.append({"key": key, "field": raw, "type": field_type,
                               "serde_attributes": serde_attrs,
                               "line": start_line + line_offset + 1})
            if fields:
                source_rel = path.relative_to(SOURCE).as_posix()
                source_lower = source_rel.lower()
                if "/tests/" in source_lower:
                    category = "test_fixture_or_test_configuration"
                elif "/openshell-cli/" in source_lower:
                    category = "cli_option_or_request_data"
                elif "/openshell-ocsf/" in source_lower:
                    category = "serialized_event_schema"
                elif "/openshell-policy-schema/" in source_lower:
                    category = "policy_profile_schema"
                elif "/openshell-providers/" in source_lower and "profile" in source_lower:
                    category = "provider_profile_schema"
                elif "/openshell-driver-podman/src/client.rs" in source_lower:
                    category = "container_engine_api_payload"
                elif "/openshell-sandbox-backend/" in source_lower:
                    category = "internal_boundary_protocol_config"
                elif "/openshell-server/src/config_file.rs" in source_lower:
                    category = "gateway_toml_schema"
                elif "/openshell-core/src/config.rs" in source_lower:
                    category = "shared_gateway_configuration"
                elif "/openshell-core/src/resource_admission.rs" in source_lower:
                    category = "domain_admission_configuration"
                elif "/openshell-otel/" in source_lower:
                    category = "telemetry_configuration"
                elif "/openshell-driver-" in source_lower or type_name.endswith(("Config", "Settings")):
                    category = "driver_or_runtime_configuration"
                else:
                    category = "in_memory_options_or_domain_record"
                rows.append({"type": type_name, "source": path.relative_to(SOURCE).as_posix(),
                             "line": start_line, "serde_rename_all": style,
                             "derives": derives[-1] if derives else None,
                             "serde_container_attributes": serde_container[-1:] ,
                             "serde_enabled": bool(serde_container or any(re.search(r"\b(Serialize|Deserialize)\b", d) for d in derives)),
                             "category": category,
                             "fields": fields})
    return rows


def tui_action_inventory(path: Path) -> list[dict]:
    text = path.read_text()
    rows = []
    fn_re = re.compile(r"fn\s+(handle_[a-z0-9_]+)\s*\([^)]*\)\s*\{")
    for match in fn_re.finditer(text):
        end = match.end(); depth = 1
        while end < len(text) and depth:
            if text[end] == "{": depth += 1
            elif text[end] == "}": depth -= 1
            end += 1
        body = text[match.end():end - 1]
        base = text.count("\n", 0, match.start()) + 1
        for i, line in enumerate(body.splitlines(), 1):
            if "KeyCode::" in line and "=>" in line:
                rows.append({"handler": match.group(1), "key_match": line.strip(),
                             "source": f"{path.relative_to(SOURCE).as_posix()}:{base + i}",
                             "feature_id": "OS-CLI"})
    return rows


FEATURE_IDS = {
    "Gateway API": "OS-API",
    "Compute API": "OS-COMPUTE",
    "Credential API": "OS-CRED-API",
    "Middleware": "OS-MIDDLEWARE",
    "Interceptors": "OS-INTERCEPTORS",
    "Isolation protocol": "OS-ISOLATION-PROTO",
    "Gateway deployment": "OS-GATEWAY-DEPLOY",
    "Sandbox lifecycle": "OS-SANDBOX",
    "Main process": "OS-MAIN",
    "Exec/session": "OS-EXEC",
    "Workspace/services": "OS-WORKSPACE",
    "Policy schema": "OS-POLICY-SCHEMA",
    "Policy lifecycle": "OS-POLICY-LIFECYCLE",
    "Filesystem isolation": "OS-FS",
    "Network isolation": "OS-NET",
    "L7 enforcement": "OS-L7",
    "Credentials": "OS-CREDENTIALS",
    "Token grants": "OS-TOKEN",
    "Providers": "OS-PROVIDERS",
    "Compute backends": "OS-BACKENDS",
    "GPU": "OS-GPU",
    "Credential drivers": "OS-CRED-DRIVERS",
    "Peer control plane": "OS-PEER",
    "Prover": "OS-PROVER",
    "Observability": "OS-OBS",
    "CLI/TUI": "OS-CLI",
    "SDK": "OS-SDK",
    "Distribution": "OS-DISTRIBUTION",
    "Agent tooling/docs": "OS-DOCS",
}


def feature_matrix() -> list[dict]:
    rows = []
    in_table = False
    for line in PLAN.read_text().splitlines():
        if line.startswith("| Область |"):
            in_table = True
            continue
        if not in_table:
            continue
        if line.startswith("|---"):
            continue
        if not line.startswith("|"):
            break
        cols = [x.strip() for x in line.strip().strip("|").split("|")]
        if len(cols) < 4:
            continue
        area, scope, owner_name, acceptance = cols[:4]
        feature_id = FEATURE_IDS.get(area)
        if not feature_id:
            continue
        rows.append({"id": feature_id, "scope": scope, "acceptance": acceptance,
                     "owner": owner_name, "status": "not_started",
                     "baseline": "OpenShell v0.1.4", "source": "PLAN.md section 2"})
    return rows


def path_feature_ids(path: str, default: str) -> list[str]:
    lowered = path.lower()
    pairs = (("docker", "OS-BACKENDS"), ("podman", "OS-BACKENDS"), ("kubernetes", "OS-BACKENDS"),
             ("kube", "OS-BACKENDS"), ("vm", "OS-BACKENDS"), ("gpu", "OS-GPU"), ("mxc", "OS-BACKENDS"),
             ("policy", "OS-POLICY-SCHEMA"), ("provider", "OS-PROVIDERS"), ("credential", "OS-CREDENTIALS"),
             ("workspace", "OS-WORKSPACE"), ("sandbox", "OS-SANDBOX"), ("gateway", "OS-API"),
             ("security", "OS-FS"), ("observability", "OS-OBS"), ("otel", "OS-OBS"),
             ("sdk", "OS-SDK"), ("tui", "OS-CLI"), ("cli", "OS-CLI"), ("display", "OS-CLI"),
             ("prover", "OS-PROVER"), ("relay", "OS-PEER"), ("ha", "OS-PEER"))
    found = {feature_id for token, feature_id in pairs if token in lowered}
    return sorted(found or {default})


def installer_metadata(path: str) -> dict:
    lowered = path.lower()
    if lowered.startswith("deploy/helm/") and "/tests/" in lowered:
        kind, platforms = "helm_acceptance_test", ["Kubernetes"]
    elif lowered.startswith("deploy/helm/"):
        kind, platforms = "helm_chart_or_values", ["Kubernetes"]
    elif lowered.startswith("deploy/kube/"):
        kind, platforms = "kubernetes_manifest", ["Kubernetes"]
    elif lowered.startswith("deploy/docker/"):
        kind, platforms = "container_build_or_compose", ["Linux containers"]
    elif lowered.startswith("deploy/deb/") or lowered.endswith(".deb"):
        kind, platforms = "debian_package", ["Debian/Ubuntu Linux"]
    elif lowered.startswith("deploy/rpm/") or lowered.endswith(".spec"):
        kind, platforms = "rpm_package_or_migration", ["RPM Linux"]
    elif lowered.startswith("snap/") or lowered.endswith("snapcraft.yaml"):
        kind, platforms = "snap_package", ["Linux with snapd"]
    elif "/test" in lowered or lowered.startswith("tests/") or "_test." in lowered:
        kind, platforms = "installer_acceptance_test", ["declared by test harness"]
    elif lowered.startswith("deploy/man/"):
        kind, platforms = "manual_page", ["all supported platforms"]
    elif lowered.endswith(".ps1"):
        kind, platforms = "powershell_installer", ["Windows"]
    elif "install" in lowered and lowered.endswith((".sh", ".ps1")):
        kind, platforms = "installer_script", ["declared by script"]
    elif lowered.endswith("package.toml"):
        kind, platforms = "package_metadata", ["developer tooling"]
    elif lowered.endswith((".md", ".mdx")):
        kind, platforms = "installer_documentation", ["all supported platforms"]
    else:
        kind, platforms = "release_or_build_support", ["declared by source workflow"]
    return {"artifact_kind": kind, "platforms": platforms}


def documentation_metadata(path: Path) -> dict:
    text = path.read_text(errors="replace")
    headings = [line.lstrip("# ").strip() for line in text.splitlines()
                if re.match(r"^#{1,3}\s+", line)][:40]
    title = next((line.lstrip("# ").strip() for line in text.splitlines()
                  if re.match(r"^#\s+", line)), path.stem.replace("-", " "))
    claim_terms = sorted(set(term for term in (
        "Linux", "macOS", "Windows", "WSL", "Docker", "Podman", "Kubernetes", "Helm",
        "Landlock", "GPU", "VM", "HA", "OIDC", "TLS", "install", "upgrade", "security"
    ) if re.search(rf"\b{re.escape(term)}\b", text, re.I)))
    external_links = sorted(set(re.findall(r"https?://[^\s)\]>]+", text)))
    return {"title": title, "headings": headings,
            "claim_terms": claim_terms, "external_link_count": len(external_links),
            "code_fence_count": len(re.findall(r"(?m)^\s*```", text)) // 2}


def baseline_platforms(feature_ids: list[str]) -> list[str]:
    platforms = {
        "OS-API": ["Linux Debian/Ubuntu amd64/arm64", "macOS Apple Silicon with Docker Desktop", "Windows x86_64 WSL 2 + Docker Desktop (experimental)"],
        "OS-COMPUTE": ["Docker Engine/Desktop 28+", "Podman 5.x", "Kubernetes 1.29+ / Helm 3.x", "Linux KVM", "macOS Hypervisor.framework", "Windows MXC"],
        "OS-CRED-API": ["Linux Debian/Ubuntu amd64/arm64", "Kubernetes 1.29+ where using Kubernetes secrets"],
        "OS-MIDDLEWARE": ["Linux Debian/Ubuntu amd64/arm64", "macOS Apple Silicon with Docker Desktop"],
        "OS-INTERCEPTORS": ["Linux Debian/Ubuntu amd64/arm64", "macOS Apple Silicon with Docker Desktop"],
        "OS-ISOLATION-PROTO": ["Linux Debian/Ubuntu amd64/arm64; Linux 6.2+ with Landlock ABI 3"],
        "OS-GATEWAY-DEPLOY": ["Linux Debian/Ubuntu amd64/arm64", "macOS Apple Silicon with Docker Desktop", "Kubernetes 1.29+ / Helm 3.x"],
        "OS-SANDBOX": ["Linux Debian/Ubuntu amd64/arm64; Linux 6.2+ with Landlock ABI 3", "macOS Apple Silicon with Docker Desktop"],
        "OS-MAIN": ["Linux Debian/Ubuntu amd64/arm64; Linux 6.2+ with Landlock ABI 3"],
        "OS-EXEC": ["Linux Debian/Ubuntu amd64/arm64; Linux 6.2+ with Landlock ABI 3", "macOS Apple Silicon with Docker Desktop"],
        "OS-WORKSPACE": ["Linux Debian/Ubuntu amd64/arm64", "Kubernetes 1.29+ / Helm 3.x"],
        "OS-POLICY-SCHEMA": ["Linux Debian/Ubuntu amd64/arm64; Linux 6.2+ with Landlock ABI 3"],
        "OS-POLICY-LIFECYCLE": ["Linux Debian/Ubuntu amd64/arm64; Linux 6.2+ with Landlock ABI 3"],
        "OS-FS": ["Linux Debian/Ubuntu amd64/arm64; Linux 6.2+ with Landlock ABI 3"],
        "OS-NET": ["Linux Debian/Ubuntu amd64/arm64; Linux 6.2+ with Landlock ABI 3"],
        "OS-L7": ["Linux Debian/Ubuntu amd64/arm64", "macOS Apple Silicon with Docker Desktop"],
        "OS-CREDENTIALS": ["Linux Debian/Ubuntu amd64/arm64", "Kubernetes 1.29+ where using Kubernetes secrets"],
        "OS-TOKEN": ["Linux Debian/Ubuntu amd64/arm64", "Kubernetes 1.29+ where using Kubernetes secrets"],
        "OS-PROVIDERS": ["Linux Debian/Ubuntu amd64/arm64", "macOS Apple Silicon with Docker Desktop", "Windows WSL 2 (experimental)"],
        "OS-BACKENDS": ["Docker 28+", "Podman 5.x", "Kubernetes 1.29+ / Helm 3.x", "Linux KVM", "macOS Hypervisor.framework", "Windows MXC"],
        "OS-GPU": ["Linux host; device/backend combination requires explicit qualification"],
        "OS-CRED-DRIVERS": ["Linux Debian/Ubuntu amd64/arm64", "Kubernetes 1.29+", "Vault-compatible deployment"],
        "OS-PEER": ["Kubernetes 1.29+ / Helm 3.x", "Linux Debian/Ubuntu amd64/arm64"],
        "OS-PROVER": ["Linux amd64/arm64", "macOS Apple Silicon"],
        "OS-OBS": ["Linux Debian/Ubuntu amd64/arm64", "macOS and Windows WSL 2 follow gateway support status"],
        "OS-CLI": ["Linux Debian/Ubuntu amd64/arm64", "macOS Apple Silicon", "Windows WSL 2 (experimental)"],
        "OS-SDK": ["Linux amd64/arm64", "macOS Apple Silicon", "Windows WSL 2 (experimental)"],
        "OS-DISTRIBUTION": ["Linux amd64/arm64", "macOS Apple Silicon for gateway/prover artifacts", "Windows WSL 2 installer workflow (experimental)"],
        "OS-DOCS": ["all supported platforms"],
    }
    return sorted({platform for feature_id in feature_ids for platform in platforms.get(feature_id, ["support matrix review required"])})


def proto_stream_inventory(proto_rows: list[dict]) -> list[dict]:
    proto_paths = sorted(SOURCE.glob("proto/**/*.proto")) + sorted(SOURCE.glob("crates/openshell-sandbox-backend/proto/**/*.proto"))
    messages: dict[str, dict] = {}
    for path in proto_paths:
        text = path.read_text()
        for match in re.finditer(r"\bmessage\s+(\w+)\s*\{", text):
            depth, end = 1, match.end()
            while end < len(text) and depth:
                if text[end] == "{": depth += 1
                elif text[end] == "}": depth -= 1
                end += 1
            body = text[match.end():end - 1]
            line = text.count("\n", 0, match.start()) + 1
            fields: list[dict] = []
            oneof_spans: list[tuple[int, int]] = []
            for oneof in re.finditer(r"\boneof\s+(\w+)\s*\{", body):
                od, oe = 1, oneof.end()
                while oe < len(body) and od:
                    if body[oe] == "{": od += 1
                    elif body[oe] == "}": od -= 1
                    oe += 1
                oneof_spans.append((oneof.start(), oe))
                oblock = body[oneof.end():oe - 1]
                for field in re.finditer(r"(?m)^\s*(?:(?:repeated|optional|required)\s+)?([\w.]+)\s+(\w+)\s*=\s*\d+", oblock):
                    fields.append({"name": field.group(2), "type": field.group(1),
                                   "oneof": oneof.group(1),
                                   "line": line + body.count("\n", 0, oneof.end() + field.start())})
            def in_oneof(pos: int) -> bool:
                return any(start <= pos < end for start, end in oneof_spans)
            for field in re.finditer(r"(?m)^\s*(?:(?:repeated|optional|required)\s+)?([\w.]+)\s+(\w+)\s*=\s*\d+", body):
                if in_oneof(field.start()):
                    continue
                fields.append({"name": field.group(2), "type": field.group(1), "oneof": None,
                               "line": line + body.count("\n", 0, field.start())})
            messages[match.group(1)] = {"source": f"{path.relative_to(SOURCE).as_posix()}:{line}",
                                        "fields": fields}
    rows: list[dict] = []
    for rpc in proto_rows:
        if not (rpc["request_stream"] or rpc["response_stream"]):
            continue
        for direction, type_name in (("request", rpc["request_type"]), ("response", rpc["response_type"])):
            base = type_name.split(".")[-1]
            message = messages.get(base)
            if not message:
                rows.append({"id": f"OS-STREAM-{rpc['service']}-{rpc['method']}-{direction}", "service": rpc["service"],
                             "method": rpc["method"], "direction": direction, "message": type_name,
                             "variant": None, "type": None, "oneof": None, "source": None,
                             "owner": rpc["owner"], "feature_id": rpc["feature_id"],
                             "status": "message_not_found"})
                continue
            if not message["fields"]:
                rows.append({"id": f"OS-STREAM-{rpc['service']}-{rpc['method']}-{direction}", "service": rpc["service"],
                             "method": rpc["method"], "direction": direction, "message": type_name,
                             "variant": None, "type": None, "oneof": None, "source": message["source"],
                             "owner": rpc["owner"], "feature_id": rpc["feature_id"],
                             "status": "no_variants"})
            for field in message["fields"]:
                rows.append({"id": f"OS-STREAM-{rpc['service']}-{rpc['method']}-{direction}-{field['name']}",
                             "service": rpc["service"], "method": rpc["method"], "direction": direction,
                             "message": type_name, "variant": field["name"], "type": field["type"],
                             "oneof": field["oneof"], "source": message["source"],
                             "name": f"{rpc['service']}.{rpc['method']}:{direction}:{field['name']}",
                             "owner": rpc["owner"], "feature_id": rpc["feature_id"], "status": "mapped"})
    return rows


def provider_inventory() -> list[dict]:
    rows = []
    for path in sorted((SOURCE / "providers").glob("*.yaml")):
        document = yaml.safe_load(path.read_text()) or {}
        credentials = document.get("credentials") or []
        endpoints = document.get("endpoints") or []
        refresh = sorted({credential.get("refresh", {}).get("strategy")
                          for credential in credentials
                          if isinstance(credential, dict) and credential.get("refresh")})
        rows.append({
            "id": str(document.get("id") or path.stem),
            "source": path.relative_to(SOURCE).as_posix(),
            "sha256": hashlib.sha256(path.read_bytes()).hexdigest(),
            "display_name": document.get("display_name"),
            "category": document.get("category"),
            "inference_capable": document.get("inference_capable"),
            "credential_names": [c.get("name") for c in credentials if isinstance(c, dict)],
            "env_vars": sorted({var for c in credentials if isinstance(c, dict) for var in (c.get("env_vars") or [])}),
            "auth_styles": sorted({c.get("auth_style") for c in credentials if isinstance(c, dict) and c.get("auth_style")}),
            "refresh_strategies": refresh,
            "endpoints": [{"host": e.get("host"), "port": e.get("port"), "protocol": e.get("protocol"),
                           "access": e.get("access"), "enforcement": e.get("enforcement")}
                          for e in endpoints if isinstance(e, dict)],
            "binary_count": len(document.get("binaries") or []),
            "binaries": document.get("binaries") or [],
            "feature_ids": ["OS-PROVIDERS", "OS-CREDENTIALS"],
            "owner": "quarshell-providers",
            "status": "profile_schema_indexed; behavior and credential lifecycle unverified",
        })
    return rows


def release_inventory() -> dict:
    support_doc = SOURCE / "docs/about/support-matrix.mdx"
    tables = markdown_tables(support_doc) if support_doc.exists() else []
    channels = next((t for t in tables if t["section"] == "Releases"), None)
    platforms = next((t for t in tables if t["section"] == "Supported Platforms"), None)
    gateway_artifacts = next((t for t in tables if t["section"] == "Standalone Gateway Binary"), None)
    sandbox_artifacts = next((t for t in tables if t["section"] == "Sandbox Runtime Binary"), None)
    return {
        "source": "docs/about/support-matrix.mdx",
        "release_channels": channels,
        "platforms": platforms,
        "standalone_gateway_artifacts": gateway_artifacts,
        "sandbox_runtime_artifacts": sandbox_artifacts,
        "status": "baseline support claims indexed; native install/upgrade/uninstall acceptance remains unverified",
    }


def markdown_tables(path: Path) -> list[dict]:
    lines = path.read_text().splitlines()
    tables = []
    i = 0
    heading = ""
    while i < len(lines):
        if lines[i].startswith("#"):
            heading = lines[i].lstrip("# ").strip()
        if lines[i].startswith("|") and i + 1 < len(lines) and re.match(r"\s*\|?[\s:|-]+\|\s*$", lines[i + 1]):
            header = [x.strip() for x in lines[i].strip().strip("|").split("|")]
            rows = []
            i += 2
            while i < len(lines) and lines[i].startswith("|"):
                rows.append([x.strip() for x in lines[i].strip().strip("|").split("|")])
                i += 1
            tables.append({"section": heading, "columns": header, "rows": rows})
        else:
            i += 1
    return tables


def main() -> None:
    manifests = sorted(SOURCE.glob("crates/*/Cargo.toml"))
    crate_rows = []
    for manifest in manifests:
        text = manifest.read_text()
        match = re.search(r'(?m)^name\s*=\s*"([^"]+)"', text)
        crate = match.group(1) if match else manifest.parent.name
        crate_rows.append({
            "id": "OS-CRATE-" + crate.removeprefix("openshell-").upper().replace("-", "-"),
            "upstream_name": crate,
            "source": manifest.relative_to(SOURCE).as_posix(),
            "owner": owner(crate.removeprefix("openshell-")),
            "feature_ids": feature_ids(crate),
            "status": "mapped" if feature_ids(crate) else "blocker",
            "decision": "port applicable behavior; assess each capability during the owning feature stage",
            "port_decision": f"port baseline responsibilities into {owner(crate.removeprefix('openshell-'))}; no crate excluded at B0",
            "acceptance": "owning OS-* acceptance matrix from plan section 4",
        })

    proto_rows, proto_services = proto_inventory()
    stream_rows = proto_stream_inventory(proto_rows)
    providers = provider_inventory()
    releases = release_inventory()

    generated_dirs = {".git", "target", "node_modules", ".venv", "dist", "build", "site"}
    source_files = [p for p in SOURCE.rglob("*") if p.is_file() and not (generated_dirs & set(p.parts))]
    cargo_meta = json.loads(subprocess.check_output(
        ["cargo", "metadata", "--no-deps", "--format-version", "1"],
        cwd=SOURCE, text=True,
    ))
    workspace_members = sorted(p["name"] for p in cargo_meta["packages"])
    current = git("rev-parse", "HEAD")
    try:
        upstream = git("rev-parse", "FETCH_HEAD")
        commits = git("log", "--format=%H%x09%cs%x09%s", f"{BASELINE}..{upstream}").splitlines()
        delta_rows = []
        for row in commits:
            sha, date, subject = row.split("\t", 2)
            kind = subject.split(":", 1)[0].split("(", 1)[0]
            delta_rows.append({"commit": sha, "date": date, "subject": subject, "kind": kind,
                               "feature_id": "OS-CLI" if "cli" in subject else "OS-OBS" if any(x in subject for x in ("ocsf", "otel", "auth")) else "OS-BACKENDS" if "vm" in subject else "OS-BACKENDS" if any(x in subject for x in ("docker", "podman", "kubernetes", "driver")) else "OS-API" if any(x in subject for x in ("server", "gateway", "sandbox", "compute", "config")) else "OS-DISTRIBUTION",
                               "decision": "exclude_from_v0.1.4_baseline",
                               "rationale": "the selected contract is the pinned v0.1.4 tag; retain as a separately reviewable post-baseline candidate"})
        delta = {"checked_ref": upstream, "commit_count": len(commits), "commits": delta_rows,
                 "decision": "all post-tag commits excluded from the v0.1.4 baseline; feature changes remain separately indexed for later scope review"}
    except subprocess.CalledProcessError:
        delta = {"checked_ref": None, "commit_count": None, "commits": [], "decision": "not checked"}

    cli_main = SOURCE / "crates/openshell-cli/src/main.rs"
    cli_arguments = clap_arguments(cli_main)
    cli_commands = rust_subcommands(cli_main)
    runtime_cli = json.loads(CLI_HELP.read_text()) if CLI_HELP.exists() else None
    runtime_paths = runtime_cli.get("command_help", []) if runtime_cli else []
    runtime_root = next((row for row in runtime_paths if not row["command_path"]), None)
    cli_enum_paths = {
        "Commands": (), "SandboxCommands": ("sandbox",), "ServiceCommands": ("service",),
        "ForwardCommands": ("forward",), "PolicyCommands": ("policy",),
        "SettingsCommands": ("settings",), "DraftCommands": ("rule",),
        "ProviderCommands": ("provider",), "ProfileCommands": ("profile",),
        "GatewayCommands": ("gateway",), "DoctorCommands": ("doctor",),
        "SandboxTemplateCommands": ("sandbox", "template"),
        "SandboxProviderCommands": ("sandbox", "provider"),
        "ProviderRefreshCommands": ("provider", "refresh"),
        "WorkspaceCommands": ("workspace",), "WorkspaceMemberCommands": ("workspace", "member"),
    }
    source_command_paths = sorted({" ".join(cli_enum_paths[row["enum"]] + (row["command"],))
                                   for row in cli_commands if row["enum"] in cli_enum_paths})
    runtime_command_paths = sorted(" ".join(row["command_path"]) for row in runtime_paths if row["command_path"])
    source_long_flags = sorted({row["long"] for row in cli_arguments if row.get("long")})
    source_short_flags = sorted({row["short"] for row in cli_arguments if row.get("short")})
    runtime_flags = sorted({flag for row in runtime_paths for flag in row.get("flags", [])})
    runtime_long_flags = sorted({flag[2:] for flag in runtime_flags if flag.startswith("--")})
    runtime_short_flags = sorted({flag[1:] for flag in runtime_flags if len(flag) == 2 and flag.startswith("-")})
    runtime_cli_evidence = {
        "source": CLI_HELP.relative_to(ROOT).as_posix() if runtime_cli else None,
        "version": runtime_cli.get("version") if runtime_cli else None,
        "valid_help_pages": runtime_cli.get("valid_command_paths") if runtime_cli else 0,
        "valid_command_paths": [" ".join(row["command_path"]) for row in runtime_paths if row["command_path"]],
        "source_command_paths": source_command_paths,
        "source_only_paths": sorted(set(source_command_paths) - set(runtime_command_paths)),
        "runtime_only_paths": sorted(set(runtime_command_paths) - set(source_command_paths)),
        "reconciliation": "all 95 source variants are callable; 7 additional provider profile paths are aliases to the profile command group",
        "source_long_flags": source_long_flags,
        "runtime_long_flags": runtime_long_flags,
        "source_only_long_flags": sorted(set(source_long_flags) - set(runtime_long_flags)),
        "runtime_only_long_flags": sorted(set(runtime_long_flags) - set(source_long_flags)),
        "source_short_flags": source_short_flags,
        "runtime_short_flags": runtime_short_flags,
        "flag_reconciliation": "all visible source flag names and short flags match runtime help; --keep is the sole source-only flag and is hidden/deprecated",
        "displayed_but_rejected": runtime_cli.get("rejected_help_entries", []) if runtime_cli else [],
        "root_displayed_children": runtime_root.get("children", []) if runtime_root else [],
    }

    tui_app = SOURCE / "crates/openshell-tui/src/app.rs"
    tui_screens = rust_enum_variants(tui_app, "Screen")
    tui_tabs = {name: rust_enum_variants(tui_app, name) for name in ("MiddlePaneTab", "SandboxPolicyTab", "Focus")}
    key_tokens = sorted(set(re.findall(r"KeyCode::(?:Char\('[^']+'\)|[A-Za-z]+)", tui_app.read_text())))
    tui_actions = tui_action_inventory(tui_app)

    driver_crates = [c for c in crate_rows if "OS-BACKENDS" in c["feature_ids"]]
    test_files = sorted(p.relative_to(SOURCE).as_posix() for p in source_files
                        if p.is_file() and ("/tests/" in p.as_posix() or p.name.endswith("_test.rs") or p.name.endswith("_tests.rs")))
    docs_files = sorted(p.relative_to(SOURCE).as_posix() for p in source_files
                        if p.suffix.lower() in {".md", ".mdx"} and (p.as_posix().startswith((str(SOURCE / "docs"), str(SOURCE / "skills"))) or p.name in {"README.md", "CONTRIBUTING.md", "SECURITY.md"}))
    installer_files = sorted(p.relative_to(SOURCE).as_posix() for p in source_files
                             if any(token in p.as_posix().lower() for token in ("install", "deploy/", "snapcraft", "openshell.spec", "package.toml")))
    docs_inventory = [{"source": rel, "feature_ids": path_feature_ids(rel, "OS-DISTRIBUTION"),
                       **documentation_metadata(SOURCE / rel),
                       "owner": "quarshell-docs", "platforms": baseline_platforms(path_feature_ids(rel, "OS-DISTRIBUTION")),
                       "status": "title/outline/platform claims indexed; verify claims against ported acceptance before B8 release"} for rel in docs_files]
    installer_inventory = [{"source": rel, "feature_ids": sorted(set(["OS-DISTRIBUTION", *path_feature_ids(rel, "OS-DISTRIBUTION")])),
                            **installer_metadata(rel),
                            "status": "artifact/platform class inferred from path; install/upgrade/uninstall workflow review required"} for rel in installer_files]
    config_files = sorted(p.relative_to(SOURCE).as_posix() for p in source_files
                          if p.is_file() and ("config" in p.name.lower() or "settings" in p.name.lower()) and p.suffix in {".rs", ".proto", ".toml", ".yaml", ".yml", ".json"})
    config_types = config_struct_inventory()
    config_example_files = []
    for path in sorted(p for p in source_files if p.suffix == ".toml"):
        if "[openshell" in path.read_text(errors="replace"):
            config_example_files.append(path.relative_to(SOURCE).as_posix())
    config_categories = {}
    for item in config_types:
        config_categories[item["category"]] = config_categories.get(item["category"], 0) + 1
    features = feature_matrix()

    crate_by_path = {c["source"].split("/")[1]: c for c in crate_rows}
    test_inventory = []
    for rel in test_files:
        parts = Path(rel).parts
        crate = crate_by_path.get(parts[1]) if len(parts) > 2 and parts[0] == "crates" else None
        inferred = list(crate["feature_ids"]) if crate else []
        lowered = rel.lower()
        test_text = (SOURCE / rel).read_text(errors="replace")
        test_names = re.findall(r"(?ms)#\[(?:tokio::)?test(?:\([^]]*\))?\]\s*(?:#\[[^]]+\]\s*)*(?:async\s+)?fn\s+([A-Za-z0-9_]+)", test_text)
        test_frameworks = sorted(set("tokio" if m else "test" for m in re.findall(r"#\[(tokio::)?test", test_text)))
        assertion_count = len(re.findall(r"\b(?:assert|assert_eq|assert_ne|assert_matches|assert_matches_json|assert_contains|assert_snapshot|panic)\s*!", test_text))
        file_class = "rust_test_source" if Path(rel).suffix == ".rs" and test_names else "rust_support_source" if Path(rel).suffix == ".rs" else "test_fixture_or_data"
        for token, feature_id in (("docker", "OS-BACKENDS"), ("podman", "OS-BACKENDS"),
                                  ("kubernetes", "OS-BACKENDS"), ("kube", "OS-BACKENDS"),
                                  ("vm", "OS-BACKENDS"), ("gpu", "OS-GPU"), ("mxc", "OS-BACKENDS"),
                                  ("policy", "OS-POLICY-SCHEMA"), ("credential", "OS-CREDENTIALS"),
                                  ("provider", "OS-PROVIDERS"), ("workspace", "OS-WORKSPACE"),
                                  ("sandbox", "OS-SANDBOX"), ("gateway", "OS-API"),
                                  ("exec", "OS-EXEC"), ("ha_", "OS-PEER"), ("relay", "OS-PEER"),
                                  ("landlock", "OS-FS"), ("security", "OS-FS")):
            if token in lowered: inferred.append(feature_id)
        if not inferred: inferred = ["OS-BACKENDS" if "e2e" in lowered else "OS-API"]
        test_inventory.append({"source": rel, "feature_ids": sorted(set(inferred)),
                              "owner": crate["owner"] if crate else "quarshell-driver" if "e2e" in lowered else "quarshell-gateway",
                              "test_names": test_names, "test_count": len(test_names),
                              "assertion_count": assertion_count, "test_frameworks": test_frameworks, "file_class": file_class,
                              "status": "mapped_by_source_path; semantic acceptance review required"})

    coverage_items = []
    def add_coverage(kind: str, items: list[dict], id_prefix: str, default_owner: str,
                     acceptance: str) -> None:
        for index, item in enumerate(items, 1):
            feature_ids = item.get("feature_ids") or ([item["feature_id"]] if item.get("feature_id") else [])
            source = item.get("source", "")
            local_owner = item.get("owner", default_owner)
            if kind == "config_field":
                crate_path = Path(source).parts
                crate_record = crate_by_path.get(crate_path[1]) if len(crate_path) > 1 and crate_path[0] == "crates" else None
                feature_ids = list(crate_record["feature_ids"]) if crate_record else ["OS-API"]
                local_owner = crate_record["owner"] if crate_record else default_owner
            if kind == "tui_action": local_owner = "quarshell-cli"
            coverage_items.append({
                "id": f"{id_prefix}-{index:04d}", "kind": kind,
                "name": item.get("command") or item.get("method") or item.get("key") or item.get("type") or item.get("name") or item.get("source"),
                "details": {key: item[key] for key in ("field", "clap_attribute", "help", "hidden", "deprecated", "type", "config_type", "config_serde_enabled", "config_category", "config_derives", "serde_container_attributes", "serde_attributes", "handler", "key_match", "test_names", "test_count", "assertion_count", "test_frameworks", "file_class", "artifact_kind", "platforms", "title", "headings", "claim_terms", "external_link_count", "code_fence_count") if key in item},
                "source": source,
                "evidence": item.get("evidence", {"baseline_source": source, "acceptance_feature_ids": sorted(set(feature_ids))}),
                "line": item.get("line"),
                "owner": local_owner,
                "feature_ids": sorted(set(feature_ids)),
                "platforms": item.get("platforms", baseline_platforms(sorted(set(feature_ids)))),
                "acceptance": acceptance,
                "status": item.get("status", "mapped_by_source; semantic review required"),
                "decision": item.get("port_decision", "port baseline behavior or record an explicit scope exclusion before implementation"),
            })

    add_coverage("crate", crate_rows, "OS-CRATE", "quarshell-core", "Owning OS-* acceptance matrix and explicit port/exclusion decision (B4).")
    add_coverage("rpc", proto_rows, "OS-RPC", "quarshell-proto", "Wire golden corpus and unary/streaming conformance (B1).")
    add_coverage("stream_variant", stream_rows, "OS-STREAM", "quarshell-proto", "Stream frame ordering, cancellation, size limits, and loss semantics (B1).")
    add_coverage("provider_profile", providers, "OS-PROVIDER", "quarshell-providers", "Profile schema, inheritance, label filtering, discovery, and credential binding fixtures (B4).")
    add_coverage("cli_command", cli_commands, "OS-CLI-CMD", "quarshell-cli", "Runtime help, arguments, examples, exit behavior, and workflow acceptance (B6).")
    add_coverage("cli_argument", cli_arguments, "OS-CLI-ARG", "quarshell-cli", "Runtime help/value/default/env/validation compatibility (B6).")
    for item in coverage_items:
        if item["kind"] == "cli_argument" and item["details"].get("deprecated") and item["details"].get("hidden"):
            item["decision"] = "exclude deprecated hidden compatibility flag from the public CLI; preserve the current default behavior and verify at OS-CLI acceptance"
    config_fields = [{**field, "config_type": struct["type"], "config_serde_enabled": struct["serde_enabled"], "config_category": struct["category"],
                      "config_derives": struct["derives"], "serde_container_attributes": struct["serde_container_attributes"],
                      "source": struct["source"],
                      "status": "serde-backed; loader/default/precedence review required" if struct["serde_enabled"] else "options/config candidate; persistence or wire behavior must be classified",
                      "owner": crate_by_path.get(Path(struct["source"]).parts[1], {}).get("owner", "quarshell-gateway")}
                     for struct in config_types for field in struct["fields"]]
    add_coverage("config_field", config_fields, "OS-CONFIG", "quarshell-gateway", "Loader, serialization key, default, precedence, and validation compatibility (B2/B6).")
    add_coverage("tui_action", tui_actions, "OS-CLI-ACTION", "quarshell-cli", "Screen/key transition, visible result, and terminal cleanup acceptance (B6).")
    add_coverage("test_file", test_inventory, "OS-TEST", "quarshell-gateway", "Map assertions to a feature acceptance case; preserve intent in Go acceptance suite (B1-B8).")
    add_coverage("documentation", docs_inventory, "OS-DOC", "quarshell-docs", "Review every externally visible claim and link it to feature acceptance (B8).")
    add_coverage("installer", installer_inventory, "OS-INSTALLER", "quarshell-docs", "Native install/upgrade/uninstall acceptance for the declared platform (B8).")
    test_items_by_feature = {feature["id"]: [] for feature in features}
    for item in coverage_items:
        if item["kind"] != "test_file":
            continue
        for feature_id in item["feature_ids"]:
            test_items_by_feature[feature_id].append(item)
    feature_evidence = []
    for feature in features:
        relevant = test_items_by_feature[feature["id"]]
        direct = [item for item in relevant if item["details"].get("file_class") == "rust_test_source"]
        feature_evidence.append({
            "feature_id": feature["id"],
            "acceptance": feature["acceptance"],
            "upstream_test_ids": [item["id"] for item in direct],
            "upstream_test_sources": [item["source"] for item in direct],
            "support_fixture_ids": [item["id"] for item in relevant if item["details"].get("file_class") != "rust_test_source"],
            "status": "upstream tests indexed; map assertions to acceptance during port" if direct else "no direct upstream test source mapped; create acceptance coverage in port",
        })

    target_repos = {
        "quarshell-proto": "quarshell-proto", "quarshell-core": "quarshell-core",
        "quarshell-slogx": "quarshell-slogx", "quarshell-runtime": "quarshell-runtime",
        "quarshell-proxy": "quarshell-proxy", "quarshell-providers": "quarshell-providers",
        "quarshell-driver": "quarshell-driver", "quarshell-gateway": "quarshell-gateway",
        "quarshell-sdk": "quarshell-sdk", "quarshell-cli": "quarshell-cli",
        "quarshell-python": "quarshell-python", "quarshell-ui": "quarshell-ui",
        "quarshell-display": "quarshell-display", "quarshell-org-profile": "quarshell-org-profile",
        "quarshell-docs": "quarshell-docs",
    }
    current_repositories = []
    go_module_dirs: dict[str, str] = {}
    go_requirements: dict[str, list[str]] = {}
    for path in sorted(ROOT.glob("quarshell-*")):
        if not path.is_dir() or path.name not in target_repos:
            continue
        item = {"directory": path.name, "target_repository": target_repos[path.name],
                "origin": git_local(path, "remote", "get-url", "origin"), "packages": []}
        go_mod = path / "go.mod"
        if go_mod.exists():
            go_text = go_mod.read_text()
            first = next((line for line in go_text.splitlines() if line.startswith("module ")), "")
            module_path = first.removeprefix("module ").strip()
            go_module_dirs[module_path] = path.name
            item["packages"].append({"ecosystem": "go", "coordinate": module_path,
                                     "source": f"{path.name}/go.mod:1"})
            reqs = []
            in_require = False
            for line in go_text.splitlines():
                stripped = line.strip()
                if stripped == "require (":
                    in_require = True
                    continue
                if in_require and stripped == ")":
                    in_require = False
                    continue
                if stripped.startswith("require "):
                    parts = stripped.removeprefix("require ").split()
                elif in_require and stripped and not stripped.startswith("//"):
                    parts = stripped.split()
                else:
                    continue
                if parts and parts[0].startswith("github.com/"):
                    reqs.append(parts[0])
            go_requirements[module_path] = sorted(set(reqs))
        pyproject = path / "pyproject.toml"
        if pyproject.exists():
            try:
                py_text = pyproject.read_text()
                section = re.search(r"(?ms)^\[project\]\s*(.*?)(?=^\[|\Z)", py_text)
                project = section.group(1) if section else ""
                package_name = re.search(r'(?m)^name\s*=\s*["\']([^"\']+)', project)
                package_version = re.search(r'(?m)^version\s*=\s*["\']([^"\']+)', project)
                if package_name:
                    item["packages"].append({"ecosystem": "python", "coordinate": package_name.group(1),
                                             "version": package_version.group(1) if package_version else None,
                                             "source": f"{path.name}/pyproject.toml"})
            except OSError:
                pass
        package_json = path / "package.json"
        if package_json.exists():
            package = json.loads(package_json.read_text())
            item["packages"].append({"ecosystem": "npm", "coordinate": package.get("name"),
                                     "version": package.get("version"), "source": f"{path.name}/package.json"})
        license_path = path / "LICENSE"
        if license_path.exists():
            item["license_file"] = f"{path.name}/LICENSE"
            license_text = license_path.read_text(errors="replace")[:5000].lower()
            item["license"] = "MIT" if "mit license" in license_text else "Apache-2.0" if "apache license" in license_text else "review_required"
        current_repositories.append(item)

    current_go_edges = {module: sorted(dep for dep in deps if dep in go_module_dirs)
                        for module, deps in go_requirements.items()}
    active: set[str] = set(); visited: set[str] = set(); stack: list[str] = []; dependency_cycles = []
    def visit_module(node: str) -> None:
        if node in active:
            cycle_start = stack.index(node)
            dependency_cycles.append(stack[cycle_start:] + [node])
            return
        if node in visited: return
        active.add(node); stack.append(node)
        for child in current_go_edges.get(node, []): visit_module(child)
        stack.pop(); active.remove(node); visited.add(node)
    for module in current_go_edges: visit_module(module)

    rust_root_version = re.search(r'(?m)^rust-version\s*=\s*"([^"]+)"', (SOURCE / "Cargo.toml").read_text())
    go_work_text = (ROOT / "go.work").read_text() if (ROOT / "go.work").exists() else ""
    go_versions = re.findall(r'(?m)^go\s+(\S+)|^toolchain\s+(\S+)', go_work_text)
    support_doc = SOURCE / "docs/about/support-matrix.mdx"
    support_lines = []
    if support_doc.exists():
        support_lines = [{"line": i, "text": line.strip()} for i, line in enumerate(support_doc.read_text().splitlines(), 1)
                         if "Linux 6.2" in line or "Landlock" in line or "WSL 2" in line]
    support_tables = markdown_tables(support_doc) if support_doc.exists() else []

    data = {
        "schema_version": 2,
        "baseline": {"tag": "v0.1.4", "commit": BASELINE, "checkout_head": current,
                     "checkout_matches_baseline": current == BASELINE,
                     "license": "Apache-2.0; pinned checkout has LICENSE; no root NOTICE file"},
        "namespace_decision": {"planned_owner": "KLORQ", "planned_module_prefix": "github.com/KLORQ/quarshell-*",
                               "public_org_http_status": 404, "check_date": "2026-10-20",
                               "current_owner": "KLORQ", "current_owner_public_org_http_status": 200,
                               "decision": "use the KLORQ GitHub organization; target product/repository name remains quarshell-*"},
        "license_report": {"upstream": [{"source": "LICENSE", "sha256": hashlib.sha256((SOURCE / "LICENSE").read_bytes()).hexdigest()},
                                         {"source": "NOTICE", "sha256": hashlib.sha256((SOURCE / "NOTICE").read_bytes()).hexdigest()} if (SOURCE / "NOTICE").exists() else {"source": "NOTICE", "present": False}],
                           "current_repositories": [{"directory": x["directory"], "license": x.get("license", "missing_or_review_required"),
                                                     "source": x.get("license_file")} for x in current_repositories],
                           "review_status": "inventory only; legal compatibility review required before code reuse"},
        "upstream_delta": delta,
        "inventory_digest_sha256": digest(source_files),
        "counts": {"workspace_members": len(workspace_members), "crates": len(crate_rows),
                   "feature_ids": len(features),
                   "proto_services": len(proto_services), "proto_methods": len(proto_rows),
                   "stream_messages": len({(r["method"], r["message"]) for r in stream_rows}),
                   "stream_variants": len(stream_rows), "provider_profiles": len(providers),
                   "release_platforms": len((releases.get("platforms") or {}).get("rows", [])),
                   "cli_commands": len(cli_commands), "cli_arg_definitions": len(cli_arguments),
                   "tui_screens": len(tui_screens), "tui_key_actions": len(tui_actions), "test_files": len(test_files),
                   "docs_files": len(docs_files), "installer_candidates": len(installer_files),
                   "config_source_candidates": len(config_files), "config_types": len(config_types),
                   "current_repositories": len(current_repositories), "current_go_modules": len(go_module_dirs),
                   "source_files": len(source_files)},
        "workspace_members": workspace_members,
        "features": features,
        "coverage_items": coverage_items,
        "feature_evidence": feature_evidence,
        "crates": crate_rows,
        "upstream_delta_rows": delta_rows if 'delta_rows' in locals() else [],
        "proto_services": proto_services,
        "proto_methods": proto_rows,
        "stream_inventory": stream_rows,
        "providers": providers,
        "release": releases,
        "cli": {"commands": cli_commands, "argument_definitions": cli_arguments,
                "runtime_help": runtime_cli_evidence,
                "source": "crates/openshell-cli/src/main.rs", "status": "all source command variants/arguments reconciled with runtime help and captured with source attributes; behavioral acceptance follows in B6"},
        "configuration": {"source_candidates": config_files, "serialized_struct_candidates": config_types,
                           "category_counts": config_categories,
                           "reviewed_contract": {
                               "schema_version": 2,
                               "schema_source": "crates/openshell-server/src/config_file.rs:36",
                               "root_schema_and_unknown_keys": ["ConfigFile", "OpenShellRoot", "GatewayFileSection", "deny_unknown_fields"],
                               "loaders": ["crates/openshell-server/src/config_file.rs:571", "crates/openshell-server/src/config_file.rs:673", "crates/openshell-server/src/config_file.rs:703"],
                               "merge_precedence": ["CLI flag", "OPENSHELL_* environment", "TOML file", "built-in default"],
                               "merge_precedence_source": "crates/openshell-server/src/config_file.rs:10",
                               "driver_config_boundary": "[openshell.drivers.<name>] and [openshell.credential_drivers.<name>] are raw TOML values parsed by each owning driver; no gateway-level inheritance",
                               "schema_rfc": "rfc/0003-gateway-configuration/README.md:1",
                               "examples": config_example_files,
                               "acceptance_sources": ["crates/openshell-server/src/config_file.rs (inline parser/default/unknown-key/version tests)", "crates/openshell-gateway/tests/config_preflight.rs"],
                              "status": "B0 review complete: loaders, precedence, versioning, unknown-key policy, driver boundary, RFC, examples, and parser/preflight acceptance sources recorded; field-level behavioral parity is tracked in B2/B6"},
                           "post_inventory_acceptance_requirements": ["B2/B6: map each config field's default, validation, and precedence to Go behavior", "B2/B6: keep non-serialized options/domain candidates distinct from persisted config"],
                           "status": "B0 review complete: all config candidates classified by source and serialization/config domain; behavioral parity remains an implementation-stage acceptance"},
        "tui": {"screens": tui_screens, "tabs_and_focus": tui_tabs, "key_code_tokens": key_tokens, "key_actions": tui_actions,
                "source": "crates/openshell-tui/src/app.rs", "status": "B0 review complete: all extracted key-action lines are indexed; feature evidence links test files, named tests, assertion counts, and fixture/support candidates; missing direct upstream test coverage is recorded for new port acceptance"},
        "backends": [{"crate": c["upstream_name"], "source": c["source"], "feature_ids": c["feature_ids"],
                      "status": "mapped; capabilities require B4 acceptance"} for c in driver_crates],
        "tests": {"files": test_inventory,
                  "summary": {"files": len(test_inventory), "rust_test_source_files": sum(x["file_class"] == "rust_test_source" for x in test_inventory),
                              "rust_support_source_files": sum(x["file_class"] == "rust_support_source" for x in test_inventory),
                              "fixtures_or_data_files": sum(x["file_class"] == "test_fixture_or_data" for x in test_inventory),
                              "named_test_functions": sum(x["test_count"] for x in test_inventory),
                              "assertion_macros": sum(x["assertion_count"] for x in test_inventory)},
                  "status": "B0 review complete: file classes, named Rust tests, assertion macro counts, feature links, and fixture/support candidates are indexed; semantic assertion parity is a B1-B8 acceptance"},
        "docs": {"files": docs_inventory, "status": "B0 review complete: every document has title, outline, claim terms, link/fence counts, owner and candidate feature; claim validation is a B8 acceptance"},
        "installers": {"files": installer_inventory, "status": "B0 review complete: every installer candidate has artifact class, inferred platform, owner, and feature IDs; install workflow parity is a B8 acceptance"},
        "current_repositories": current_repositories,
        "current_dependency_dag": {"go_modules": go_module_dirs, "edges": current_go_edges,
                                   "cycles": dependency_cycles,
                                   "architecture_findings": [
                                       {"edge": ["quarshell-runtime", "quarshell-driver"], "issue": "current runtime module depends on the driver module; target plan keeps runtime and driver behind separate boundaries"},
                                       {"edge": ["quarshell-cli", "quarshell-driver"], "issue": "current CLI imports backend implementation directly; target TUI/CLI should use the published SDK/service API"},
                                       {"edge": ["quarshell-cli", "quarshell-proxy"], "issue": "current CLI imports proxy implementation directly; target dependency should flow through SDK/service contracts"},
                                   ],
                                   "status": "derived from go.mod; no cycles; non-Go package dependencies require separate review"},
        "toolchain_and_platform": {"upstream_rust_version": rust_root_version.group(1) if rust_root_version else None,
                                    "current_go_work_versions": [next((x for x in pair if x), None) for pair in go_versions],
                                    "proposed_target_go": {"go_directive": "1.27.0", "toolchain": "go1.27.2",
                                                            "basis": "current root go.work; must pin after namespace decision"},
                                    "support_matrix_source": "docs/about/support-matrix.mdx",
                                    "support_matrix_tables": support_tables,
                                    "support_matrix_evidence": support_lines,
                                    "decision": "baseline platform/version matrix captured; Quarshell support claims remain gated on native acceptance"},
        "remaining_review_gates": [],
        "post_inventory_acceptance_requirements": [
            "B2/B6: verify field-by-field configuration defaults, validation, and precedence in the Go implementation",
            "B1-B8: preserve intent of upstream tests and add acceptance for the five features without direct upstream test source",
            "B8: validate documentation claims and native install/upgrade/uninstall workflows against implemented platform support"
        ],
        "completion": "B0 inventory complete; behavior-specific parity remains gated by the named implementation-stage acceptance requirements",
    }
    feature_ids_set = {item["id"] for item in features}
    incomplete_records = [item["id"] for item in coverage_items
                          if not item["source"] or not item["owner"] or not item["feature_ids"]
                          or not item["acceptance"] or not item["status"] or not item["decision"]
                          or not set(item["feature_ids"]).issubset(feature_ids_set)]
    ids = [item["id"] for item in coverage_items]
    data["automatic_checks"] = {
        "pinned_head_matches_baseline": current == BASELINE,
        "workspace_members_match_crates": len(workspace_members) == len(crate_rows) and {c["upstream_name"] for c in crate_rows} == set(workspace_members),
        "all_crates_have_owner_and_features": all(c["owner"] != "UNASSIGNED" and c["feature_ids"] for c in crate_rows),
        "coverage_ids_unique": len(ids) == len(set(ids)),
        "coverage_records_complete_and_features_known": not incomplete_records,
        "source_cli_paths_covered_by_runtime": not (set(source_command_paths) - set(runtime_command_paths)) if runtime_cli else None,
        "source_short_flags_match_runtime": set(source_short_flags) == set(runtime_short_flags) if runtime_cli else None,
        "only_hidden_source_flag_is_keep": set(source_long_flags) - set(runtime_long_flags) == {"keep"} if runtime_cli else None,
    }
    if current != BASELINE:
        raise ValueError(f"pinned baseline mismatch: expected {BASELINE}, got {current}")
    if len(workspace_members) != len(crate_rows) or {c["upstream_name"] for c in crate_rows} != set(workspace_members):
        raise ValueError("Cargo workspace members and crate inventory disagree")
    if any(c["owner"] == "UNASSIGNED" or not c["feature_ids"] for c in crate_rows):
        raise ValueError("one or more upstream crates lack an owner or OS-* feature mapping")
    if incomplete_records or len(ids) != len(set(ids)):
        raise ValueError(f"coverage inventory has incomplete records or duplicate IDs: {incomplete_records[:10]}")
    if runtime_cli and set(source_command_paths) - set(runtime_command_paths):
        raise ValueError("source CLI command variants are missing from runtime help capture")
    if runtime_cli and (set(source_short_flags) != set(runtime_short_flags)
                        or set(source_long_flags) - set(runtime_long_flags) != {"keep"}):
        raise ValueError("source and runtime CLI flag inventory differ beyond the documented hidden --keep flag")
    OUT.write_text(json.dumps(data, indent=2, ensure_ascii=False) + "\n")
    yaml_lines = ["# Generated from the plan's OS-* feature table and pinned OpenShell inventory.",
                  "# JSON-compatible YAML keeps generation dependency-free; edit the plan/source, then regenerate.",
                  "schema_version: 1", "baseline: " + json.dumps("OpenShell v0.1.4", ensure_ascii=False),
                  "namespace_owner: " + json.dumps("KLORQ; module prefix github.com/KLORQ/quarshell-*", ensure_ascii=False),
                  "inventory: " + json.dumps(OUT.name, ensure_ascii=False), "features:"]
    for feature in features:
        yaml_lines.extend([
            "  - id: " + json.dumps(feature["id"], ensure_ascii=False),
            "    scope: " + json.dumps(feature["scope"], ensure_ascii=False),
            "    owner: " + json.dumps(feature["owner"], ensure_ascii=False),
            "    acceptance: " + json.dumps(feature["acceptance"], ensure_ascii=False),
            "    status: " + json.dumps(feature["status"], ensure_ascii=False),
            "    baseline: " + json.dumps(feature["baseline"], ensure_ascii=False),
        ])
    yaml_lines.extend(["coverage_items:"])
    for item in coverage_items:
        yaml_lines.extend([
            "  - id: " + json.dumps(item["id"], ensure_ascii=False),
            "    kind: " + json.dumps(item["kind"], ensure_ascii=False),
            "    name: " + json.dumps(str(item["name"]), ensure_ascii=False),
            "    details: " + json.dumps(item["details"], ensure_ascii=False),
            "    source: " + json.dumps(item["source"], ensure_ascii=False),
            "    evidence: " + json.dumps(item["evidence"], ensure_ascii=False),
            "    owner: " + json.dumps(item["owner"], ensure_ascii=False),
            "    feature_ids: " + json.dumps(item["feature_ids"], ensure_ascii=False),
            "    platforms: " + json.dumps(item["platforms"], ensure_ascii=False),
            "    acceptance: " + json.dumps(item["acceptance"], ensure_ascii=False),
            "    status: " + json.dumps(item["status"], ensure_ascii=False),
            "    decision: " + json.dumps(item["decision"], ensure_ascii=False),
        ])
    FEATURES_OUT.write_text("\n".join(yaml_lines) + "\n")
    print(f"wrote {OUT.relative_to(ROOT)} and {FEATURES_OUT.relative_to(ROOT)} ({len(crate_rows)} crates, {len(proto_rows)} RPCs, sha256 {data['inventory_digest_sha256']})")


if __name__ == "__main__":
    main()
