"""Tests for the native gRPC client and HTTP health probe."""

from __future__ import annotations

from concurrent.futures import ThreadPoolExecutor
from io import BytesIO
from threading import Thread

import grpc
import httpx
import pytest

from quarshell import (
    Client,
    ConnectUnsupported,
    ExecInput,
    Provider,
    ProviderRefresh,
    Sandbox,
    SandboxLogLine,
    ServiceExposure,
    WorkloadTemplate,
)
from quarshell._proto.openshell import (
    datamodel_pb2,
    openshell_pb2,
    openshell_pb2_grpc,
    sandbox_pb2,
)
from google.protobuf.timestamp_pb2 import Timestamp


class _CompatibilityService(openshell_pb2_grpc.OpenShellServicer):
    def __init__(self):
        self.exec_sandbox_id = ""
        self.exec_interactive_requests = []
        self.template_create = None
        self.template_get = None
        self.template_list = []
        self.repeat_template_token = False
        self.template_delete = None
        self.providers = {}
        self.provider_calls = []
        self.provider_profile_list_requests = []
        self.provider_profile_requests = []
        self.provider_list_requests = []
        self.repeat_provider_token = False
        self.draft_policy_requests = []
        self.draft_approvals = []
        self.draft_rejections = []
        self.provider_attachments = []
        self.refresh_requests = []
        self.rootfs_upload_path = ""
        self.rootfs_staging_request = None
        self.policy_status_requests = []
        self.policy_updates = []
        self.policy_history_requests = []
        self.native_log_requests = []
        self.native_log_pushes = []
        self.watch_requests = []
        self.sandboxes = {}
        self.get_names = []
        self.lifecycle_requests = []
        self.auth = []

    def _record(self, context):
        self.auth.append(dict(context.invocation_metadata()).get("authorization"))

    def GetGatewayInfo(self, request, context):  # noqa: N802
        self._record(context)
        return openshell_pb2.GetGatewayInfoResponse(
            status=openshell_pb2.SERVICE_STATUS_HEALTHY,
            gateway_version="0.1.4",
            compute_drivers=[openshell_pb2.ComputeDriverInfo(
                name="docker",
                capabilities=openshell_pb2.ComputeDriverCapabilities(driver_version="1.2"),
            )],
        )

    def GetCurrentUser(self, request, context):  # noqa: N802
        self._record(context)
        return openshell_pb2.GetCurrentUserResponse(
            subject="alice", roles=["user"], scopes=["sandbox:read"],
            identity_provider="oidc",
        )

    def CreateSandbox(self, request, context):  # noqa: N802
        self._record(context)
        sandbox = openshell_pb2.Sandbox(
            metadata=datamodel_pb2.ObjectMeta(
                id="id-" + request.name, name=request.name,
                workspace=request.workspace_scope.workspace, resource_version=1,
                labels=request.labels,
            ),
            spec=request.spec,
            status=openshell_pb2.SandboxStatus(phase=openshell_pb2.SANDBOX_PHASE_READY),
        )
        self.sandboxes[request.name] = sandbox
        self.create_request = request
        return openshell_pb2.SandboxResponse(sandbox=sandbox)

    def ListSandboxes(self, request, context):  # noqa: N802
        self._record(context)
        return openshell_pb2.ListSandboxesResponse(
            sandboxes=list(self.sandboxes.values())
        )

    def DeleteSandbox(self, request, context):  # noqa: N802
        self._record(context)
        self.lifecycle_requests.append(("delete", request))
        self.sandboxes.pop(request.name, None)
        return openshell_pb2.DeleteSandboxResponse(
            outcome=openshell_pb2.DELETION_OUTCOME_COMPLETED,
            sandbox_id="id-" + request.name,
        )

    def StartSandbox(self, request, context):  # noqa: N802
        self._record(context)
        self.lifecycle_requests.append(("start", request))
        sandbox = self.sandboxes[request.name]
        sandbox.status.phase = openshell_pb2.SANDBOX_PHASE_READY
        return openshell_pb2.SandboxResponse(sandbox=sandbox)

    def StopSandbox(self, request, context):  # noqa: N802
        self._record(context)
        self.lifecycle_requests.append(("stop", request))
        sandbox = self.sandboxes[request.name]
        sandbox.status.phase = openshell_pb2.SANDBOX_PHASE_STOPPED
        return openshell_pb2.SandboxResponse(sandbox=sandbox)

    def GetSandboxLogs(self, request, context):  # noqa: N802
        self.native_log_requests.append(request)
        return openshell_pb2.GetSandboxLogsResponse(
            logs=[openshell_pb2.SandboxLogLine(
                sandbox_id="id-demo", event_time=Timestamp(seconds=1_800_000_000),
                source="supervisor", level="INFO", target="stdout", message="hello",
            )],
            buffer_total=7,
        )

    def PushSandboxLogs(self, requests, context):  # noqa: N802
        self.native_log_pushes.extend(requests)
        return openshell_pb2.PushSandboxLogsResponse()

    def WatchSandbox(self, request, context):  # noqa: N802
        self.watch_requests.append(request)
        yield openshell_pb2.SandboxStreamEvent(
            log=openshell_pb2.SandboxLogLine(
                sandbox_id="id-demo", source="supervisor", level="INFO", message="live",
            ),
            cursor="cursor-7",
        )

    def BeginRootfsTarStaging(self, request, context):  # noqa: N802
        self.rootfs_staging_request = request
        return openshell_pb2.BeginRootfsTarStagingResponse(
            staging_token="opaque-token",
            upload_path=self.rootfs_upload_path,
            max_bytes=1024,
        )

    def GetSandbox(self, request, context):  # noqa: N802
        self._record(context)
        self.get_names.append(request.name)
        if request.name in self.sandboxes:
            return openshell_pb2.SandboxResponse(sandbox=self.sandboxes[request.name])
        return openshell_pb2.SandboxResponse(
            sandbox=openshell_pb2.Sandbox(
                metadata={
                    "id": "id-" + request.name,
                    "name": request.name,
                    "workspace": request.workspace_scope.workspace,
                    "resource_version": 42,
                }
            )
        )

    def GetSandboxPolicyStatus(self, request, context):  # noqa: N802
        self.policy_status_requests.append(request)
        version = request.version or 3
        policy = sandbox_pb2.SandboxPolicy(
            version=1,
            network_policies={
                "api": sandbox_pb2.NetworkPolicyRule(
                    name="api",
                    endpoints=[
                        sandbox_pb2.NetworkEndpoint(
                            host="api.example.test",
                            port=443,
                            tls=sandbox_pb2.NETWORK_TLS_MODE_SKIP,
                        )
                    ],
                )
            },
        )
        return openshell_pb2.GetSandboxPolicyStatusResponse(
            revision=openshell_pb2.SandboxPolicyRevision(
                version=version,
                status=openshell_pb2.POLICY_STATUS_LOADED,
                policy=policy,
                created_time=Timestamp(seconds=1_800_000_000, nanos=123_000_000),
            )
        )

    def UpdateConfig(self, request, context):  # noqa: N802
        self.policy_updates.append(request)
        return openshell_pb2.UpdateConfigResponse(version=3)

    def ListSandboxPolicies(self, request, context):  # noqa: N802
        self.policy_history_requests.append(request)
        if request.page_token:
            return openshell_pb2.ListSandboxPoliciesResponse()
        return openshell_pb2.ListSandboxPoliciesResponse(
            revisions=[
                openshell_pb2.SandboxPolicyRevision(
                    version=3,
                    status=openshell_pb2.POLICY_STATUS_LOADED,
                    created_time=Timestamp(seconds=1_800_000_000, nanos=123_000_000),
                )
            ]
        )

    def ExecSandbox(self, request, context):  # noqa: N802
        self.exec_sandbox_id = request.sandbox
        yield openshell_pb2.ExecSandboxEvent(
            stdout=openshell_pb2.ExecSandboxStdout(data=b"hi\n")
        )
        yield openshell_pb2.ExecSandboxEvent(
            exit=openshell_pb2.ExecSandboxExit(exit_code=0)
        )

    def ExecSandboxInteractive(self, request_iterator, context):  # noqa: N802
        self.exec_interactive_requests = list(request_iterator)
        yield openshell_pb2.ExecSandboxEvent(
            stdout=openshell_pb2.ExecSandboxStdout(data=b"ready")
        )
        yield openshell_pb2.ExecSandboxEvent(
            stderr=openshell_pb2.ExecSandboxStderr(data=b"warning")
        )
        yield openshell_pb2.ExecSandboxEvent(
            exit=openshell_pb2.ExecSandboxExit(exit_code=7)
        )

    def CreateSandboxTemplate(self, request, context):  # noqa: N802
        self.template_create = request
        return openshell_pb2.SandboxTemplateResponse(template=request.template)

    def GetSandboxTemplate(self, request, context):  # noqa: N802
        self.template_get = request
        return openshell_pb2.SandboxTemplateResponse(
            template=_template(request.name, request.workspace_scope.workspace)
        )

    def ListSandboxTemplates(self, request, context):  # noqa: N802
        self.template_list.append(request)
        if request.page_token == "" or self.repeat_template_token:
            return openshell_pb2.ListSandboxTemplatesResponse(
                templates=[_template("one", request.workspace_scope.workspace)],
                next_page_token="page-2",
            )
        return openshell_pb2.ListSandboxTemplatesResponse(
            templates=[_template("two", request.workspace_scope.workspace)]
        )

    def DeleteSandboxTemplate(self, request, context):  # noqa: N802
        self.template_delete = request
        return openshell_pb2.DeleteSandboxTemplateResponse(
            outcome=openshell_pb2.DELETION_OUTCOME_COMPLETED
        )

    def GetProvider(self, request, context):  # noqa: N802
        provider = self.providers.get(request.name)
        if provider is None:
            context.set_code(grpc.StatusCode.NOT_FOUND)
            context.set_details("provider not found")
            return openshell_pb2.ProviderResponse()
        return openshell_pb2.ProviderResponse(provider=provider)

    def CreateProvider(self, request, context):  # noqa: N802
        self.provider_calls.append(("create", request))
        self.providers[request.provider.metadata.name] = request.provider
        return openshell_pb2.ProviderResponse(provider=request.provider)

    def UpdateProvider(self, request, context):  # noqa: N802
        self.provider_calls.append(("update", request))
        self.providers[request.provider.metadata.name] = request.provider
        return openshell_pb2.ProviderResponse(provider=request.provider)

    def ListProviders(self, request, context):  # noqa: N802
        self.provider_list_requests.append(request)
        if request.page_token == "" or self.repeat_provider_token:
            return openshell_pb2.ListProvidersResponse(
                providers=list(self.providers.values()),
                next_page_token="providers-next",
            )
        return openshell_pb2.ListProvidersResponse()

    def ListProviderProfiles(self, request, context):  # noqa: N802
        self.provider_profile_list_requests.append(request)
        return openshell_pb2.ListProviderProfilesResponse(
            profiles=[
                openshell_pb2.ProviderProfile(
                    id="openai",
                    category=openshell_pb2.PROVIDER_PROFILE_CATEGORY_INFERENCE,
                    source="builtin",
                    scope="platform",
                )
            ]
        )

    def GetDraftPolicy(self, request, context):  # noqa: N802
        self.draft_policy_requests.append(request)
        if request.sandbox != "sandbox-a":
            return openshell_pb2.GetDraftPolicyResponse()
        first = openshell_pb2.PolicyChunk(
            id="p1", status="pending", rule_name="api.example.test",
            rationale="needed by app", security_notes="security", review_token="review-p1",
            proposed_rule=sandbox_pb2.NetworkPolicyRule(
                endpoints=[sandbox_pb2.NetworkEndpoint(host="api.example.test")]
            ),
        )
        first.created_time.seconds = 1_800_000_000
        first.created_time.nanos = 123_000_000
        second = openshell_pb2.PolicyChunk(
            id="p2", status="pending", security_notes="review carefully",
            review_token="review-p2",
        )
        return openshell_pb2.GetDraftPolicyResponse(chunks=[first, second])

    def ApproveDraftChunk(self, request, context):  # noqa: N802
        self.draft_approvals.append(request)
        return openshell_pb2.ApproveDraftChunkResponse(policy_version=2)

    def RejectDraftChunk(self, request, context):  # noqa: N802
        self.draft_rejections.append(request)
        return openshell_pb2.RejectDraftChunkResponse()

    def GetProviderProfile(self, request, context):  # noqa: N802
        self.provider_profile_requests.append(("get", request))
        return openshell_pb2.ProviderProfileResponse(
            profile=openshell_pb2.ProviderProfile(
                id=request.id,
                category=openshell_pb2.PROVIDER_PROFILE_CATEGORY_INFERENCE,
                source="user",
                resource_version=8,
            )
        )

    def ImportProviderProfiles(self, request, context):  # noqa: N802
        self.provider_profile_requests.append(("create", request))
        return openshell_pb2.ImportProviderProfilesResponse(imported=True)

    def UpdateProviderProfiles(self, request, context):  # noqa: N802
        self.provider_profile_requests.append(("update", request))
        return openshell_pb2.UpdateProviderProfilesResponse(updated=True)

    def DeleteProviderProfile(self, request, context):  # noqa: N802
        self.provider_profile_requests.append(("delete", request))
        return openshell_pb2.DeleteProviderProfileResponse(
            outcome=openshell_pb2.DELETION_OUTCOME_COMPLETED
        )

    def DeleteProvider(self, request, context):  # noqa: N802
        self.provider_calls.append(("delete", request))
        self.providers.pop(request.name, None)
        return openshell_pb2.DeleteProviderResponse(
            outcome=openshell_pb2.DELETION_OUTCOME_COMPLETED
        )

    def AttachSandboxProvider(self, request, context):  # noqa: N802
        self.provider_attachments.append(("attach", request))
        return openshell_pb2.AttachSandboxProviderResponse(attached=True)

    def DetachSandboxProvider(self, request, context):  # noqa: N802
        self.provider_attachments.append(("detach", request))
        return openshell_pb2.DetachSandboxProviderResponse(detached=True)

    def ListSandboxProviders(self, request, context):  # noqa: N802
        return openshell_pb2.ListSandboxProvidersResponse(
            providers=list(self.providers.values())
        )

    def ConfigureProviderRefresh(self, request, context):  # noqa: N802
        self.refresh_requests.append(("configure", request))
        return openshell_pb2.ConfigureProviderRefreshResponse()

    def RotateProviderCredential(self, request, context):  # noqa: N802
        self.refresh_requests.append(("rotate", request))
        return openshell_pb2.RotateProviderCredentialResponse()

    def DeleteProviderRefresh(self, request, context):  # noqa: N802
        self.refresh_requests.append(("delete", request))
        return openshell_pb2.DeleteProviderRefreshResponse(
            outcome=openshell_pb2.DELETION_OUTCOME_COMPLETED
        )


def _template(name, workspace):
    return openshell_pb2.SandboxWorkloadTemplate(
        metadata={"name": name, "workspace": workspace, "labels": {"team": "sdk"}},
        spec={
            "workload": {
                "image": "python:3.12-slim",
                "environment": {"MODE": "test"},
                "resources": {
                    "cpu": "500m",
                    "memory": "1Gi",
                    "gpu": {"count": 2},
                },
            },
            "driver_config": {"docker": {"network": "isolated"}},
        },
    )


@pytest.fixture()
def gateway():
    control = _CompatibilityService()
    control.compatibility_service = control
    server = grpc.server(ThreadPoolExecutor(max_workers=4))
    openshell_pb2_grpc.add_OpenShellServicer_to_server(control, server)
    port = server.add_insecure_port("127.0.0.1:0")
    server.start()
    yield "http://127.0.0.1:%s" % port, control
    server.stop(0).wait()


def test_crud_and_exec(gateway):
    gateway_url, control = gateway
    with Client(gateway_url, token=" sdk-test-token ", workspace="team-a") as client:
        created = client.create(Sandbox(
            name="demo", image="python:3.12-slim", command=["python", "-V"],
            cpu_millis=250, memory_bytes=1024, tty=True,
            environment={"MODE": "test"},
            service_exposures=[ServiceExposure(name="web", target_port=8080)],
        ))
        assert created.id == "id-demo" and created.status == "ready"
        assert control.compatibility_service.create_request.workspace_scope.workspace == "team-a"
        assert control.compatibility_service.create_request.request_id
        assert control.compatibility_service.create_request.spec.template.image == "python:3.12-slim"
        assert control.compatibility_service.create_request.spec.command == ["python", "-V"]
        assert control.compatibility_service.create_request.spec.environment["MODE"] == "test"
        assert control.compatibility_service.create_request.service_exposures[0].service == "web"
        assert len(client.list()) == 1
        assert client.get("demo").name == "demo"
        result = client.exec("demo", "echo", "hi")
        assert result.exit_code == 0 and result.output == "hi\n"
        assert control.compatibility_service.exec_sandbox_id == "id-demo"
        with pytest.raises(ConnectUnsupported):
            client.connect("demo")
        client.delete("demo")
    assert control.compatibility_service.auth
    assert all(value == "Bearer sdk-test-token" for value in control.compatibility_service.auth)


def test_info_and_whoami_use_native_openshell(gateway):
    gateway_url, control = gateway
    with Client(gateway_url, workspace="team-a") as client:
        info = client.info()
        viewer = client.whoami()
    assert info["gateway_version"] == "0.1.4"
    assert info["compute_drivers"][0]["name"] == "docker"
    assert viewer["subject"] == "alice"
    assert viewer["roles"] == ["user"]
    assert viewer["identity_provider"] == "oidc"


def test_rootfs_tar_staging_writes_to_openshell_local_path(gateway, tmp_path):
    gateway_url, control = gateway
    upload_path = tmp_path / "rootfs.tar"
    control.compatibility_service.rootfs_upload_path = str(upload_path)
    with Client(gateway_url, workspace="team-a") as client:
        token = client.upload_rootfs_tar("rootfs.tar", 9, BytesIO(b"tar-bytes"))
    assert token == "opaque-token"
    assert upload_path.read_bytes() == b"tar-bytes"
    request = control.compatibility_service.rootfs_staging_request
    assert request.file_name == "rootfs.tar"
    assert request.size_bytes == 9
    assert request.workspace_scope.workspace == "team-a"


def test_interactive_exec_streams_input_resize_and_output(gateway):
    gateway_url, control = gateway
    with Client(gateway_url, workspace="team-a") as client:
        events = list(
            client.exec_interactive(
                "demo",
                "sh",
                "-i",
                inputs=[ExecInput.send(b"echo hi\n"), ExecInput.resize(120, 40)],
                tty=True,
                workdir="/workspace",
                environment={"MODE": "test"},
            )
        )

    assert [(event.stdout, event.stderr, event.exit_code) for event in events] == [
        (b"ready", b"", None),
        (b"", b"warning", None),
        (b"", b"", 7),
    ]
    requests = control.compatibility_service.exec_interactive_requests
    assert [request.WhichOneof("payload") for request in requests] == [
        "start",
        "stdin",
        "resize",
    ]
    assert requests[0].start.sandbox == "id-demo"
    assert requests[0].start.workspace_scope.workspace == "team-a"
    assert requests[0].start.tty
    assert requests[0].start.workdir == "/workspace"
    assert requests[0].start.environment["MODE"] == "test"
    assert requests[1].stdin == b"echo hi\n"
    assert (requests[2].resize.cols, requests[2].resize.rows) == (120, 40)


@pytest.mark.parametrize(
    "kwargs",
    [
        {"stdin": b"data", "cols": 80, "rows": 24},
        {"cols": 80},
        {"cols": 0, "rows": 24},
    ],
)
def test_interactive_exec_input_rejects_invalid_frames(kwargs):
    with pytest.raises(ValueError):
        ExecInput(**kwargs)


def test_start_stop_use_workspace_and_resource_version(gateway):
    gateway_url, control = gateway
    with Client(gateway_url, workspace="team-a") as client:
        client.create(Sandbox(name="demo", image="python:3.12-slim"))
        started = client.start("demo")
        stopped = client.stop("demo")
    assert started.status == "ready"
    assert stopped.status == "stopped"
    assert [action for action, _ in control.compatibility_service.lifecycle_requests] == ["start", "stop"]
    for action, request in control.compatibility_service.lifecycle_requests:
        assert request.workspace_scope.workspace == "team-a"
        assert request.name == "demo"
        assert request.request_id


def test_native_sandbox_log_read_and_push(gateway):
    gateway_url, control = gateway
    service = control.compatibility_service
    service.sandboxes["demo"] = openshell_pb2.Sandbox(
        metadata=datamodel_pb2.ObjectMeta(
            id="id-demo", name="demo", workspace="team-a", resource_version=1,
        )
    )
    with Client(gateway_url, workspace="team-a") as client:
        page = client.get_sandbox_logs("demo", limit=5, since_unix_ms=1_800_000_000_000, source="supervisor", level="INFO")
        assert page.lines[0].message == "hello" and page.cursor == 7
        assert client.append_sandbox_logs("demo", [SandboxLogLine(
            timestamp_unix_ms=1_800_000_001_000, source="cli", level="INFO", message="sent",
        )]) is None
    request = service.native_log_requests[-1]
    assert request.workspace_scope.workspace == "team-a"
    assert request.sandbox == "demo" and request.lines == 5
    assert request.sources == ["supervisor"] and request.min_level == "INFO"
    assert request.since_time.ToMilliseconds() == 1_800_000_000_000
    pushed = service.native_log_pushes[0]
    assert pushed.sandbox_id == "id-demo"
    assert pushed.logs[0].message == "sent" and pushed.logs[0].source == "cli"


def test_native_sandbox_watch_uses_resume_cursor(gateway):
    gateway_url, control = gateway
    with Client(gateway_url, workspace="team-a") as client:
        events = list(client.watch_sandbox_logs(
            "demo", initial_limit=12, since_unix_ms=1_800_000_000_000,
            source="supervisor", level="WARN", resume_after_cursor="cursor-6",
        ))
    assert len(events) == 1 and events[0].log.message == "live"
    assert events[0].cursor == "cursor-7"
    request = control.compatibility_service.watch_requests[-1]
    assert request.workspace_scope.workspace == "team-a"
    assert request.follow_logs and not request.follow_status and not request.follow_events
    assert request.log_tail_lines == 12 and request.resume_after_cursor == "cursor-6"
    assert request.log_sources == ["supervisor"] and request.log_min_level == "WARN"
    assert request.since_time.ToMilliseconds() == 1_800_000_000_000


def test_workspace_template_crud_and_cursor_pagination(gateway):
    gateway_url, control = gateway
    with Client(gateway_url, workspace="team-a") as client:
        created = client.create_template(
            WorkloadTemplate(
                name="worker",
                image="python:3.12-slim",
                environment={"MODE": "test"},
                cpu="500m",
                memory="1Gi",
                gpu=True,
                gpu_count=2,
                labels={"team": "sdk"},
                driver_config_json='{"docker":{"network":"isolated"}}',
            )
        )
        request = control.compatibility_service.template_create
        assert request.workspace_scope.workspace == "team-a"
        assert request.template.metadata.workspace == "team-a"
        assert (created.name, created.workspace, created.gpu_count) == (
            "worker",
            "team-a",
            2,
        )
        assert request.template.spec.workload.resources.cpu == "500m"
        assert request.template.spec.workload.resources.gpu.count == 2
        assert request.template.spec.driver_config["docker"]["network"] == "isolated"

        fetched = client.get_template("worker")
        assert fetched.name == "worker" and fetched.workspace == "team-a"
        assert control.compatibility_service.template_get.workspace_scope.workspace == "team-a"

        listed = client.list_templates()
        assert [item.name for item in listed] == ["one", "two"]
        assert [request.page_token for request in control.compatibility_service.template_list] == [
            "",
            "page-2",
        ]
        assert all(
            request.page_size == 1000 for request in control.compatibility_service.template_list
        )

        assert client.delete_template("worker") is True
        assert control.compatibility_service.template_delete.allow_missing
        assert control.compatibility_service.template_delete.workspace_scope.workspace == "team-a"


def test_template_validation_and_repeated_pagination_token(gateway):
    gateway_url, control = gateway
    with Client(gateway_url) as client:
        with pytest.raises(ValueError, match="GPU count"):
            client.create_template(
                WorkloadTemplate(name="bad", image="busybox", gpu_count=1025)
            )
        with pytest.raises(ValueError, match="driver config"):
            client.create_template(
                WorkloadTemplate(
                    name="bad", image="busybox", driver_config_json="[]"
                )
            )
        control.compatibility_service.repeat_template_token = True
        with pytest.raises(RuntimeError, match="repeated a pagination token"):
            client.list_templates()


def test_provider_crud_hides_secret_values_and_uses_workspace(gateway):
    gateway_url, control = gateway
    with Client(gateway_url, workspace="team-a") as client:
        provider = Provider(
            name="model-api",
            type="openai",
            env_vars=["API_KEY"],
            credentials={"API_KEY": "test-secret"},
            credential_expires_at_ms={"API_KEY": 1_800_000_000_123},
            runtime_credentials=True,
            config={"base_url": "https://api.example.test"},
        )
        client.put_provider(provider)
        action, create = control.compatibility_service.provider_calls[-1]
        assert action == "create"
        assert create.workspace_scope.workspace == "team-a"
        assert create.provider.credentials["API_KEY"] == "test-secret"
        assert create.provider.metadata.annotations[
            "cautem.io/runtime-credentials"
        ] == "true"

        stored = datamodel_pb2.Provider()
        stored.CopyFrom(create.provider)
        stored.metadata.id = "provider-id"
        stored.metadata.resource_version = 7
        control.compatibility_service.providers["model-api"] = stored
        read = client.get_provider("model-api")
        assert read.env_vars == ["API_KEY"]
        assert read.credentials == {}
        assert read.config == {"base_url": "https://api.example.test"}
        assert read.credential_expires_at_ms == {"API_KEY": 1_800_000_000_123}
        assert read.runtime_credentials
        assert "test-secret" not in repr(read)
        assert [item.name for item in client.list_providers()] == ["model-api"]
        assert [
            request.page_token
            for request in control.compatibility_service.provider_list_requests
        ] == ["", "providers-next"]
        assert [item.name for item in client.list_sandbox_providers("sandbox-a")] == [
            "model-api"
        ]

        updated = Provider(
            name="model-api",
            type="openai",
            env_vars=["API_KEY"],
            credentials={"API_KEY": "rotated-secret"},
            config={"base_url": "https://new.example.test"},
        )
        client.put_provider(updated)
        action, update = control.compatibility_service.provider_calls[-1]
        assert action == "update"
        assert update.provider.metadata.id == "provider-id"
        assert update.provider.metadata.resource_version == 7

        client.delete_provider("model-api")
        assert control.compatibility_service.provider_calls[-1][0] == "delete"


def test_provider_credentials_attachment_and_refresh_contracts(gateway):
    gateway_url, control = gateway
    with Client(gateway_url, workspace="team-a") as client:
        client.update_provider_credentials("model-api", {"API_KEY": "secret-patch"})
        action, patch = control.compatibility_service.provider_calls[-1]
        assert action == "update"
        assert patch.workspace_scope.workspace == "team-a"
        assert patch.provider.metadata.name == "model-api"
        assert patch.provider.credentials["API_KEY"] == "secret-patch"
        client.put_provider(
            Provider(
                name="model-api",
                type="openai",
                env_vars=["EXISTING_KEY", "API_KEY"],
                credentials={"API_KEY": "partial-secret"},
            )
        )
        action, patch = control.compatibility_service.provider_calls[-1]
        assert action == "update"
        assert patch.provider.credentials["API_KEY"] == "partial-secret"
        assert patch.provider.credentials["EXISTING_KEY"] == ""

        client.attach_provider("sandbox-a", "model-api")
        client.detach_provider("sandbox-a", "model-api")
        assert [action for action, _ in control.compatibility_service.provider_attachments] == [
            "attach",
            "detach",
        ]
        for _, request in control.compatibility_service.provider_attachments:
            assert request.workspace_scope.workspace == "team-a"
            assert request.expected_resource_version == 42

        client.configure_provider_refresh(
            "model-api",
            "API_KEY",
            "oauth2_client_credentials",
            {"client_id": "id", "client_secret": "hidden"},
            expires_at_ms=1_800_000_000_123,
        )
        refresh = ProviderRefresh(
            strategy="oauth2-client-credentials",
            material={"client_secret": "hidden"},
        )
        assert "hidden" not in repr(refresh)
        client.rotate_provider_refresh("model-api", "API_KEY")
        client.delete_provider_refresh("model-api", "API_KEY")
        configure = control.compatibility_service.refresh_requests[0][1]
        assert configure.strategy == (
            openshell_pb2.PROVIDER_CREDENTIAL_REFRESH_STRATEGY_OAUTH2_CLIENT_CREDENTIALS
        )
        assert configure.material["client_secret"] == "hidden"
        assert configure.expiration_time.ToMilliseconds() == 1_800_000_000_123
        assert [action for action, _ in control.compatibility_service.refresh_requests] == [
            "configure",
            "rotate",
            "delete",
        ]


def test_invalid_provider_refresh_is_rejected_before_provider_write(gateway):
    gateway_url, control = gateway
    with Client(gateway_url, workspace="team-a") as client:
        with pytest.raises(ValueError, match="unsupported refresh strategy"):
            client.put_provider(
                Provider(
                    name="bad",
                    type="openai",
                    env_vars=["API_KEY"],
                    credentials={"API_KEY": "secret"},
                    refresh={"API_KEY": ProviderRefresh(strategy="unknown")},
                )
            )
    assert control.compatibility_service.provider_calls == []


def test_provider_list_rejects_repeated_cursor(gateway):
    gateway_url, control = gateway
    control.compatibility_service.repeat_provider_token = True
    with Client(gateway_url, workspace="team-a") as client:
        with pytest.raises(RuntimeError, match="repeated a pagination token"):
            client.list_providers()


def test_policy_reads_revisions_and_optimistic_updates(gateway):
    gateway_url, control = gateway
    with Client(gateway_url, workspace="team-a") as client:
        assert b"tls: skip" in client.get_global_policy()
        policy_doc = b"version: 1\nnetwork_policies:\n  api:\n    name: api\n    endpoints:\n    - host: api.example.test\n      port: 443\n      tls: skip\n"
        client.put_global_policy(policy_doc)
        update = control.compatibility_service.policy_updates[-1]
        assert getattr(update, "global")
        assert update.policy.network_policies["api"].endpoints[0].tls == sandbox_pb2.NETWORK_TLS_MODE_SKIP

        assert b"tls: skip" in client.get_sandbox_policy("sandbox-a", "effective")
        assert b"tls: skip" in client.get_sandbox_policy("sandbox-a", "BASE")
        assert control.compatibility_service.policy_status_requests[-1].workspace_scope.workspace == "team-a"
        with pytest.raises(ValueError, match="policy view"):
            client.get_sandbox_policy("sandbox-a", "invalid")

        effective, stripped = client.put_sandbox_policy(
            "sandbox-a", policy_doc
        )
        assert b"tls: skip" in effective and stripped == 0
        update = control.compatibility_service.policy_updates[-1]
        assert update.expected_resource_version == 42
        assert update.sandbox == "sandbox-a" and update.policy.network_policies["api"].name == "api"

        revisions = client.list_policy_revisions("sandbox-a")
        assert revisions[0].rev == 3
        assert revisions[0].updated_at.microsecond == 123000
        assert revisions[0].status == "loaded"
        assert client.get_policy_revision("sandbox-a", 3).find("api.example.test") >= 0
        with pytest.raises(ValueError, match="positive"):
            client.get_policy_revision("sandbox-a", 0)


def test_policy_draft_methods_use_native_open_shell_contract(gateway):
    gateway_url, control = gateway
    with Client(gateway_url, workspace="team-a") as client:
        draft = client.get_draft_policy("sandbox-a", "pending")
        assert [item.id for item in draft.chunks] == ["p1", "p2"]
        assert draft.chunks[0].review_token == "review-p1"
        assert draft.chunks[0].security_notes == "security"
        assert [request.sandbox for request in control.compatibility_service.draft_policy_requests] == ["sandbox-a"]
        assert control.compatibility_service.draft_policy_requests[-1].workspace_scope.workspace == "team-a"
        assert client.approve_draft_chunk("sandbox-a", "p1", "review-p1").policy_version == 2
        assert control.compatibility_service.draft_approvals[-1].review_token == "review-p1"
        client.reject_draft_chunk("sandbox-a", "p2", "too broad")
        assert control.compatibility_service.draft_rejections[-1].reason == "too broad"
        with pytest.raises(ValueError, match="draft review token"):
            client.approve_draft_chunk("sandbox-a", "p1", "")


def test_provider_profile_scopes_crud_and_resource_version(gateway):
    gateway_url, control = gateway
    with Client(gateway_url, workspace="team-a") as client:
        profiles = client.list_profiles()
        assert profiles[0].id == "openai"
        assert not control.compatibility_service.provider_profile_list_requests[-1].HasField("workspace_scope")

        doc = client.get_profile("openai", scope="workspace", workspace="team-a")
        assert b"id: openai" in doc.yaml and b"category: inference" in doc.yaml
        assert (doc.source, doc.resource_version) == ("user", "8")
        assert control.compatibility_service.provider_profile_requests[-1][1].workspace_scope.workspace == "team-a"

        client.create_profile(
            "openai", b"id: openai\ncategory: inference\n", scope="workspace", workspace="team-a"
        )
        create = control.compatibility_service.provider_profile_requests[-1][1]
        assert create.workspace_scope.workspace == "team-a"
        assert create.profiles[0].profile.id == "openai"
        assert create.profiles[0].profile.category == openshell_pb2.PROVIDER_PROFILE_CATEGORY_INFERENCE

        client.put_profile(
            "openai",
            b"id: openai\ncategory: inference\nresource_version: 8\n",
            '"8"',
            scope="workspace",
            workspace="team-a",
        )
        update = control.compatibility_service.provider_profile_requests[-1][1]
        assert update.expected_resource_version == 8
        assert update.workspace_scope.workspace == "team-a"

        client.delete_profile("openai", scope="workspace", workspace="team-a")
        assert control.compatibility_service.provider_profile_requests[-1][0] == "delete"


def test_profile_mutations_validate_before_rpc(gateway):
    gateway_url, control = gateway
    with Client(gateway_url) as client:
        with pytest.raises(ValueError, match="between 1 byte"):
            client.create_profile("openai", b"")
        with pytest.raises(ValueError, match="1 MiB"):
            client.create_profile("openai", b"x" * (1 << 20 | 1))
        with pytest.raises(ValueError, match="positive resource version"):
            client.put_profile("openai", b"id: openai", "0")
    assert control.compatibility_service.provider_profile_requests == []


def test_token_defaults_to_gateway_environment(gateway, monkeypatch):
    gateway_url, control = gateway
    monkeypatch.setenv("CAUTEM_GATEWAY_TOKEN", "env-token")
    with Client(gateway_url) as client:
        assert client.list() == []
    assert control.compatibility_service.auth == ["Bearer env-token"]


def test_sandbox_name_is_sent_as_rpc_data(gateway):
    gateway_url, control = gateway
    with Client(gateway_url) as client:
        assert client.get("demo?admin=true").name == "demo?admin=true"
    assert control.compatibility_service.get_names == ["demo?admin=true"]


def test_health_probe_stays_http():
    seen = []

    def respond(request):
        seen.append(request.url.path)
        return httpx.Response(200, json={"ok": True}, request=request)

    client = Client("http://127.0.0.1:1")
    client._http.close()
    client._http = httpx.Client(
        base_url="http://127.0.0.1:1", transport=httpx.MockTransport(respond)
    )
    try:
        assert client.healthz() == {"ok": True}
    finally:
        client.close()
    assert seen == ["/healthz"]
