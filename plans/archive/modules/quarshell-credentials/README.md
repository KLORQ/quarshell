# quarshell-credentials

Credential storage and provider token exchange for Quarshell.

The module includes in-memory, file, Vault, Kubernetes and SPIFFE-backed credential sources, with grant and refresh primitives.

## Install

```sh
go get github.com/KLORQ/quarshell-credentials@latest
```

## Development

```sh
go test ./...
```

Requires Go 1.27 or newer. See the [Quarshell documentation](https://klorq.github.io/sandbox.dev/) for product guides and compatibility status.

## License

Apache-2.0.
