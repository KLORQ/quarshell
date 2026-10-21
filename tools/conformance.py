#!/usr/bin/env python3
"""Run the policy conformance corpus against a candidate and optional oracle."""

from __future__ import annotations

import argparse
import json
import shlex
import subprocess
import sys
import tempfile
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[1]
DEFAULT_MANIFEST = ROOT / "plans/conformance/corpus/manifest.yaml"


def run(command: list[str], path: Path, timeout: float) -> tuple[dict | None, str]:
    try:
        completed = subprocess.run(
            [*command, str(path)],
            capture_output=True,
            text=True,
            timeout=timeout,
            check=False,
        )
    except subprocess.TimeoutExpired:
        return None, "timeout"
    except OSError as error:
        return None, f"cannot execute: {error}"
    try:
        result = json.loads(completed.stdout)
    except json.JSONDecodeError:
        return None, f"invalid JSON output: {completed.stdout[:200]!r} stderr={completed.stderr[:200]!r}"
    if not isinstance(result, dict):
        return None, "result must be a JSON object"
    return result, ""


def materialize(case: dict, base: Path, directory: Path) -> Path:
    if "file" in case:
        return base / case["file"]
    generate = case.get("generate")
    if not isinstance(generate, dict):
        raise ValueError(f"{case.get('id')}: missing file or generate")
    prefix = str(generate.get("prefix", ""))
    fill = str(generate.get("fill", "x"))
    count = int(generate.get("count", 0))
    suffix = str(generate.get("suffix", ""))
    if len(fill) != 1:
        raise ValueError(f"{case.get('id')}: fill must be one character")
    path = directory / f"{case.get('id')}.yaml"
    path.write_text(prefix + fill * count + suffix)
    return path


def compare(candidate: dict, expected: dict) -> list[str]:
    errors = []
    for key in ("accepted", "version", "error_code"):
        if key in expected and candidate.get(key) != expected[key]:
            errors.append(f"{key}: got {candidate.get(key)!r}, want {expected[key]!r}")
    return errors


def compare_eval(candidate: dict, expected: dict) -> list[str]:
    errors = []
    for key in ("filesystem", "network", "process"):
        want = expected.get(key)
        if want is None:
            continue
        got = candidate.get(key)
        if not isinstance(got, list):
            errors.append(f"{key}: candidate returned no decision list")
            continue
        if len(got) != len(want):
            errors.append(f"{key}: got {len(got)} decisions, want {len(want)}")
            continue
        for index, effect in enumerate(want):
            decision = got[index] if isinstance(got[index], dict) else {}
            value = decision.get("Effect", decision.get("effect"))
            if not isinstance(value, str) or value.lower() != str(effect).lower():
                errors.append(f"{key}[{index}]: got {value!r}, want {effect!r}")
    return errors


def run_eval_cases(manifest: dict, base: Path, timeout: float) -> int:
    cases = manifest.get("eval_cases")
    if not cases:
        return 0
    command = shlex.split(manifest.get("eval_candidate", ""))
    if not command:
        print("no eval candidate command", file=sys.stderr)
        return len(cases)
    failures = 0
    for case in cases:
        case_id = case.get("id", "<unnamed>")
        path = base / case["file"]
        expected = case.get("expect")
        if not isinstance(expected, dict):
            print(f"FAIL {case_id}: no expected result")
            failures += 1
            continue
        candidate, error = run(command, path, timeout)
        if candidate is None:
            print(f"FAIL {case_id}: candidate {error}")
            failures += 1
            continue
        errors = compare_eval(candidate, expected)
        if errors:
            print(f"FAIL {case_id}: " + "; ".join(errors))
            failures += 1
        else:
            print(f"PASS {case_id}")
    return failures


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--manifest", type=Path, default=DEFAULT_MANIFEST)
    parser.add_argument("--candidate-cmd")
    parser.add_argument("--oracle-cmd")
    parser.add_argument("--timeout", type=float, default=60.0)
    args = parser.parse_args()

    manifest = yaml.safe_load(args.manifest.read_text())
    if not isinstance(manifest, dict) or not isinstance(manifest.get("cases"), list):
        print("manifest must contain a cases list", file=sys.stderr)
        return 2
    candidate = shlex.split(args.candidate_cmd or manifest.get("candidate", ""))
    if not candidate:
        print("no candidate command", file=sys.stderr)
        return 2
    oracle = shlex.split(args.oracle_cmd) if args.oracle_cmd else None
    base = args.manifest.parent

    failures = 0
    with tempfile.TemporaryDirectory(prefix="quarshell-conformance-") as tmp:
        directory = Path(tmp)
        for case in manifest["cases"]:
            case_id = case.get("id", "<unnamed>")
            try:
                path = materialize(case, base, directory)
            except (OSError, ValueError) as error:
                print(f"FAIL {case_id}: {error}")
                failures += 1
                continue
            expected = case.get("expect")
            if oracle:
                oracle_result, oracle_error = run(oracle, path, args.timeout)
                if oracle_result is None:
                    print(f"FAIL {case_id}: oracle {oracle_error}")
                    failures += 1
                    continue
                expected = oracle_result
            if not isinstance(expected, dict):
                print(f"FAIL {case_id}: no expected result")
                failures += 1
                continue
            candidate_result, candidate_error = run(candidate, path, args.timeout)
            if candidate_result is None:
                print(f"FAIL {case_id}: candidate {candidate_error}")
                failures += 1
                continue
            errors = compare(candidate_result, expected)
            if errors:
                print(f"FAIL {case_id}: " + "; ".join(errors))
                failures += 1
            else:
                print(f"PASS {case_id}")
    eval_failures = run_eval_cases(manifest, base, args.timeout)
    total = len(manifest["cases"]) + len(manifest.get("eval_cases", []))
    failures += eval_failures
    print(f"conformance: {total - failures}/{total} passed")
    return 1 if failures else 0


if __name__ == "__main__":
    raise SystemExit(main())
