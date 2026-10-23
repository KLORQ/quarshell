# quarshell-isolation

Linux process and resource isolation primitives for Quarshell workers.

The module provides platform-aware admission, cgroup and seccomp support. Unsupported platforms fail closed.

## Install

```sh
go get github.com/KLORQ/quarshell-isolation@latest
```

## Development

```sh
go test ./...
```

Requires Go 1.27 or newer. See the [Quarshell documentation](https://klorq.github.io/sandbox.dev/) for product guides and compatibility status.

## License

Apache-2.0.
