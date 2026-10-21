"""gRPC client for quarshell-gateway and the pinned OpenShell contract."""

from __future__ import annotations

import json
import os
import uuid
from dataclasses import dataclass, field
from datetime import datetime, timezone
from typing import (
    BinaryIO,
    Callable,
    Dict,
    Generic,
    Hashable,
    Iterable,
    Iterator,
    List,
    Optional,
    TypeVar,
)
from urllib.parse import quote, urlparse

import grpc
import httpx
import yaml
from google.protobuf.json_format import ParseDict
from google.protobuf import struct_pb2
from google.protobuf.json_format import MessageToDict
from google.protobuf.timestamp_pb2 import Timestamp

from ._proto.openshell import (
    datamodel_pb2,
    openshell_pb2,
    openshell_pb2_grpc,
    sandbox_pb2,
)


class ConnectUnsupported(RuntimeError):
    """Interactive SSH terminal management stays on the CLI."""


T = TypeVar("T")


@dataclass
class CursorPage(Generic[T]):
    """One page returned by an OpenShell cursor-based list RPC."""

    items: List[T] = field(default_factory=list)
    next_page_token: str = ""


@dataclass
class ServiceExposure:
    """Named browser service registered with a Gateway-managed sandbox."""

    name: str
    target_port: int
    authorization_mode: str = "strip"


@dataclass
class Sandbox:
    name: str
    status: str = ""
    id: str = ""
    image: str = ""
    workspace: str = ""
    resource_version: int = 0
    labels: Dict[str, str] = field(default_factory=dict)
    command: List[str] = field(default_factory=list)
    network: str = ""
    base_policy_yaml: str = ""
    attached_providers: List[str] = field(default_factory=list)
    compute_driver: str = ""
    updated_at_unix_ms: int = 0
    runtime_status: str = ""
    runtime_status_available: bool = False
    workload_template_name: str = ""
    cpu_millis: int = 0
    memory_bytes: int = 0
    gpu_count: int = 0
    pids_limit: int = 0
    cdi_devices: List[str] = field(default_factory=list)
    tty: bool = False
    environment: Dict[str, str] = field(default_factory=dict)
    driver_config_json: str = ""
    service_exposures: List[ServiceExposure] = field(default_factory=list)


@dataclass
class WorkspaceMember:
    subject: str
    role: str


@dataclass
class Workspace:
    name: str
    id: str = ""
    status: str = ""
    resource_version: int = 0
    labels: Dict[str, str] = field(default_factory=dict)
    created_at_ms: int = 0
    members: List[WorkspaceMember] = field(default_factory=list)


@dataclass
class ServiceEndpoint:
    name: str
    sandbox: str
    port: int
    url: str = ""
    domain: bool = False
    authorization_mode: str = ""



@dataclass(frozen=True)
class SSHSession:
    """Short-lived SSH authorization. The token is shown only at creation."""

    sandbox_id: str
    token: str = field(repr=False)
    gateway_host: str = ""
    gateway_port: int = 0
    gateway_scheme: str = ""
    host_key_fingerprint: str = ""
    expires_at_ms: int = 0


@dataclass
class ExecResult:
    exit_code: int
    output: str


@dataclass(frozen=True)
class ExecInput:
    """One stdin chunk or terminal resize for an interactive execution."""

    stdin: Optional[bytes] = None
    cols: Optional[int] = None
    rows: Optional[int] = None

    def __post_init__(self) -> None:
        is_stdin = self.stdin is not None
        is_resize = self.cols is not None or self.rows is not None
        if is_stdin == is_resize:
            raise ValueError("provide either stdin bytes or both cols and rows")
        if is_resize and (self.cols is None or self.rows is None):
            raise ValueError("terminal resize requires both cols and rows")
        if is_resize and (self.cols <= 0 or self.rows <= 0):
            raise ValueError("terminal dimensions must be positive")

    @classmethod
    def send(cls, data: bytes) -> "ExecInput":
        return cls(stdin=data)

    @classmethod
    def resize(cls, cols: int, rows: int) -> "ExecInput":
        return cls(cols=cols, rows=rows)


@dataclass(frozen=True)
class ExecEvent:
    """A streamed stdout, stderr, or process exit event."""

    stdout: bytes = b""
    stderr: bytes = b""
    exit_code: Optional[int] = None


@dataclass
class SandboxLogLine:
    timestamp_unix_ms: int
    source: str
    level: str
    message: str
    target: str = ""
    cursor: int = 0


@dataclass
class SandboxLogPage:
    lines: List[SandboxLogLine] = field(default_factory=list)
    cursor: int = 0


@dataclass
class WorkloadTemplate:
    """Workspace-scoped reusable sandbox workload configuration."""

    name: str
    image: str
    workspace: str = ""
    environment: Dict[str, str] = field(default_factory=dict)
    cpu: str = ""
    memory: str = ""
    gpu: bool = False
    gpu_count: int = 0
    labels: Dict[str, str] = field(default_factory=dict)
    driver_config_json: str = ""


@dataclass
class ProviderRefresh:
    """Refresh metadata for one provider credential; material is sensitive."""

    strategy: str
    material: Dict[str, str] = field(default_factory=dict, repr=False)
    expires_at_ms: int = 0


@dataclass
class Provider:
    """Workspace provider metadata. Credential values are write-only."""

    name: str
    type: str
    workspace: str = ""
    env_vars: List[str] = field(default_factory=list)
    credentials: Dict[str, str] = field(default_factory=dict, repr=False)
    credential_expires_at_ms: Dict[str, int] = field(default_factory=dict)
    runtime_credentials: bool = False
    config: Dict[str, str] = field(default_factory=dict)
    refresh: Dict[str, ProviderRefresh] = field(default_factory=dict, repr=False)


@dataclass
class PolicyRevision:
    rev: int
    updated_at: datetime
    bytes: int
    status: str


@dataclass
class PolicyRevisionRecord:
    """Policy revision metadata returned by the pinned OpenShell Gateway API."""

    rev: int
    policy_hash: str
    status: str
    load_error: str
    created_at: Optional[datetime]
    loaded_at: Optional[datetime]
    provenance: Dict[str, str] = field(default_factory=dict)


@dataclass
class ProfileInfo:
    id: str
    category: str
    source: str
    scope: str
    resource_version: int


@dataclass
class ProfileDocument:
    yaml: bytes
    source: str
    resource_version: str


class Client:
    """Uses native gRPC for resources and OpenShell execution RPCs."""

    def __init__(
        self,
        base_url: str,
        timeout: float = 70.0,
        token: Optional[str] = None,
        workspace: str = "default",
    ) -> None:
        self.base_url = base_url.rstrip("/")
        self.timeout = timeout
        self.workspace = workspace or "default"
        bearer = (
            token if token is not None else os.getenv("CAUTEM_GATEWAY_TOKEN", "")
        ).strip()
        self._metadata = (("authorization", f"Bearer {bearer}"),) if bearer else ()
        self._http = httpx.Client(base_url=self.base_url, timeout=timeout)
        self._channel: Optional[grpc.Channel] = None

    def close(self) -> None:
        self._http.close()
        if self._channel is not None:
            self._channel.close()
            self._channel = None

    def __enter__(self) -> "Client":
        return self

    def __exit__(self, *args: object) -> None:
        self.close()

    def _rpc(self) -> grpc.Channel:
        if self._channel is not None:
            return self._channel
        endpoint = urlparse(self.base_url)
        if (
            not endpoint.hostname
            or endpoint.path
            or endpoint.query
            or endpoint.fragment
        ):
            raise ValueError("gateway URL must contain only scheme and authority")
        target = endpoint.netloc
        if endpoint.scheme == "https":
            self._channel = grpc.secure_channel(target, grpc.ssl_channel_credentials())
        elif endpoint.scheme == "http":
            host = endpoint.hostname.lower()
            if host != "localhost" and not host.endswith(".localhost"):
                import ipaddress

                if not ipaddress.ip_address(host).is_loopback:
                    raise ValueError(
                        "unencrypted gRPC is allowed only for loopback gateways"
                    )
            self._channel = grpc.insecure_channel(target)
        else:
            raise ValueError(f"unsupported gateway URL scheme: {endpoint.scheme!r}")
        return self._channel

    def _call(self, method, request):
        return method(request, timeout=self.timeout, metadata=self._metadata)

    def healthz(self) -> Dict[str, object]:
        """Check liveness over the transport-specific HTTP health endpoint."""
        response = self._http.get("/healthz")
        response.raise_for_status()
        return response.json()

    def info(self) -> Dict[str, object]:
        """Return native OpenShell Gateway status and advertised drivers."""
        response = self._call(
            openshell_pb2_grpc.OpenShellStub(self._rpc()).GetGatewayInfo,
            openshell_pb2.GetGatewayInfoRequest(),
        )
        return {
            "status": openshell_pb2.ServiceStatus.Name(response.status),
            "gateway_version": response.gateway_version,
            "compute_drivers": [
                {
                    "name": item.name,
                    "version": item.capabilities.driver_version,
                    "capabilities": MessageToDict(
                        item.capabilities, preserving_proto_field_name=True
                    ),
                }
                for item in response.compute_drivers
            ],
            "extensions": [MessageToDict(item, preserving_proto_field_name=True) for item in response.extensions],
        }

    def whoami(self) -> Dict[str, object]:
        """Return identity metadata from native OpenShell Gateway RPCs."""
        viewer = self._call(
            openshell_pb2_grpc.OpenShellStub(self._rpc()).GetCurrentUser,
            openshell_pb2.GetCurrentUserRequest(),
        )
        return {
            "auth": "authenticated",
            "subject": viewer.subject,
            "roles": list(viewer.roles),
            "scopes": list(viewer.scopes),
            "identity_provider": viewer.identity_provider,
        }

    def create(self, sb: Sandbox) -> Sandbox:
        """Create a sandbox through the pinned OpenShell Gateway API."""
        _require_nonempty(sb.name, "sandbox name")
        _require_nonempty(sb.image or sb.workload_template_name, "sandbox image or workload template")
        if not 0 <= sb.cpu_millis < (1 << 32):
            raise ValueError("sandbox cpu_millis must fit uint32")
        if not 0 <= sb.memory_bytes < (1 << 64):
            raise ValueError("sandbox memory_bytes must fit uint64")
        if not 0 <= sb.gpu_count < (1 << 32):
            raise ValueError("sandbox gpu_count must fit uint32")
        if sb.pids_limit:
            raise ValueError("pids_limit is Gateway driver configuration, not an OpenShell sandbox field")
        if sb.base_policy_yaml:
            raise ValueError("base_policy_yaml is not an OpenShell create field; use the policy API")
        if sb.cdi_devices:
            raise ValueError("CDI device selection must be expressed in driver_config_json")
        if sb.workload_template_name and (sb.image or sb.cpu_millis or sb.memory_bytes or sb.gpu_count or sb.driver_config_json or sb.environment):
            raise ValueError("workload template cannot be combined with inline workload options")
        if not 0 <= sb.gpu_count < (1 << 32):
            raise ValueError("sandbox gpu_count must fit uint32")
        template = None
        if not sb.workload_template_name:
            template = openshell_pb2.SandboxTemplate(image=sb.image)
            if sb.driver_config_json:
                try:
                    ParseDict(json.loads(sb.driver_config_json), template.driver_config)
                except (json.JSONDecodeError, TypeError, ValueError) as exc:
                    raise ValueError(f"sandbox driver_config_json: {exc}") from exc
            limits = {}
            if sb.cpu_millis:
                limits["cpu"] = f"{sb.cpu_millis}m"
            if sb.memory_bytes:
                limits["memory"] = str(sb.memory_bytes)
            if limits:
                ParseDict({"limits": limits}, template.resources)
        spec = openshell_pb2.SandboxSpec(
            command=sb.command, tty=sb.tty, environment=sb.environment,
        )
        if template is not None:
            spec.template.CopyFrom(template)
        if sb.gpu_count:
            spec.resource_requirements.gpu.count = sb.gpu_count
        exposures = []
        seen_names = set()
        if len(sb.service_exposures) > 32:
            raise ValueError("at most 32 service exposures are allowed")
        for exposure in sb.service_exposures:
            _require_nonempty(exposure.name, "service exposure name")
            if not 1 <= exposure.target_port <= 65535:
                raise ValueError("service exposure target_port must be between 1 and 65535")
            if exposure.name in seen_names:
                raise ValueError(f"duplicate service exposure name {exposure.name!r}")
            seen_names.add(exposure.name)
            exposures.append(openshell_pb2.SandboxServiceExposure(
                service=exposure.name, target_port=exposure.target_port,
                authorization_mode=_service_authorization_mode(exposure.authorization_mode),
            ))
        request_id = uuid.uuid4().hex
        request = openshell_pb2.CreateSandboxRequest(
            workspace_scope=_workspace_scope(sb.workspace or self.workspace),
            name=sb.name, labels=sb.labels, spec=spec,
            workload_template=sb.workload_template_name,
            request_id=request_id, service_exposures=exposures,
        )
        response = self._call(openshell_pb2_grpc.OpenShellStub(self._rpc()).CreateSandbox, request)
        return _sandbox_from_native(response.sandbox)

    def list(self) -> List[Sandbox]:
        """Collect native OpenShell sandbox pages in the selected workspace."""
        token = ""
        seen_tokens = set()
        seen_sandboxes = set()
        items: List[Sandbox] = []
        while True:
            response = self._call(
                openshell_pb2_grpc.OpenShellStub(self._rpc()).ListSandboxes,
                openshell_pb2.ListSandboxesRequest(
                    workspace_scope=_workspace_scope(self.workspace),
                    page_size=1000, page_token=token,
                ),
            )
            _append_unique_page_items(
                items, seen_sandboxes,
                (_sandbox_from_native(item) for item in response.sandboxes),
                key=lambda item: (item.workspace, item.id or item.name),
                description="sandbox",
            )
            token = response.next_page_token
            if not token:
                return items
            if token in seen_tokens:
                raise RuntimeError("list sandboxes: gateway repeated a pagination token")
            seen_tokens.add(token)

    def list_page(self, *, page_size: int = 100, page_token: str = "") -> CursorPage[Sandbox]:
        """List one page of native OpenShell sandboxes."""
        _validate_page_size(page_size, "sandbox")
        response = self._call(
            openshell_pb2_grpc.OpenShellStub(self._rpc()).ListSandboxes,
            openshell_pb2.ListSandboxesRequest(
                workspace_scope=_workspace_scope(self.workspace),
                page_size=min(page_size, 1000), page_token=page_token,
            ),
        )
        return CursorPage(
            items=[_sandbox_from_native(item) for item in response.sandboxes],
            next_page_token=response.next_page_token,
        )

    def get(self, name: str) -> Sandbox:
        response = self._call(
            openshell_pb2_grpc.OpenShellStub(self._rpc()).GetSandbox,
            openshell_pb2.GetSandboxRequest(
                workspace_scope=_workspace_scope(self.workspace), name=name,
            ),
        )
        return _sandbox_from_native(response.sandbox)

    def watch_sandbox(
        self,
        name: str,
        *,
        follow_status: bool = False,
        follow_logs: bool = True,
        follow_events: bool = False,
        log_tail_lines: int = 100,
        event_tail: int = 0,
        since_unix_ms: int = 0,
        sources: Iterable[str] = (),
        min_level: str = "",
        resume_after_cursor: str = "",
    ) -> Iterator[openshell_pb2.SandboxStreamEvent]:
        """Stream native OpenShell sandbox status, logs, or platform events."""
        _require_nonempty(name, "sandbox name")
        if log_tail_lines < 0 or event_tail < 0 or since_unix_ms < 0:
            raise ValueError("watch tails and timestamp must not be negative")
        if not any((follow_status, follow_logs, follow_events)):
            raise ValueError("at least one sandbox watch stream must be enabled")
        request = openshell_pb2.WatchSandboxRequest(
            workspace_scope=_workspace_scope(self.workspace), sandbox=name,
            follow_status=follow_status, follow_logs=follow_logs,
            follow_events=follow_events, log_tail_lines=min(log_tail_lines, 1000),
            event_tail=min(event_tail, 1000), log_sources=list(sources),
            log_min_level=min_level, resume_after_cursor=resume_after_cursor,
        )
        if since_unix_ms:
            request.since_time.FromMilliseconds(since_unix_ms)
        stream = openshell_pb2_grpc.OpenShellStub(self._rpc()).WatchSandbox(
            request, timeout=self.timeout, metadata=self._metadata,
        )
        try:
            yield from stream
        finally:
            stream.cancel()

    def watch_sandbox_logs(
        self, name: str, *, initial_limit: int = 100, since_unix_ms: int = 0,
        source: str = "", level: str = "", resume_after_cursor: str = "",
    ) -> Iterator[openshell_pb2.SandboxStreamEvent]:
        """Stream native OpenShell log events for one sandbox."""
        return self.watch_sandbox(
            name, follow_logs=True, follow_status=False, follow_events=False,
            log_tail_lines=initial_limit, since_unix_ms=since_unix_ms,
            sources=[source] if source else [], min_level=level,
            resume_after_cursor=resume_after_cursor,
        )

    def get_sandbox_logs(
        self,
        name: str,
        *,
        limit: int = 100,
        since_unix_ms: int = 0,
        source: str = "",
        level: str = "",
    ) -> SandboxLogPage:
        """Read a bounded log tail through the native OpenShell Gateway API."""
        _require_nonempty(name, "sandbox name")
        if limit < 0 or since_unix_ms < 0:
            raise ValueError("log limit and timestamp must not be negative")
        request = openshell_pb2.GetSandboxLogsRequest(
            workspace_scope=_workspace_scope(self.workspace), sandbox=name,
            lines=min(limit, 1000), sources=[source] if source else [], min_level=level,
        )
        if since_unix_ms:
            request.since_time.FromMilliseconds(since_unix_ms)
        response = self._call(
            openshell_pb2_grpc.OpenShellStub(self._rpc()).GetSandboxLogs, request,
        )
        return SandboxLogPage(
            lines=[_sandbox_log_line(item) for item in response.logs],
            cursor=response.buffer_total,
        )

    def append_sandbox_logs(self, name: str, lines: Iterable[SandboxLogLine]) -> None:
        """Push client-collected logs through the native OpenShell stream RPC."""
        _require_nonempty(name, "sandbox name")
        sandbox = self.get(name)
        request_lines = []
        for line in lines:
            if not isinstance(line, SandboxLogLine):
                raise TypeError("lines must contain SandboxLogLine values")
            event_time = Timestamp()
            event_time.FromMilliseconds(line.timestamp_unix_ms)
            request_lines.append(openshell_pb2.SandboxLogLine(
                sandbox_id=sandbox.id, event_time=event_time, source=line.source,
                level=line.level, message=line.message, target=line.target,
            ))
        if not request_lines:
            return None
        stub = openshell_pb2_grpc.OpenShellStub(self._rpc())
        stub.PushSandboxLogs(
            iter([openshell_pb2.PushSandboxLogsRequest(
                sandbox_id=sandbox.id, logs=request_lines,
            )]), timeout=self.timeout, metadata=self._metadata,
        )
        return None

    def delete(self, name: str) -> None:
        """Delete a sandbox through native OpenShell lifecycle RPCs."""
        _require_nonempty(name, "sandbox name")
        self._call(
            openshell_pb2_grpc.OpenShellStub(self._rpc()).DeleteSandbox,
            openshell_pb2.DeleteSandboxRequest(
                workspace_scope=_workspace_scope(self.workspace),
                name=name,
                request_id=uuid.uuid4().hex,
            ),
        )

    def start(self, name: str) -> Sandbox:
        """Start a stopped sandbox through native OpenShell lifecycle RPCs."""
        _require_nonempty(name, "sandbox name")
        response = self._call(
            openshell_pb2_grpc.OpenShellStub(self._rpc()).StartSandbox,
            openshell_pb2.StartSandboxRequest(
                workspace_scope=_workspace_scope(self.workspace),
                name=name,
                request_id=uuid.uuid4().hex,
            ),
        )
        return _sandbox_from_native(response.sandbox)

    def stop(self, name: str) -> Sandbox:
        """Stop a sandbox through native OpenShell lifecycle RPCs."""
        _require_nonempty(name, "sandbox name")
        response = self._call(
            openshell_pb2_grpc.OpenShellStub(self._rpc()).StopSandbox,
            openshell_pb2.StopSandboxRequest(
                workspace_scope=_workspace_scope(self.workspace),
                name=name,
                request_id=uuid.uuid4().hex,
            ),
        )
        return _sandbox_from_native(response.sandbox)

    def exec(self, name: str, *argv: str) -> ExecResult:
        if not name or not argv:
            raise ValueError("usage: exec(name, *argv)")
        stub = openshell_pb2_grpc.OpenShellStub(self._rpc())
        sandbox = self._call(
            stub.GetSandbox,
            openshell_pb2.GetSandboxRequest(
                name=name, workspace_scope=_workspace_scope(self.workspace)
            ),
        ).sandbox
        stream = stub.ExecSandbox(
            openshell_pb2.ExecSandboxRequest(
                sandbox=sandbox.metadata.id,
                workspace_scope=_workspace_scope(self.workspace),
                command=list(argv),
            ),
            timeout=self.timeout,
            metadata=self._metadata,
        )
        output: List[bytes] = []
        exit_code = -1
        for event in stream:
            payload = event.WhichOneof("payload")
            if payload == "stdout":
                output.append(event.stdout.data)
            elif payload == "stderr":
                output.append(event.stderr.data)
            elif payload == "exit":
                exit_code = event.exit.exit_code
        return ExecResult(
            exit_code=exit_code, output=b"".join(output).decode(errors="replace")
        )

    def exec_interactive(
        self,
        name: str,
        *argv: str,
        inputs: Iterable[ExecInput] = (),
        tty: bool = False,
        cols: int = 80,
        rows: int = 24,
        workdir: str = "",
        environment: Optional[Dict[str, str]] = None,
        no_login_shell: bool = False,
    ) -> Iterator[ExecEvent]:
        """Run a process with streaming stdin, output, and optional TTY resize.

        The first request starts the process. Subsequent ``inputs`` may send
        stdin chunks or resize the terminal. Output is yielded as received.
        Closing the iterator cancels the gRPC stream and its remote process.
        """
        if not name or not argv:
            raise ValueError("usage: exec_interactive(name, *argv)")
        if cols <= 0 or rows <= 0:
            raise ValueError("terminal dimensions must be positive")
        stub = openshell_pb2_grpc.OpenShellStub(self._rpc())
        sandbox = self._call(
            stub.GetSandbox,
            openshell_pb2.GetSandboxRequest(
                name=name, workspace_scope=_workspace_scope(self.workspace)
            ),
        ).sandbox
        start = openshell_pb2.ExecSandboxRequest(
            sandbox=sandbox.metadata.id,
            workspace_scope=_workspace_scope(self.workspace),
            command=list(argv),
            workdir=workdir,
            environment=environment or {},
            tty=tty,
            cols=cols,
            rows=rows,
            no_login_shell=no_login_shell,
        )

        def request_stream():
            yield openshell_pb2.ExecSandboxInput(start=start)
            for item in inputs:
                if not isinstance(item, ExecInput):
                    raise TypeError("interactive inputs must be ExecInput values")
                if item.stdin is not None:
                    yield openshell_pb2.ExecSandboxInput(stdin=item.stdin)
                else:
                    yield openshell_pb2.ExecSandboxInput(
                        resize=openshell_pb2.ExecSandboxWindowResize(
                            cols=item.cols, rows=item.rows
                        )
                    )

        stream = stub.ExecSandboxInteractive(
            request_stream(), timeout=self.timeout, metadata=self._metadata
        )
        try:
            for event in stream:
                payload = event.WhichOneof("payload")
                if payload == "stdout":
                    yield ExecEvent(stdout=event.stdout.data)
                elif payload == "stderr":
                    yield ExecEvent(stderr=event.stderr.data)
                elif payload == "exit":
                    yield ExecEvent(exit_code=event.exit.exit_code)
        finally:
            stream.cancel()

    def create_ssh_session(self, sandbox: str) -> SSHSession:
        """Issue a short-lived SSH token for a sandbox in this workspace."""
        _require_nonempty(sandbox, "sandbox name")
        stub = openshell_pb2_grpc.OpenShellStub(self._rpc())
        response = self._call(
            stub.CreateSshSession,
            openshell_pb2.CreateSshSessionRequest(
                sandbox=sandbox,
                workspace_scope=_workspace_scope(self.workspace),
            ),
        )
        expires_at_ms = 0
        if response.HasField("expiration_time"):
            expires_at_ms = int(
                response.expiration_time.seconds * 1000
                + response.expiration_time.nanos // 1_000_000
            )
        return SSHSession(
            sandbox_id=response.sandbox_id,
            token=response.token,
            gateway_host=response.gateway_host,
            gateway_port=response.gateway_port,
            gateway_scheme=response.gateway_scheme,
            host_key_fingerprint=response.host_key_fingerprint,
            expires_at_ms=expires_at_ms,
        )

    def revoke_ssh_session(self, token: str, *, allow_missing: bool = False) -> None:
        """Revoke an SSH session by its secret token without exposing it in errors."""
        _require_nonempty(token, "SSH session token")
        stub = openshell_pb2_grpc.OpenShellStub(self._rpc())
        self._call(
            stub.RevokeSshSession,
            openshell_pb2.RevokeSshSessionRequest(
                token=token,
                allow_missing=allow_missing,
            ),
        )

    def create_template(self, template: WorkloadTemplate) -> WorkloadTemplate:
        """Create a shared workload template in the selected workspace."""
        if not template.name or not template.image:
            raise ValueError("template name and image are required")
        if template.gpu_count < 0 or template.gpu_count > 1024:
            raise ValueError("template GPU count must be between 0 and 1024")
        workspace = template.workspace or self.workspace
        resources = openshell_pb2.SandboxResources(
            cpu=template.cpu, memory=template.memory
        )
        if template.gpu:
            gpu = openshell_pb2.GpuResourceRequirements()
            if template.gpu_count:
                gpu.count = template.gpu_count
            resources.gpu.CopyFrom(gpu)
        driver_config = _driver_config(template.driver_config_json)
        request = openshell_pb2.CreateSandboxTemplateRequest(
            workspace_scope=_workspace_scope(workspace),
            request_id=uuid.uuid4().hex,
            template=openshell_pb2.SandboxWorkloadTemplate(
                metadata=datamodel_pb2.ObjectMeta(
                    name=template.name,
                    workspace=workspace,
                    labels=template.labels,
                ),
                spec=openshell_pb2.SandboxWorkloadTemplateSpec(
                    workload=openshell_pb2.SandboxWorkloadConfig(
                        image=template.image,
                        environment=template.environment,
                        resources=resources,
                    ),
                    driver_config=driver_config,
                ),
            ),
        )
        response = self._call(
            openshell_pb2_grpc.OpenShellStub(self._rpc()).CreateSandboxTemplate,
            request,
        )
        return _workload_template_from_proto(response.template)

    def get_template(
        self, name: str, workspace: Optional[str] = None
    ) -> WorkloadTemplate:
        """Get a shared workload template from one workspace."""
        response = self._call(
            openshell_pb2_grpc.OpenShellStub(self._rpc()).GetSandboxTemplate,
            openshell_pb2.GetSandboxTemplateRequest(
                workspace_scope=_workspace_scope(workspace or self.workspace), name=name
            ),
        )
        return _workload_template_from_proto(response.template)

    def list_templates_page(
        self,
        workspace: Optional[str] = None,
        *,
        page_size: int = 100,
        page_token: str = "",
        label_selector: str = "",
    ) -> CursorPage[WorkloadTemplate]:
        """List one workspace-scoped page of workload templates."""
        _validate_page_size(page_size, "template")
        page_size = min(page_size, 1000)
        response = self._call(
            openshell_pb2_grpc.OpenShellStub(self._rpc()).ListSandboxTemplates,
            openshell_pb2.ListSandboxTemplatesRequest(
                workspace_scope=_workspace_scope(workspace or self.workspace),
                page_size=page_size,
                page_token=page_token,
                label_selector=label_selector,
            ),
        )
        return CursorPage(
            items=[_workload_template_from_proto(item) for item in response.templates],
            next_page_token=response.next_page_token,
        )

    def list_templates(self, workspace: Optional[str] = None) -> List[WorkloadTemplate]:
        """List all templates in one workspace, following cursor pagination."""
        stub = openshell_pb2_grpc.OpenShellStub(self._rpc())
        token = ""
        seen_tokens = set()
        seen_templates = set()
        templates: List[WorkloadTemplate] = []
        while True:
            response = self._call(
                stub.ListSandboxTemplates,
                openshell_pb2.ListSandboxTemplatesRequest(
                    workspace_scope=_workspace_scope(workspace or self.workspace),
                    page_size=1000,
                    page_token=token,
                ),
            )
            if response.next_page_token and response.next_page_token in seen_tokens:
                raise RuntimeError("list templates: gateway repeated a pagination token")
            _append_unique_page_items(
                templates,
                seen_templates,
                (_workload_template_from_proto(item) for item in response.templates),
                key=lambda item: (item.workspace, item.name),
                description="template",
            )
            token = response.next_page_token
            if not token:
                return templates
            if token in seen_tokens:
                raise RuntimeError(
                    "list templates: gateway repeated a pagination token"
                )
            seen_tokens.add(token)

    def delete_template(self, name: str, workspace: Optional[str] = None) -> bool:
        """Delete a template; return whether this call completed the deletion."""
        response = self._call(
            openshell_pb2_grpc.OpenShellStub(self._rpc()).DeleteSandboxTemplate,
            openshell_pb2.DeleteSandboxTemplateRequest(
                workspace_scope=_workspace_scope(workspace or self.workspace),
                name=name,
                allow_missing=True,
                request_id=uuid.uuid4().hex,
            ),
        )
        return response.outcome == openshell_pb2.DELETION_OUTCOME_COMPLETED

    def list_providers(
        self,
        workspace: Optional[str] = None,
        *,
        all_workspaces: bool = False,
        label_selector: str = "",
    ) -> List[Provider]:
        """List provider metadata, optionally across workspaces for admins."""
        if all_workspaces and workspace:
            raise ValueError("workspace and all_workspaces cannot be combined")
        token = ""
        seen_tokens = set()
        seen_providers = set()
        providers: List[Provider] = []
        while True:
            page = self.list_providers_page(
                workspace,
                page_size=1000,
                page_token=token,
                label_selector=label_selector,
                all_workspaces=all_workspaces,
            )
            _append_unique_page_items(
                providers,
                seen_providers,
                page.items,
                key=lambda item: (item.workspace, item.name),
                description="provider",
            )
            token = page.next_page_token
            if not token:
                return sorted(providers, key=lambda item: (item.workspace, item.name))
            if token in seen_tokens:
                raise RuntimeError(
                    "list providers: gateway repeated a pagination token"
                )
            seen_tokens.add(token)

    def list_providers_page(
        self,
        workspace: Optional[str] = None,
        *,
        page_size: int = 100,
        page_token: str = "",
        label_selector: str = "",
        all_workspaces: bool = False,
    ) -> CursorPage[Provider]:
        """List one page of redacted provider metadata."""
        _validate_page_size(page_size, "provider")
        if all_workspaces and workspace:
            raise ValueError("workspace and all_workspaces cannot be combined")
        page_size = min(page_size, 1000)
        scope = (
            datamodel_pb2.WorkspaceSelector(
                all_workspaces=datamodel_pb2.AllWorkspaces()
            )
            if all_workspaces
            else _workspace_scope(workspace or self.workspace)
        )
        response = self._call(
            openshell_pb2_grpc.OpenShellStub(self._rpc()).ListProviders,
            openshell_pb2.ListProvidersRequest(
                workspace_scope=scope,
                page_size=page_size,
                page_token=page_token,
                label_selector=label_selector,
            ),
        )
        return CursorPage(
            items=[_provider_from_proto(item) for item in response.providers],
            next_page_token=response.next_page_token,
        )

    def get_provider(self, name: str, workspace: Optional[str] = None) -> Provider:
        """Get provider metadata and credential key names, never secret values."""
        response = self._call(
            openshell_pb2_grpc.OpenShellStub(self._rpc()).GetProvider,
            openshell_pb2.GetProviderRequest(
                workspace_scope=_workspace_scope(workspace or self.workspace), name=name
            ),
        )
        return _provider_from_proto(response.provider)

    def put_provider(self, provider: Provider) -> None:
        """Create or update a provider. Credential values are write-only."""
        if not provider.name or not provider.type:
            raise ValueError("provider name and type are required")
        workspace = provider.workspace or self.workspace
        for refresh in provider.refresh.values():
            _refresh_strategy(refresh.strategy)
        if set(provider.env_vars) != set(provider.credentials):
            # OpenShell updates credential maps sparsely. Empty values remove
            # keys that were present before but are absent from the desired set.
            patch = dict(provider.credentials)
            patch.update({key: "" for key in provider.env_vars if key not in provider.credentials})
            self.update_provider_credentials(
                provider.name, patch, workspace=workspace
            )
            return

        stub = openshell_pb2_grpc.OpenShellStub(self._rpc())
        resource = _provider_to_proto(provider, workspace)
        request_id = uuid.uuid4().hex
        try:
            current = self._call(
                stub.GetProvider,
                openshell_pb2.GetProviderRequest(
                    workspace_scope=_workspace_scope(workspace), name=provider.name
                ),
            ).provider
        except grpc.RpcError as exc:
            if exc.code() != grpc.StatusCode.NOT_FOUND:
                raise
            self._call(
                stub.CreateProvider,
                openshell_pb2.CreateProviderRequest(
                    workspace_scope=_workspace_scope(workspace),
                    provider=resource,
                    request_id=request_id,
                ),
            )
        else:
            resource.metadata.id = current.metadata.id
            resource.metadata.resource_version = current.metadata.resource_version
            expiry = {
                key: _timestamp_from_millis(value)
                for key, value in provider.credential_expires_at_ms.items()
            }
            self._call(
                stub.UpdateProvider,
                openshell_pb2.UpdateProviderRequest(
                    workspace_scope=_workspace_scope(workspace),
                    provider=resource,
                    credential_expiration_times=expiry,
                    request_id=request_id,
                ),
            )
        for key, refresh in provider.refresh.items():
            self.configure_provider_refresh(
                provider.name,
                key,
                refresh.strategy,
                refresh.material,
                refresh.expires_at_ms,
                workspace=workspace,
            )

    def delete_provider(self, name: str, workspace: Optional[str] = None) -> None:
        self._call(
            openshell_pb2_grpc.OpenShellStub(self._rpc()).DeleteProvider,
            openshell_pb2.DeleteProviderRequest(
                workspace_scope=_workspace_scope(workspace or self.workspace),
                name=name,
                request_id=uuid.uuid4().hex,
            ),
        )

    def update_provider_credentials(
        self,
        name: str,
        credentials: Dict[str, str],
        workspace: Optional[str] = None,
    ) -> None:
        """Patch credential values without reading or replacing other secrets."""
        self._call(
            openshell_pb2_grpc.OpenShellStub(self._rpc()).UpdateProvider,
            openshell_pb2.UpdateProviderRequest(
                workspace_scope=_workspace_scope(workspace or self.workspace),
                provider=datamodel_pb2.Provider(
                    metadata=datamodel_pb2.ObjectMeta(name=name),
                    credentials=credentials,
                ),
                request_id=uuid.uuid4().hex,
            ),
        )

    def attach_provider(
        self, sandbox: str, provider: str, workspace: Optional[str] = None
    ) -> None:
        self._change_provider_attachment(sandbox, provider, True, workspace)

    def detach_provider(
        self, sandbox: str, provider: str, workspace: Optional[str] = None
    ) -> None:
        self._change_provider_attachment(sandbox, provider, False, workspace)

    def _change_provider_attachment(
        self, sandbox: str, provider: str, attach: bool, workspace: Optional[str]
    ) -> None:
        scope = _workspace_scope(workspace or self.workspace)
        stub = openshell_pb2_grpc.OpenShellStub(self._rpc())
        current = self._call(
            stub.GetSandbox,
            openshell_pb2.GetSandboxRequest(workspace_scope=scope, name=sandbox),
        ).sandbox
        common = {
            "workspace_scope": scope,
            "sandbox": sandbox,
            "provider": provider,
            "expected_resource_version": current.metadata.resource_version,
            "request_id": uuid.uuid4().hex,
        }
        if attach:
            request = openshell_pb2.AttachSandboxProviderRequest(**common)
            self._call(stub.AttachSandboxProvider, request)
        else:
            request = openshell_pb2.DetachSandboxProviderRequest(**common)
            self._call(stub.DetachSandboxProvider, request)

    def list_sandbox_providers(
        self, sandbox: str, workspace: Optional[str] = None
    ) -> List[Provider]:
        stub = openshell_pb2_grpc.OpenShellStub(self._rpc())
        scope = _workspace_scope(workspace or self.workspace)
        token = ""
        seen_tokens = set()
        seen_providers = set()
        providers: List[Provider] = []
        while True:
            response = self._call(
                stub.ListSandboxProviders,
                openshell_pb2.ListSandboxProvidersRequest(
                    workspace_scope=scope,
                    sandbox=sandbox,
                    page_size=1000,
                    page_token=token,
                ),
            )
            _append_unique_page_items(
                providers,
                seen_providers,
                (_provider_from_proto(item) for item in response.providers),
                key=lambda item: (item.workspace, item.name),
                description="sandbox provider",
            )
            token = response.next_page_token
            if not token:
                return sorted(providers, key=lambda item: item.name)
            if token in seen_tokens:
                raise RuntimeError(
                    "list sandbox providers: gateway repeated a pagination token"
                )
            seen_tokens.add(token)

    def list_sandbox_providers_page(
        self,
        sandbox: str,
        workspace: Optional[str] = None,
        *,
        page_size: int = 100,
        page_token: str = "",
    ) -> CursorPage[Provider]:
        """List one page of redacted providers attached to a sandbox."""
        _require_nonempty(sandbox, "sandbox name")
        _validate_page_size(page_size, "sandbox provider")
        page_size = min(page_size, 1000)
        response = self._call(
            openshell_pb2_grpc.OpenShellStub(self._rpc()).ListSandboxProviders,
            openshell_pb2.ListSandboxProvidersRequest(
                workspace_scope=_workspace_scope(workspace or self.workspace),
                sandbox=sandbox,
                page_size=page_size,
                page_token=page_token,
            ),
        )
        return CursorPage(
            items=[_provider_from_proto(item) for item in response.providers],
            next_page_token=response.next_page_token,
        )

    def configure_provider_refresh(
        self,
        name: str,
        credential_key: str,
        strategy: str,
        material: Dict[str, str],
        expires_at_ms: int = 0,
        workspace: Optional[str] = None,
    ) -> None:
        request = openshell_pb2.ConfigureProviderRefreshRequest(
            workspace_scope=_workspace_scope(workspace or self.workspace),
            provider=name,
            credential_key=credential_key,
            strategy=_refresh_strategy(strategy),
            material=material,
            request_id=uuid.uuid4().hex,
        )
        if expires_at_ms > 0:
            request.expiration_time.CopyFrom(_timestamp_from_millis(expires_at_ms))
        self._call(
            openshell_pb2_grpc.OpenShellStub(self._rpc()).ConfigureProviderRefresh,
            request,
        )

    def rotate_provider_refresh(
        self, name: str, credential_key: str, workspace: Optional[str] = None
    ) -> None:
        self._call(
            openshell_pb2_grpc.OpenShellStub(self._rpc()).RotateProviderCredential,
            openshell_pb2.RotateProviderCredentialRequest(
                workspace_scope=_workspace_scope(workspace or self.workspace),
                provider=name,
                credential_key=credential_key,
                request_id=uuid.uuid4().hex,
            ),
        )

    def delete_provider_refresh(
        self, name: str, credential_key: str, workspace: Optional[str] = None
    ) -> None:
        self._call(
            openshell_pb2_grpc.OpenShellStub(self._rpc()).DeleteProviderRefresh,
            openshell_pb2.DeleteProviderRefreshRequest(
                workspace_scope=_workspace_scope(workspace or self.workspace),
                provider=name,
                credential_key=credential_key,
                request_id=uuid.uuid4().hex,
            ),
        )

    def get_global_policy(self) -> bytes:
        response = self._call(
            openshell_pb2_grpc.OpenShellStub(self._rpc()).GetSandboxPolicyStatus,
            openshell_pb2.GetSandboxPolicyStatusRequest(**{"global": True}),
        )
        return _sandbox_policy_to_yaml(response.revision.policy).encode()

    def put_global_policy(self, document: bytes) -> None:
        policy = _sandbox_policy_from_yaml(document)
        self._call(
            openshell_pb2_grpc.OpenShellStub(self._rpc()).UpdateConfig,
            openshell_pb2.UpdateConfigRequest(
                policy=policy,
                request_id=str(uuid.uuid4()),
                **{"global": True},
            ),
        )

    def get_sandbox_policy(self, sandbox: str, view: str = "full") -> bytes:
        normalized = view.strip().lower()
        if normalized in ("", "effective"):
            normalized = "full"
        if normalized not in ("base", "full"):
            raise ValueError("policy view must be base or full")
        response = self._call(
            openshell_pb2_grpc.OpenShellStub(self._rpc()).GetSandboxPolicyStatus,
            openshell_pb2.GetSandboxPolicyStatusRequest(
                workspace_scope=_workspace_scope(self.workspace), sandbox=sandbox
            ),
        )
        return _sandbox_policy_to_yaml(response.revision.policy).encode()

    def effective_policy(self, sandbox: str) -> bytes:
        return self.get_sandbox_policy(sandbox, "full")

    def put_sandbox_policy(self, sandbox: str, document: bytes) -> tuple[bytes, int]:
        policy = _sandbox_policy_from_yaml(document)
        stub = openshell_pb2_grpc.OpenShellStub(self._rpc())
        current = self._call(
            stub.GetSandbox,
            openshell_pb2.GetSandboxRequest(
                workspace_scope=_workspace_scope(self.workspace), name=sandbox
            ),
        )
        updated = self._call(
            stub.UpdateConfig,
            openshell_pb2.UpdateConfigRequest(
                workspace_scope=_workspace_scope(self.workspace),
                sandbox=sandbox,
                policy=policy,
                expected_resource_version=current.sandbox.metadata.resource_version,
                request_id=str(uuid.uuid4()),
            ),
        )
        status_response = self._call(
            stub.GetSandboxPolicyStatus,
            openshell_pb2.GetSandboxPolicyStatusRequest(
                workspace_scope=_workspace_scope(self.workspace),
                sandbox=sandbox,
                version=updated.version,
            ),
        )
        return _sandbox_policy_to_yaml(status_response.revision.policy).encode(), 0

    def list_policy_revisions(self, sandbox: str) -> List[PolicyRevision]:
        response = self.list_policy_revision_records(sandbox)
        return [
            PolicyRevision(
                rev=item.rev,
                updated_at=item.created_at or datetime.fromtimestamp(0, timezone.utc),
                bytes=0,
                status=item.status,
            )
            for item in response
        ]

    def list_policy_revisions_page(
        self, sandbox: str, *, page_size: int = 100, page_token: str = ""
    ) -> CursorPage[PolicyRevisionRecord]:
        """List one page of policy revision metadata for a sandbox."""
        _require_nonempty(sandbox, "sandbox name")
        _validate_page_size(page_size, "policy revision")
        response = self._call(
            openshell_pb2_grpc.OpenShellStub(self._rpc()).ListSandboxPolicies,
            openshell_pb2.ListSandboxPoliciesRequest(
                workspace_scope=_workspace_scope(self.workspace),
                sandbox=sandbox,
                page_size=min(page_size, 1000),
                page_token=page_token,
            ),
        )
        return CursorPage(
            items=_policy_revision_records_from_proto(response.revisions),
            next_page_token=response.next_page_token,
        )

    def list_global_policy_revisions_page(
        self, *, page_size: int = 100, page_token: str = ""
    ) -> CursorPage[PolicyRevisionRecord]:
        """List one page of platform-wide policy history (platform admin only)."""
        _validate_page_size(page_size, "global policy revision")
        response = self._call(
            openshell_pb2_grpc.OpenShellStub(self._rpc()).ListSandboxPolicies,
            openshell_pb2.ListSandboxPoliciesRequest(
                **{
                    "global": True,
                    "page_size": min(page_size, 1000),
                    "page_token": page_token,
                }
            ),
        )
        return CursorPage(
            items=_policy_revision_records_from_proto(response.revisions),
            next_page_token=response.next_page_token,
        )

    def list_policy_revision_records(self, sandbox: str) -> List[PolicyRevisionRecord]:
        """Collect Gateway policy revision metadata across all cursor pages."""
        records: List[PolicyRevisionRecord] = []
        seen_tokens = set()
        seen_revisions = set()
        page_token = ""
        while True:
            page = self.list_policy_revisions_page(
                sandbox, page_size=1000, page_token=page_token
            )
            _append_unique_policy_revisions(records, seen_revisions, page.items)
            page_token = page.next_page_token
            if not page_token:
                return records
            if page_token in seen_tokens:
                raise RuntimeError(
                    "list policy revision records: gateway repeated a pagination token"
                )
            seen_tokens.add(page_token)

    def list_global_policy_revision_records(self) -> List[PolicyRevisionRecord]:
        """Collect global Gateway policy history (platform admin only)."""
        records: List[PolicyRevisionRecord] = []
        seen_tokens = set()
        seen_revisions = set()
        page_token = ""
        while True:
            page = self.list_global_policy_revisions_page(
                page_size=1000, page_token=page_token
            )
            _append_unique_policy_revisions(records, seen_revisions, page.items)
            page_token = page.next_page_token
            if not page_token:
                return records
            if page_token in seen_tokens:
                raise RuntimeError(
                    "list global policy revision records: gateway repeated a pagination token"
                )
            seen_tokens.add(page_token)

    def get_policy_revision(self, sandbox: str, revision: int) -> str:
        if revision <= 0:
            raise ValueError("policy revision must be positive")
        response = self._call(
            openshell_pb2_grpc.OpenShellStub(self._rpc()).GetSandboxPolicyStatus,
            openshell_pb2.GetSandboxPolicyStatusRequest(
                workspace_scope=_workspace_scope(self.workspace),
                sandbox=sandbox,
                version=revision,
            ),
        )
        return _sandbox_policy_to_yaml(response.revision.policy)

    def get_draft_policy(self, sandbox: str, status_filter: str = ""):
        """Return the native OpenShell draft policy response for one sandbox."""
        _require_nonempty(sandbox, "sandbox name")
        return self._call(
            openshell_pb2_grpc.OpenShellStub(self._rpc()).GetDraftPolicy,
            openshell_pb2.GetDraftPolicyRequest(
                workspace_scope=_workspace_scope(self.workspace),
                sandbox=sandbox,
                status_filter=status_filter,
            ),
        )

    def approve_draft_chunk(self, sandbox: str, chunk_id: str, review_token: str):
        """Approve the exact native OpenShell draft chunk and reviewed inputs."""
        _require_nonempty(sandbox, "sandbox name")
        _require_nonempty(chunk_id, "draft chunk ID")
        _require_nonempty(review_token, "draft review token")
        return self._call(
            openshell_pb2_grpc.OpenShellStub(self._rpc()).ApproveDraftChunk,
            openshell_pb2.ApproveDraftChunkRequest(
                workspace_scope=_workspace_scope(self.workspace),
                sandbox=sandbox,
                chunk_id=chunk_id,
                review_token=review_token,
                request_id=str(uuid.uuid4()),
            ),
        )

    def reject_draft_chunk(self, sandbox: str, chunk_id: str, reason: str = ""):
        """Reject one native OpenShell draft chunk."""
        _require_nonempty(sandbox, "sandbox name")
        _require_nonempty(chunk_id, "draft chunk ID")
        return self._call(
            openshell_pb2_grpc.OpenShellStub(self._rpc()).RejectDraftChunk,
            openshell_pb2.RejectDraftChunkRequest(
                workspace_scope=_workspace_scope(self.workspace),
                sandbox=sandbox,
                chunk_id=chunk_id,
                reason=reason,
                request_id=str(uuid.uuid4()),
            ),
        )

    def list_profiles(
        self, scope: str = "global", workspace: str = ""
    ) -> List[ProfileInfo]:
        result: List[ProfileInfo] = []
        token = ""
        seen_tokens = set()
        seen_profiles = set()
        while True:
            page = self.list_profiles_page(
                scope, workspace, page_size=1000, page_token=token
            )
            _append_unique_page_items(
                result,
                seen_profiles,
                page.items,
                key=lambda item: item.id,
                description="provider profile",
            )
            token = page.next_page_token
            if not token:
                return result
            if token in seen_tokens:
                raise RuntimeError("list profiles: gateway repeated a pagination token")
            seen_tokens.add(token)

    def list_profiles_page(
        self,
        scope: str = "global",
        workspace: str = "",
        *,
        page_size: int = 100,
        page_token: str = "",
    ) -> CursorPage[ProfileInfo]:
        """List one cursor page from the global or workspace profile catalog."""
        _validate_page_size(page_size, "provider profile")
        page_size = min(page_size, 1000)
        normalized_scope = scope.strip().lower()
        if normalized_scope not in ("global", "workspace"):
            raise ValueError("profile scope must be 'global' or 'workspace'")
        if normalized_scope == "workspace":
            _require_nonempty(workspace, "workspace name")
            workspace_scope = _workspace_scope(workspace)
        else:
            workspace_scope = None
        request = openshell_pb2.ListProviderProfilesRequest(
            page_size=page_size, page_token=page_token
        )
        if workspace_scope is not None:
            request.workspace_scope.CopyFrom(workspace_scope)
        response = self._call(
            openshell_pb2_grpc.OpenShellStub(self._rpc()).ListProviderProfiles,
            request,
        )
        profiles = [
            ProfileInfo(
                id=item.id,
                category=openshell_pb2.ProviderProfileCategory.Name(item.category)
                .replace("PROVIDER_PROFILE_CATEGORY_", "")
                .lower(),
                source=item.source,
                scope=item.scope,
                resource_version=item.resource_version,
            )
            for item in response.profiles
        ]
        return CursorPage(items=profiles, next_page_token=response.next_page_token)

    def get_profile(
        self, profile_id: str, scope: str = "global", workspace: str = ""
    ) -> ProfileDocument:
        response = self._call(
            openshell_pb2_grpc.OpenShellStub(self._rpc()).GetProviderProfile,
            openshell_pb2.GetProviderProfileRequest(
                workspace_scope=_profile_workspace_scope(scope, workspace), id=profile_id
            ),
        )
        profile = response.profile
        document = MessageToDict(profile, preserving_proto_field_name=True)
        category = document.get("category", "")
        document["category"] = category.replace(
            "PROVIDER_PROFILE_CATEGORY_", ""
        ).lower()
        return ProfileDocument(
            yaml=yaml.safe_dump(document, sort_keys=False).encode(),
            source=profile.source,
            resource_version=str(profile.resource_version),
        )

    def create_profile(
        self,
        profile_id: str,
        profile_yaml: bytes,
        scope: str = "global",
        workspace: str = "",
    ) -> None:
        _validate_profile_document(profile_yaml)
        profile = _provider_profile_from_yaml(profile_yaml, profile_id)
        self._call(
            openshell_pb2_grpc.OpenShellStub(self._rpc()).ImportProviderProfiles,
            openshell_pb2.ImportProviderProfilesRequest(
                workspace_scope=_profile_workspace_scope(scope, workspace),
                profiles=[openshell_pb2.ProviderProfileImportItem(profile=profile)],
            ),
        )

    def put_profile(
        self,
        profile_id: str,
        profile_yaml: bytes,
        expected_version: str,
        scope: str = "global",
        workspace: str = "",
    ) -> None:
        try:
            version = int(expected_version.strip().strip('"'))
        except (AttributeError, ValueError) as exc:
            raise ValueError(
                "profile update requires a positive resource version"
            ) from exc
        if version <= 0:
            raise ValueError("profile update requires a positive resource version")
        _validate_profile_document(profile_yaml)
        profile = _provider_profile_from_yaml(profile_yaml, profile_id)
        self._call(
            openshell_pb2_grpc.OpenShellStub(self._rpc()).UpdateProviderProfiles,
            openshell_pb2.UpdateProviderProfilesRequest(
                workspace_scope=_profile_workspace_scope(scope, workspace),
                id=profile_id,
                expected_resource_version=version,
                profile=openshell_pb2.ProviderProfileImportItem(profile=profile),
            ),
        )

    def delete_profile(
        self, profile_id: str, scope: str = "global", workspace: str = ""
    ) -> None:
        self._call(
            openshell_pb2_grpc.OpenShellStub(self._rpc()).DeleteProviderProfile,
            openshell_pb2.DeleteProviderProfileRequest(
                workspace_scope=_profile_workspace_scope(scope, workspace), id=profile_id
            ),
        )

    def list_workspaces(self, *, page_size: int = 1000) -> List[Workspace]:
        """List all visible workspaces, following the gateway cursor safely."""
        _validate_page_size(page_size, "workspace")
        page_size = min(page_size, 1000)
        stub = openshell_pb2_grpc.OpenShellStub(self._rpc())
        result: List[Workspace] = []
        seen_tokens = set()
        seen_workspaces = set()
        page_token = ""
        while True:
            response = self._call(
                stub.ListWorkspaces,
                openshell_pb2.ListWorkspacesRequest(
                    page_size=page_size,
                    page_token=page_token,
                ),
            )
            _append_unique_page_items(
                result,
                seen_workspaces,
                (_workspace_from_record(item) for item in response.workspaces),
                key=lambda item: item.id or item.name,
                description="workspace",
            )
            page_token = response.next_page_token
            if not page_token:
                return result
            if page_token in seen_tokens:
                raise RuntimeError("gateway repeated a workspace page cursor")
            seen_tokens.add(page_token)

    def list_workspaces_page(
        self, *, page_size: int = 100, page_token: str = "", label_selector: str = ""
    ) -> CursorPage[Workspace]:
        """List one page of visible workspaces."""
        _validate_page_size(page_size, "workspace")
        page_size = min(page_size, 1000)
        response = self._call(
            openshell_pb2_grpc.OpenShellStub(self._rpc()).ListWorkspaces,
            openshell_pb2.ListWorkspacesRequest(
                page_size=page_size,
                page_token=page_token,
                label_selector=label_selector,
            ),
        )
        return CursorPage(
            items=[_workspace_from_record(item) for item in response.workspaces],
            next_page_token=response.next_page_token,
        )

    def list_workspace_members_page(
        self,
        workspace: str,
        *,
        page_size: int = 100,
        page_token: str = "",
    ) -> CursorPage[WorkspaceMember]:
        """List one page of members from a workspace."""
        _require_nonempty(workspace, "workspace name")
        _validate_page_size(page_size, "workspace member")
        page_size = min(page_size, 1000)
        response = self._call(
            openshell_pb2_grpc.OpenShellStub(self._rpc()).ListWorkspaceMembers,
            openshell_pb2.ListWorkspaceMembersRequest(
                workspace_scope=_workspace_scope(workspace),
                page_size=page_size,
                page_token=page_token,
            ),
        )
        return CursorPage(
            items=[_workspace_member_from_proto(item) for item in response.members],
            next_page_token=response.next_page_token,
        )

    def get_workspace(self, name: str) -> Workspace:
        """Read a workspace and all of its members."""
        _require_nonempty(name, "workspace name")
        stub = openshell_pb2_grpc.OpenShellStub(self._rpc())
        response = self._call(
            stub.GetWorkspace, openshell_pb2.GetWorkspaceRequest(name=name)
        )
        workspace = _workspace_from_record(response.workspace)
        seen_tokens = set()
        seen_members = set()
        page_token = ""
        while True:
            members = self._call(
                stub.ListWorkspaceMembers,
                openshell_pb2.ListWorkspaceMembersRequest(
                    workspace_scope=_workspace_scope(name),
                    page_size=1000,
                    page_token=page_token,
                ),
            )
            _append_unique_page_items(
                workspace.members,
                seen_members,
                (
                    WorkspaceMember(
                        subject=item.principal_subject,
                        role=(
                            "admin"
                            if item.role == openshell_pb2.WORKSPACE_ROLE_ADMIN
                            else "user"
                        ),
                    )
                    for item in members.members
                ),
                key=lambda item: item.subject,
                description="workspace member",
            )
            page_token = members.next_page_token
            if not page_token:
                return workspace
            if page_token in seen_tokens:
                raise RuntimeError("gateway repeated a workspace member cursor")
            seen_tokens.add(page_token)

    def create_workspace(
        self, name: str, *, labels: Optional[Dict[str, str]] = None
    ) -> Workspace:
        """Create a workspace with optional metadata labels."""
        _require_nonempty(name, "workspace name")
        stub = openshell_pb2_grpc.OpenShellStub(self._rpc())
        response = self._call(
            stub.CreateWorkspace,
            openshell_pb2.CreateWorkspaceRequest(
                name=name,
                labels=labels or {},
                request_id=str(uuid.uuid4()),
            ),
        )
        return _workspace_from_record(response.workspace)

    def delete_workspace(self, name: str, *, allow_missing: bool = False) -> None:
        """Delete a workspace."""
        _require_nonempty(name, "workspace name")
        stub = openshell_pb2_grpc.OpenShellStub(self._rpc())
        self._call(
            stub.DeleteWorkspace,
            openshell_pb2.DeleteWorkspaceRequest(
                name=name,
                allow_missing=allow_missing,
                request_id=str(uuid.uuid4()),
            ),
        )

    def add_workspace_member(self, name: str, subject: str, role: str) -> None:
        """Add or update a workspace member as a user or admin."""
        _require_nonempty(name, "workspace name")
        _require_nonempty(subject, "member subject")
        normalized_role = role.lower()
        if normalized_role not in ("user", "admin"):
            raise ValueError("workspace member role must be 'user' or 'admin'")
        enum_role = (
            openshell_pb2.WORKSPACE_ROLE_ADMIN
            if normalized_role == "admin"
            else openshell_pb2.WORKSPACE_ROLE_USER
        )
        stub = openshell_pb2_grpc.OpenShellStub(self._rpc())
        self._call(
            stub.AddWorkspaceMember,
            openshell_pb2.AddWorkspaceMemberRequest(
                workspace_scope=_workspace_scope(name),
                principal_subject=subject,
                role=enum_role,
                request_id=str(uuid.uuid4()),
            ),
        )

    def remove_workspace_member(
        self, name: str, subject: str, *, allow_missing: bool = False
    ) -> None:
        """Remove a member from a workspace."""
        _require_nonempty(name, "workspace name")
        _require_nonempty(subject, "member subject")
        stub = openshell_pb2_grpc.OpenShellStub(self._rpc())
        self._call(
            stub.RemoveWorkspaceMember,
            openshell_pb2.RemoveWorkspaceMemberRequest(
                workspace_scope=_workspace_scope(name),
                principal_subject=subject,
                allow_missing=allow_missing,
                request_id=str(uuid.uuid4()),
            ),
        )

    def expose_service(
        self,
        sandbox: str,
        name: str,
        port: int,
        *,
        domain: bool = True,
        authorization_mode: str = "",
    ) -> ServiceEndpoint:
        """Expose a TCP port from a sandbox through the gateway relay."""
        _require_nonempty(sandbox, "sandbox name")
        if not 1 <= port <= 65535:
            raise ValueError("service target port must be between 1 and 65535")
        stub = openshell_pb2_grpc.OpenShellStub(self._rpc())
        request = openshell_pb2.ExposeServiceRequest(
            workspace_scope=_workspace_scope(self.workspace),
            sandbox=sandbox,
            name=name,
            target_port=port,
            domain=domain,
            request_id=str(uuid.uuid4()),
        )
        if authorization_mode:
            request.authorization_mode = _service_authorization_mode(authorization_mode)
        response = self._call(stub.ExposeService, request)
        return _service_endpoint_from_proto(response.endpoint)

    def get_service(self, sandbox: str, name: str = "") -> ServiceEndpoint:
        """Read one exposed endpoint in the selected workspace."""
        _require_nonempty(sandbox, "sandbox name")
        stub = openshell_pb2_grpc.OpenShellStub(self._rpc())
        response = self._call(
            stub.GetService,
            openshell_pb2.GetServiceRequest(
                workspace_scope=_workspace_scope(self.workspace),
                sandbox=sandbox,
                name=name,
            ),
        )
        return _service_endpoint_from_proto(response.endpoint)

    def list_services(
        self,
        *,
        sandbox: str = "",
        page_size: int = 1000,
        all_workspaces: bool = False,
    ) -> List[ServiceEndpoint]:
        """List exposed endpoints, following pages and rejecting cursor loops."""
        _validate_page_size(page_size, "service")
        page_size = min(page_size, 1000)
        stub = openshell_pb2_grpc.OpenShellStub(self._rpc())
        scope = (
            datamodel_pb2.WorkspaceSelector(
                all_workspaces=datamodel_pb2.AllWorkspaces()
            )
            if all_workspaces
            else _workspace_scope(self.workspace)
        )
        result: List[ServiceEndpoint] = []
        seen_tokens = set()
        seen_services = set()
        page_token = ""
        while True:
            response = self._call(
                stub.ListServices,
                openshell_pb2.ListServicesRequest(
                    workspace_scope=scope,
                    sandbox=sandbox,
                    page_size=page_size,
                    page_token=page_token,
                ),
            )
            _append_unique_page_items(
                result,
                seen_services,
                (
                    _service_endpoint_from_proto(item.endpoint, url=item.url)
                    for item in response.services
                ),
                key=lambda item: (item.sandbox, item.name),
                description="service endpoint",
            )
            page_token = response.next_page_token
            if not page_token:
                return result
            if page_token in seen_tokens:
                raise RuntimeError("gateway repeated a service page cursor")
            seen_tokens.add(page_token)

    def list_services_page(
        self,
        *,
        sandbox: str = "",
        page_size: int = 100,
        page_token: str = "",
        all_workspaces: bool = False,
    ) -> CursorPage[ServiceEndpoint]:
        """List one page of exposed service endpoints."""
        _validate_page_size(page_size, "service")
        page_size = min(page_size, 1000)
        scope = (
            datamodel_pb2.WorkspaceSelector(
                all_workspaces=datamodel_pb2.AllWorkspaces()
            )
            if all_workspaces
            else _workspace_scope(self.workspace)
        )
        response = self._call(
            openshell_pb2_grpc.OpenShellStub(self._rpc()).ListServices,
            openshell_pb2.ListServicesRequest(
                workspace_scope=scope,
                sandbox=sandbox,
                page_size=page_size,
                page_token=page_token,
            ),
        )
        return CursorPage(
            items=[
                _service_endpoint_from_proto(item.endpoint, url=item.url)
                for item in response.services
            ],
            next_page_token=response.next_page_token,
        )

    def delete_service(
        self,
        sandbox: str,
        name: str = "",
        *,
        allow_missing: bool = False,
    ) -> None:
        """Delete an exposed service endpoint."""
        _require_nonempty(sandbox, "sandbox name")
        stub = openshell_pb2_grpc.OpenShellStub(self._rpc())
        self._call(
            stub.DeleteService,
            openshell_pb2.DeleteServiceRequest(
                workspace_scope=_workspace_scope(self.workspace),
                sandbox=sandbox,
                name=name,
                allow_missing=allow_missing,
                request_id=str(uuid.uuid4()),
            ),
        )

    def upload_rootfs_tar(
        self, file_name: str, size_bytes: int, source: BinaryIO
    ) -> str:
        """Write a rootfs tar to the OpenShell gateway-local staging path."""
        if (
            not file_name.strip()
            or "/" in file_name
            or "\\" in file_name
            or file_name in (".", "..")
        ):
            raise ValueError("rootfs tar file_name must be a base name")
        if (
            source is None
            or isinstance(size_bytes, bool)
            or not 0 < size_bytes < (1 << 63)
        ):
            raise ValueError("rootfs tar size and source are required")
        stub = openshell_pb2_grpc.OpenShellStub(self._rpc())
        staging = self._call(
            stub.BeginRootfsTarStaging,
            openshell_pb2.BeginRootfsTarStagingRequest(
                workspace_scope=_workspace_scope(self.workspace),
                file_name=file_name,
                size_bytes=size_bytes,
            ),
        )
        token = staging.staging_token
        if not token:
            raise RuntimeError("gateway returned an empty rootfs staging token")
        if not staging.upload_path:
            raise RuntimeError("gateway returned an empty rootfs staging path")
        # OpenShell returns a gateway-local path. As in the upstream CLI, this
        # operation requires the client to share that filesystem.
        fd = os.open(
            staging.upload_path,
            os.O_WRONLY | os.O_CREAT | os.O_EXCL,
            0o600,
        )
        try:
            with os.fdopen(fd, "wb") as output:
                remaining = size_bytes
                while remaining:
                    chunk = source.read(min(64 * 1024, remaining))
                    if not chunk:
                        raise ValueError("rootfs tar source ended before size_bytes")
                    if not isinstance(chunk, bytes):
                        raise TypeError("rootfs tar source must return bytes")
                    if len(chunk) > remaining:
                        raise ValueError("rootfs tar source exceeded size_bytes")
                    output.write(chunk)
                    remaining -= len(chunk)
                if source.read(1):
                    raise ValueError("rootfs tar source exceeded size_bytes")
                output.flush()
                os.fsync(output.fileno())
        except BaseException:
            try:
                os.unlink(staging.upload_path)
            except FileNotFoundError:
                pass
            raise
        return token

    def get_setting(self, key: str) -> object:
        """Read a configured gateway setting as its native Python value."""
        values = self.list_settings()
        if key not in values:
            raise KeyError(f"gateway setting {key!r} is not configured")
        return values[key]

    def list_settings(self) -> Dict[str, object]:
        """List gateway settings without including the reserved policy value."""
        stub = openshell_pb2_grpc.OpenShellStub(self._rpc())
        response = self._call(
            stub.GetGatewayConfig,
            sandbox_pb2.GetGatewayConfigRequest(),
        )
        return {
            key: _setting_value_from_proto(value)
            for key, value in response.settings.items()
        }

    def put_setting(self, key: str, value: object) -> None:
        """Set a global gateway setting using its string/bool/int/bytes type."""
        _require_nonempty(key, "setting key")
        setting_value = _setting_value_to_proto(value)
        stub = openshell_pb2_grpc.OpenShellStub(self._rpc())
        request = openshell_pb2.UpdateConfigRequest(
            setting_key=key,
            setting_value=setting_value,
            request_id=str(uuid.uuid4()),
        )
        setattr(request, "global", True)
        self._call(
            stub.UpdateConfig,
            request,
        )

    def connect(self, name: str) -> None:
        raise ConnectUnsupported(
            "interactive connect is not supported in the Python SDK; "
            f"use the quarshell CLI for {name}"
        )


def _sandbox_from_native(sandbox: openshell_pb2.Sandbox) -> Sandbox:
    metadata = sandbox.metadata
    spec = sandbox.spec
    status = sandbox.status
    created_at = metadata.created_time.ToMilliseconds() if metadata.HasField("created_time") else 0
    image = spec.template.image if spec.HasField("template") else ""
    template_name = sandbox.created_from_workload_template.name if sandbox.HasField("created_from_workload_template") else ""
    return Sandbox(
        name=metadata.name,
        status=openshell_pb2.SandboxPhase.Name(status.phase).removeprefix("SANDBOX_PHASE_").lower(),
        id=metadata.id,
        image=image,
        workspace=metadata.workspace or "default",
        resource_version=int(metadata.resource_version),
        labels=dict(metadata.labels),
        command=list(spec.command),
        updated_at_unix_ms=created_at,
        workload_template_name=template_name,
        tty=spec.tty,
        environment=dict(spec.environment),
    )


def _sandbox_log_line(line: openshell_pb2.SandboxLogLine) -> SandboxLogLine:
    return SandboxLogLine(
        timestamp_unix_ms=line.event_time.ToMilliseconds() if line.HasField("event_time") else 0,
        source=line.source,
        level=line.level,
        message=line.message,
        target=line.target,
    )


def _workspace_from_record(record: datamodel_pb2.Workspace) -> Workspace:
    metadata = record.metadata
    created_at_ms = 0
    if metadata.HasField("created_time"):
        created_at_ms = int(
            metadata.created_time.seconds * 1000
            + metadata.created_time.nanos // 1_000_000
        )
    status = ""
    if record.HasField("status"):
        status = datamodel_pb2.WorkspacePhase.Name(record.status.phase)
    return Workspace(
        name=metadata.name,
        id=metadata.id,
        status=status,
        resource_version=metadata.resource_version,
        labels=dict(metadata.labels),
        created_at_ms=created_at_ms,
    )


def _workspace_member_from_proto(
    item: openshell_pb2.WorkspaceMember,
) -> WorkspaceMember:
    return WorkspaceMember(
        subject=item.principal_subject,
        role=("admin" if item.role == openshell_pb2.WORKSPACE_ROLE_ADMIN else "user"),
    )


def _service_endpoint_from_proto(
    endpoint: Optional[openshell_pb2.ServiceEndpoint], *, url: str = ""
) -> ServiceEndpoint:
    if endpoint is None:
        return ServiceEndpoint(name="", sandbox="", port=0, url=url)
    return ServiceEndpoint(
        name=endpoint.name,
        sandbox=endpoint.sandbox,
        port=endpoint.target_port,
        url=url,
        domain=endpoint.domain,
        authorization_mode=openshell_pb2.ServiceAuthorizationMode.Name(
            endpoint.authorization_mode
        ),
    )


def _service_authorization_mode(value: str) -> int:
    modes = {
        "strip": openshell_pb2.SERVICE_AUTHORIZATION_MODE_STRIP,
        "bearer-passthrough": (
            openshell_pb2.SERVICE_AUTHORIZATION_MODE_BEARER_PASSTHROUGH
        ),
    }
    try:
        return modes[value.lower().replace("_", "-")]
    except KeyError as exc:
        raise ValueError(f"unsupported service authorization mode {value!r}") from exc


def _setting_value_from_proto(value: sandbox_pb2.SettingValue) -> object:
    selected = value.WhichOneof("value")
    if selected is None:
        return None
    return getattr(value, selected)


def _setting_value_to_proto(value: object) -> sandbox_pb2.SettingValue:
    if isinstance(value, bool):
        return sandbox_pb2.SettingValue(bool_value=value)
    if isinstance(value, str):
        return sandbox_pb2.SettingValue(string_value=value)
    if isinstance(value, int):
        if not -(1 << 63) <= value < (1 << 63):
            raise ValueError("integer setting value must fit signed int64")
        return sandbox_pb2.SettingValue(int_value=value)
    if isinstance(value, bytes):
        return sandbox_pb2.SettingValue(bytes_value=value)
    raise TypeError("setting value must be str, bool, int, or bytes")


def _require_nonempty(value: str, description: str) -> None:
    if not value or not value.strip():
        raise ValueError(f"{description} must not be empty")


def _validate_page_marker(marker: int, limit: int, description: str) -> None:
    if isinstance(marker, bool) or not 0 <= marker < (1 << 64):
        raise ValueError(f"{description} page marker must fit uint64")
    if isinstance(limit, bool) or limit < 0:
        raise ValueError(f"{description} page limit must not be negative")


def _validate_page_size(page_size: int, description: str) -> None:
    if isinstance(page_size, bool) or not isinstance(page_size, int) or page_size < 0:
        raise ValueError(f"{description} page size must be a non-negative integer")


def _workspace_scope(workspace: str) -> datamodel_pb2.WorkspaceSelector:
    return datamodel_pb2.WorkspaceSelector(workspace=workspace or "default")


def _driver_config(value: str) -> Optional[struct_pb2.Struct]:
    if not value:
        return None
    try:
        data = json.loads(value)
    except json.JSONDecodeError as exc:
        raise ValueError(
            "template driver config must be a non-empty JSON object"
        ) from exc
    if not isinstance(data, dict) or not data:
        raise ValueError("template driver config must be a non-empty JSON object")
    result = struct_pb2.Struct()
    result.update(data)
    return result


def _workload_template_from_proto(
    template: openshell_pb2.SandboxWorkloadTemplate,
) -> WorkloadTemplate:
    result = WorkloadTemplate(name="", image="", workspace="")
    if template.HasField("metadata"):
        result.name = template.metadata.name
        result.workspace = template.metadata.workspace or "default"
        result.labels = dict(template.metadata.labels)
    if template.HasField("spec"):
        spec = template.spec
        if spec.HasField("workload"):
            workload = spec.workload
            result.image = workload.image
            result.environment = dict(workload.environment)
            if workload.HasField("resources"):
                resources = workload.resources
                result.cpu = resources.cpu
                result.memory = resources.memory
                if resources.HasField("gpu"):
                    result.gpu = True
                    result.gpu_count = resources.gpu.count
        if spec.HasField("driver_config"):
            result.driver_config_json = json.dumps(
                MessageToDict(spec.driver_config), sort_keys=True, separators=(",", ":")
            )
    return result


def _timestamp_from_millis(value: int) -> Timestamp:
    timestamp = Timestamp()
    timestamp.FromMilliseconds(value)
    return timestamp


def _refresh_strategy(strategy: str) -> int:
    strategies = {
        "oauth2-refresh-token": (
            openshell_pb2.PROVIDER_CREDENTIAL_REFRESH_STRATEGY_OAUTH2_REFRESH_TOKEN
        ),
        "oauth2-client-credentials": (
            openshell_pb2.PROVIDER_CREDENTIAL_REFRESH_STRATEGY_OAUTH2_CLIENT_CREDENTIALS
        ),
    }
    normalized = strategy.lower().replace("_", "-")
    try:
        return strategies[normalized]
    except KeyError as exc:
        raise ValueError(f"unsupported refresh strategy {strategy!r}") from exc


def _provider_to_proto(provider: Provider, workspace: str) -> datamodel_pb2.Provider:
    result = datamodel_pb2.Provider(
        metadata=datamodel_pb2.ObjectMeta(
            name=provider.name,
            workspace=workspace,
            annotations={
                "cautem.io/runtime-credentials": str(
                    provider.runtime_credentials
                ).lower()
            },
        ),
        type=provider.type,
        credentials=provider.credentials,
        config=provider.config,
    )
    for key, value in provider.credential_expires_at_ms.items():
        result.credential_expiration_times[key].CopyFrom(_timestamp_from_millis(value))
    return result


def _provider_from_proto(provider: datamodel_pb2.Provider) -> Provider:
    expirations = {
        key: value.seconds * 1000 + value.nanos // 1_000_000
        for key, value in provider.credential_expiration_times.items()
    }
    refresh = {}
    annotations = provider.metadata.annotations
    runtime_credentials = (
        annotations.get("cautem.io/runtime-credentials", "").lower() == "true"
    )
    return Provider(
        name=provider.metadata.name,
        type=provider.type,
        workspace=provider.metadata.workspace or "default",
        env_vars=sorted(provider.credentials.keys()),
        credential_expires_at_ms=expirations,
        runtime_credentials=runtime_credentials,
        config=dict(provider.config),
        refresh=refresh,
    )


def _datetime_from_millis(value: int) -> datetime:
    seconds, milliseconds = divmod(value, 1000)
    return datetime.fromtimestamp(seconds, timezone.utc).replace(
        microsecond=milliseconds * 1000
    )


def _sandbox_policy_from_yaml(document: bytes):
    if len(document) > 1 << 20:
        raise ValueError("policy document must not exceed 1 MiB")
    try:
        fields = yaml.safe_load(document) if document else {}
    except yaml.YAMLError as exc:
        raise ValueError(f"policy document is invalid YAML: {exc}") from exc
    if not isinstance(fields, dict):
        raise ValueError("policy document must be a YAML mapping")
    fields = dict(fields)
    if "filesystem_policy" in fields:
        if "filesystem" in fields:
            raise ValueError("policy document cannot contain both filesystem and filesystem_policy")
        fields["filesystem"] = fields.pop("filesystem_policy")
    _policy_enum_values(fields, to_proto=True)
    policy = sandbox_pb2.SandboxPolicy()
    ParseDict(fields, policy, ignore_unknown_fields=False)
    if policy.version == 0:
        policy.version = 1
    return policy


def _sandbox_policy_to_yaml(policy) -> str:
    if policy is None:
        raise ValueError("OpenShell policy response did not include a policy")
    fields = MessageToDict(policy, preserving_proto_field_name=True)
    _policy_enum_values(fields, to_proto=False)
    return yaml.safe_dump(fields, sort_keys=False)


def _policy_enum_values(value, *, to_proto: bool) -> None:
    if isinstance(value, dict):
        for key, child in list(value.items()):
            if isinstance(child, str):
                prefix = {
                    "tls": "NETWORK_TLS_MODE_",
                    "enforcement": "NETWORK_ENFORCEMENT_MODE_",
                    "access": "NETWORK_ACCESS_PRESET_",
                }.get(key)
                if prefix:
                    if to_proto:
                        normalized = child.upper().replace("-", "_").replace(" ", "_")
                        value[key] = prefix + normalized
                    elif child.startswith(prefix):
                        normalized = child[len(prefix) :].lower()
                        value[key] = normalized.replace("_", "-") if key == "access" else normalized
                    continue
            _policy_enum_values(child, to_proto=to_proto)
    elif isinstance(value, list):
        for child in value:
            _policy_enum_values(child, to_proto=to_proto)


def _policy_status_name(value: int) -> str:
    try:
        return openshell_pb2.PolicyStatus.Name(value).removeprefix("POLICY_STATUS_").lower()
    except ValueError:
        return f"POLICY_STATUS_UNKNOWN_{value}"


def _policy_revision_records_from_proto(revisions) -> List[PolicyRevisionRecord]:
    records = []
    for item in revisions:
        created_at = (
            item.created_time.ToDatetime(tzinfo=timezone.utc)
            if item.HasField("created_time")
            else None
        )
        loaded_at = (
            item.loaded_time.ToDatetime(tzinfo=timezone.utc)
            if item.HasField("loaded_time")
            else None
        )
        records.append(
            PolicyRevisionRecord(
                rev=item.version,
                policy_hash=item.policy_hash,
                status=_policy_status_name(item.status),
                load_error=item.load_error,
                created_at=created_at,
                loaded_at=loaded_at,
                provenance=dict(item.provenance),
            )
        )
    return records


def _append_unique_policy_revisions(
    result: List[PolicyRevisionRecord],
    seen: set[int],
    page: List[PolicyRevisionRecord],
) -> None:
    for record in page:
        if record.rev in seen:
            raise RuntimeError(f"gateway repeated policy revision {record.rev}")
        seen.add(record.rev)
        result.append(record)


def _append_unique_page_items(
    result: List[T],
    seen: set[Hashable],
    items: Iterable[T],
    *,
    key: Callable[[T], Hashable],
    description: str,
) -> None:
    for item in items:
        item_key = key(item)
        if item_key in seen:
            raise RuntimeError(f"list {description}: gateway returned a duplicate item")
        seen.add(item_key)
        result.append(item)


def _profile_workspace_scope(scope: str, workspace: str):
    normalized_scope = scope.strip().lower()
    if normalized_scope == "global":
        return None
    if normalized_scope != "workspace":
        raise ValueError("profile scope must be 'global' or 'workspace'")
    _require_nonempty(workspace, "workspace name")
    return _workspace_scope(workspace)


def _provider_profile_from_yaml(profile_yaml: bytes, profile_id: str):
    try:
        document = yaml.safe_load(profile_yaml)
    except yaml.YAMLError as exc:
        raise ValueError(f"profile document is invalid YAML: {exc}") from exc
    if not isinstance(document, dict):
        raise ValueError("profile document must be a YAML mapping")
    if document.get("id") != profile_id:
        raise ValueError("profile id does not match request")
    document = dict(document)
    category = document.get("category")
    if isinstance(category, str):
        document["category"] = "PROVIDER_PROFILE_CATEGORY_" + category.upper()
    for server_field in ("source", "scope", "resource_version"):
        document.pop(server_field, None)
    profile = openshell_pb2.ProviderProfile()
    ParseDict(document, profile, ignore_unknown_fields=False)
    return profile


def _validate_profile_document(profile_yaml: bytes) -> None:
    if not profile_yaml or len(profile_yaml) > 1 << 20:
        raise ValueError("profile document must be between 1 byte and 1 MiB")
