import os
import time

import pytest

from quarshell import Client


def test_python_sdk_against_live_gateway():
    base_url = os.getenv("QUARSHELL_GATEWAY_URL")
    grpc_endpoint = os.getenv("QUARSHELL_GATEWAY_GRPC_ENDPOINT")
    token = os.getenv("QUARSHELL_GATEWAY_TOKEN")
    if not base_url or not grpc_endpoint or not token:
        pytest.skip("live Gateway URL, gRPC endpoint and token are required")

    name = f"python-sdk-{time.time_ns()}"
    with Client(base_url, token=token, grpc_endpoint=grpc_endpoint) as client:
        assert client.healthz()["status"] == "ok"
        assert client.info()["gateway_version"] == "0.0.0-dev"
        created = client.create_workspace(name, labels={"source": "python-sdk-e2e"})
        assert created.name == name
        assert client.get_workspace(name).labels["source"] == "python-sdk-e2e"
        assert any(item.name == name for item in client.list_workspaces())
        client.delete_workspace(name)
