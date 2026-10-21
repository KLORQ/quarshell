#!/usr/bin/env python3
"""Create Go-generatable OpenShell v0.1.4 schemas from the immutable source lock."""
from __future__ import annotations
import hashlib
import json
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SOURCE = ROOT / "upstream/openshell/v0.1.4/proto"
OUTPUT = ROOT / "proto"
LOCK = json.loads((SOURCE / "SOURCE.lock.json").read_text())

FILES = {
    "compute_driver.proto": ("openshell/compute/v1/compute_driver.proto", "computev1"),
    "credential_driver.proto": ("openshell/credentials/v1/credential_driver.proto", "credentialsv1"),
    "datamodel.proto": ("openshell/datamodel/v1/datamodel.proto", "datamodelv1"),
    "extension.proto": ("openshell/extension/v1/extension.proto", "extensionv1"),
    "gateway_interceptor.proto": ("openshell/gateway_interceptor/v1/gateway_interceptor.proto", "gatewayinterceptorv1"),
    "openshell.proto": ("openshell/gateway/v1/openshell.proto", "gatewayv1"),
    "options.proto": ("openshell/options/v1/options.proto", "optionsv1"),
    "pagination.proto": ("openshell/pagination/v1/pagination.proto", "paginationv1"),
    "sandbox.proto": ("openshell/sandbox/v1/sandbox.proto", "sandboxv1"),
    "supervisor_middleware.proto": ("openshell/middleware/v1/supervisor_middleware.proto", "middlewarev1"),
    "test.proto": ("openshell/test/v1/test.proto", "testv1"),
    "server/storage.proto": ("openshell/storage/v1/storage.proto", "storagev1"),
    "sandbox-backend/openshell_sandbox.proto": ("openshell/sandbox_protocol/v1/openshell_sandbox.proto", "sandboxprotocolv1"),
}
SOURCE_TO_TARGET = {source: target for source, (target, _) in FILES.items()}
GO_IMPORT = {source: f"github.com/KLORQ/quarshell-proto/gen/go/{Path(target).parent.as_posix()}" for source, (target, _) in FILES.items()}

expected = {row["path"]: row["sha256"] for row in LOCK["files"]}
actual = {p.relative_to(SOURCE).as_posix() for p in SOURCE.rglob("*.proto")}
if actual != set(expected):
    raise SystemExit(f"pinned source file set mismatch: missing={sorted(set(expected)-actual)} extra={sorted(actual-set(expected))}")

for source, (target, go_package) in FILES.items():
    src = SOURCE / source
    if hashlib.sha256(src.read_bytes()).hexdigest() != expected[source]:
        raise SystemExit(f"pinned source digest mismatch: {source}")
    text = src.read_text()
    for imported, destination in SOURCE_TO_TARGET.items():
        basename = Path(imported).name
        if re.search(rf'^import\s+"{re.escape(basename)}";', text, re.M):
            text = text.replace(f'import "{basename}";', f'import "{destination}";')
    go_path = GO_IMPORT[source]
    option = f'option go_package = "{go_path};{go_package}";'
    if "option go_package" not in text:
        imports = list(re.finditer(r'^import\s+"[^\"]+";\s*$', text, re.M))
        if imports:
            pos = imports[-1].end()
            text = text[:pos] + "\n" + option + text[pos:]
        else:
            package = re.search(r'^package\s+[^;]+;', text, re.M)
            if not package:
                raise SystemExit(f"missing package declaration: {source}")
            pos = package.end()
            text = text[:pos] + "\n\n" + option + text[pos:]
    destination = OUTPUT / target
    destination.parent.mkdir(parents=True, exist_ok=True)
    destination.write_text(text)
print(f"materialized {len(FILES)} OpenShell v0.1.4 protobuf schemas")
