# Development

Quarshell uses independent Git repositories and Go 1.27 modules. A local
workspace can place the checkouts next to each other; the root itself does not
need a Git repository. Run Git commands inside the module you are changing.

## Go workspace

Use a `go.work` file to link sibling modules. Keep local replacements in that
file, not in published `go.mod` files. Replacements for workspace modules must
include the dependency version:

```bash
go work edit -replace=github.com/KLORQ/quarshell-core@v0.4.2=./quarshell-core
git -C quarshell-cli status
```

Match replacement versions to the checked-out manifests. Release checks must
also build without a workspace once the dependency versions are published.

The workspace `Taskfile.yml` runs shared checks. Use `task workspace:sync` after adding or renaming a Go checkout, `task check` for the Go modules, and `task compat:pin-check`, `task compat:proto`, and `task compat:inventory` for the pinned OpenShell contract. Run module tests from their own checkout before publishing.

The proto check uses an exact OpenShell source checkout. Prepare it with `bash tools/fetch-openshell-upstream.sh`; the script checks an existing checkout against `tools/upstream/UPSTREAM_SHA` and fetches that commit when absent.

## Releases

Modules use semantic version tags. Alpha, beta, and release candidate tags are prereleases. Nightly CLI builds are manual and are not part of the current public Releases page. Each module has its own release. When a public module path or API changes, release dependencies first, update consumer requirements, and test a clean build with `GOWORK=off` and no local replacements. Do not rewrite published tags.

All Go modules are pre-stable and stay in the `0.x` series. New module paths begin at `v0.1.0`; a path with existing public tags uses its next available immutable `0.x` version. Do not claim API stability or rewrite a published tag. Publish dependencies first, update consumers and workflow refs, then pass the `GOWORK=off` consumer gate before tagging. Each module has its own sequence; use the versions recorded in its `go.mod` and `tools/modules.lock`. Python remains unpublished on PyPI.

The logging checkout is locally named `quarshell-core/slogx`, while its published module path is still `github.com/KLORQ/quarshell-core/slogx`. Change the public path only together with a new repository and a tested consumer migration.

## Review conventions

- Declare small interfaces in the consuming package; keep helpers private.
- Use semantic map aliases for public payloads, such as SDK `Credentials` and
  `Labels`, while preserving compatibility with ordinary maps.
- Return errors that affect correctness. Explain intentional discarded errors;
  a blank identifier alone does not require a comment in Go.
- Keep shared paths, ports, host aliases, and endpoints in `core/defaults`.
- Apply standard Go modernization with `go fix`, reviewing behavior changes.
  Changes to JSON parsing or serialization require a compatibility review.

Validate formatting, build, race tests, vet, platform-specific compilation,
and the linters configured in each repository. Build this site with
`mkdocs build --strict` before submitting documentation changes.

## Inference endpoints

Provider records may set `config.base_url` to an HTTP(S) upstream with a host
and without embedded credentials. Known provider types use their defaults;
unknown types have no implicit OpenAI fallback. DeepInfra uses its own endpoint.
The sandbox policy must allow the configured destination.
