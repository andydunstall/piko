# CONTRIBUTING

## Building

Piko consists of a single binary with subcommands for `server`, `agent` and
`status`. Build `piko` with `make piko` which outputs the binary to `bin/piko`.

Requires Go 1.22.

## Testing

Piko has two types of tests:

#### Inline

Inline tests cover a relatively narrow scope and must run quickly (in under a
millisecond). These usually invoke the code being tested directly with
function/method calls, though may also use `localhost` to test a server
interface.

The tests live alongside the code being tested, such as to test `foo.go`
you may have `foo_test.go`.

Piko aims for as much code coverage as possible with inline tests.

Run with `go test ./...` or `make inline-test`.

#### System

System tests spin up a Piko cluster to test against.

These tests are used to verify all components and integrated and configured
properly, and cover more complex scenarios like chaos testing and load testing
of Piko.

The system tests are split into "short" and "long" tests (using the
`-test.short` flag).

Short tests are typically functional tests that run quickly, whereas long tests
are often load tests or more complex scenarios that take longer to run.

Most system tests spin up a Piko cluster within the same process as the test
runner, though others run Piko as an external process.

System tests are kept in `tests/` and can be run with `make system-test` and
`make system-test-short`.

## Style

Piko uses the [Uber Style Guide](https://github.com/uber-go/guide/blob/master/style.md)
though isn't too strict.

Format Piko with `make fmt` and run linters with `make lint`.

Group imports using `goimports -w -local github.com/dragonflydb/piko .`, or
`make import`.

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
