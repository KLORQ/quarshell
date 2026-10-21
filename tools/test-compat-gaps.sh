#!/bin/sh
set -eu

root="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
report="$root/quarshell-gateway/internal/httpapi/testdata/openshell_compatibility_gaps.json"
inventory="$root/quarshell-gateway/internal/httpapi/testdata/openshell_rpc_inventory.json"
matrix="$root/quarshell-gateway/internal/httpapi/testdata/openshell_compatibility_matrix.json"
fixture="$(mktemp /tmp/cautem-compat-gaps.XXXXXX)"
inventory_fixture="$(mktemp /tmp/cautem-compat-rpc.XXXXXX)"
matrix_fixture="$(mktemp /tmp/cautem-compat-matrix.XXXXXX)"
trap 'rm -f "$fixture" "$inventory_fixture" "$matrix_fixture"' EXIT HUP INT TERM

expect_rejected() {
  label="$1"
  if sh "$root/tools/check-compat-gaps.sh" "$fixture" "${2:-$inventory}" >/dev/null 2>&1; then
    echo "compat gap check accepted $label" >&2
    exit 1
  fi
}

jq 'del(.gaps[] | select(.id == "P0-CONFIG-CORPUS"))' "$report" > "$fixture"
expect_rejected 'a deleted open requirement'

jq 'del(.gaps[] | select(.id == "P3-VM-PRODUCTION-GATE"))' "$report" > "$fixture"
expect_rejected 'a deleted MicroVM production gate'

jq '.gaps[1].id = .gaps[0].id' "$report" > "$fixture"
expect_rejected 'a duplicate requirement ID'

jq '.upstream_commit = "0000000000000000000000000000000000000000"' "$report" > "$fixture"
expect_rejected 'a different upstream commit'

jq '.gaps[0].evidence = ""' "$report" > "$fixture"
expect_rejected 'empty evidence'

cp "$report" "$fixture"
jq 'del(.services[0].methods[0])' "$inventory" > "$inventory_fixture"
expect_rejected 'a deleted public RPC method' "$inventory_fixture"

jq '.features[0].evidence_status = "invented"' "$matrix" > "$matrix_fixture"
if sh "$root/tools/check-compat-gaps.sh" "$report" "$inventory" "$matrix_fixture" >/dev/null 2>&1; then
  echo 'compat gap check accepted an unknown feature evidence status' >&2
  exit 1
fi

jq '.features[0].test_refs[0] = "missing/test/file.go"' "$matrix" > "$matrix_fixture"
if sh "$root/tools/check-compat-gaps.sh" "$report" "$inventory" "$matrix_fixture" >/dev/null 2>&1; then
  echo 'compat gap check accepted a missing test reference' >&2
  exit 1
fi

echo 'compat gap negative checks: PASS'
