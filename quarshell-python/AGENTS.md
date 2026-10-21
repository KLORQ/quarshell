# Agent instructions

## Scope

This repository contains the Python SDK for the cautem gateway control plane.

## Working rules

- Keep Python method names, request shapes, and error behavior aligned with the
  gateway API and the Go SDK where the contracts match.
- Treat credentials and returned sandbox data as sensitive; do not add secret
  values to logs or exception messages.
- Update `pyproject.toml`, README examples, and changelog when public package
  behavior or supported Python versions change.

## Checks

- Install the `dev` extra and run `python -m pytest` from this checkout.
- Check package build and import behavior when changing packaging or generated
  protocol code.
