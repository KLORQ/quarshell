# quarshell-spi

Stable service-provider interfaces for Quarshell compute, credential, isolation and extension modules.

Implementations should depend on these contracts rather than importing Gateway internals.

## Install

```sh
go get github.com/KLORQ/quarshell-spi@latest
```

## Development

```sh
go test ./...
```

Requires Go 1.27 or newer. See the [Quarshell documentation](https://klorq.github.io/sandbox.dev/) for product guides and compatibility status.

## License

Apache-2.0.
