# CONTRIBUTING

## Building

Piko consists of a single binary with subcommands for `server`, `agent` and
`status`. Build `piko` with `make piko` which outputs the binary to `bin/piko`.

Requires Go 1.22.

## Testing

Piko has three categories of tests:
* Unit: Test a fairly narrow scope with no IO or blocking
* Integration: Again testing a fairly narrow scope but may use IO
* System: Spin up a Piko cluster to test against

Unit tests should cover as much of the code as possible.

Integration and system tests verify all components are integrated and
configured properly. They don't aim for complete coverage but test a few key
operations.

Unit and integration tests live along side the code under `foo_test.go` and
`foo_integration_test.go` respectively. System tests live in `tests/`.
Integration and system tests use Go build tags that must be enabled.

Run unit, integration and system tests with `make unit-test`,
`make integration-test` and `make system-test` respectively.

## Style

Piko uses the [Uber Style Guide](https://github.com/uber-go/guide/blob/master/style.md)
though isn't too strict.

Format Piko with `make fmt` and run linters with `make lint`.

## Commits

Piko uses rebase 'merge's, therefore please write well defined commits. Each
commit should be small with a clear description, and be easy to review in
isolation.

Each commit should have format:
```
<scope>: <description>

<optional body>
```

Such as:
```
server: add cluster status api

Adds a '/status/cluster' endpoint to inspect the nodes view of the cluster.
```

Or:
```
docs: extend routing docs

Extend architecture overview to describe how Piko handles nodes joining and
leaving the cluster.
```

## License
MIT License, please see [LICENSE](LICENSE) for details.
