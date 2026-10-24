# protocol repo

Public (Apache-2.0). The Glitchscout protocol v1 (`glitchscout/1`): JSON Schemas, golden fixtures and,
later, the binary frame spec and conformance material.

## Checks

`go test ./...` is the check loop. It validates every schema against the Draft 2020-12 metaschema,
compiles it, and checks every fixture: `fixtures/valid/<TYPE>/` must pass, `fixtures/invalid/<TYPE>/`
must be rejected. Also run `gofmt -l .` and `go vet ./...`.

## Rules

- This repo is public. Never add orchestrator code, internal planning docs, pricing, internal notes or
  secrets. Only what an SDK author needs to implement the protocol belongs here.
- Red → Green: add or change the fixtures first, see `go test` fail, then change the schema.
- Every message type needs at least one valid and one invalid fixture.
- A schema change that isn't backward compatible needs a protocol version bump.
- Commits and PR titles start with the ticket key: `GSC-n <imperative summary>`.
- No AI attribution in commits, PRs or review comments.
