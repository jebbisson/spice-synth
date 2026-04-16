# Module Review Follow-Ups

## Goal

Capture the remaining repository and module-consumption cleanup work that is separate from the main licensing and dynamic-linking redesign.

This file intentionally excludes:

- converting `adl/` away from derivative-license risk
- separating LGPL components into replaceable dynamic-link targets
- repository split decisions for runtime-linked backends

Those items are the primary licensing architecture track and should be handled separately.

## Remaining Work

### 1. Remove private sample files from tests

- [ ] remove unit/integration tests that require private `examples/adl/*.ADL` files
- [ ] remove unit/integration tests that require private `examples/midi/*.mid` files
- [ ] replace those tests with synthetic byte fixtures, generated test data, or parser-level coverage that does not rely on private media files
- [ ] keep manual file-driven validation in standalone examples or documented local-only workflows
- [ ] ensure `go test ./...` works in a clean checkout with no private assets present

### 2. Stop documenting private assets as if they ship with the module

- [ ] update `README.md` so quick-start usage does not assume bundled `DUNE*.ADL` or `Title.mid` files exist
- [ ] update `examples/README.md` so private asset usage is clearly marked as local-only developer workflow
- [ ] prefer examples that accept user-provided file paths over examples that imply repo-shipped copyrighted content
- [ ] document which examples are expected to work from a clean clone and which require user-supplied content

### 3. Align Go version messaging

- [ ] decide the real minimum supported Go version for the root module
- [ ] make `go.mod`, `README.md`, `CONTRIBUTING.md`, and CI all state the same support policy
- [ ] decide whether patch-level `go` directives like `go 1.24.4` are intentional or should be normalized to a major/minor target
- [ ] make the same decision consistently across nested example modules

### 4. Clarify example-module boundaries

- [ ] document that `examples/adl_player`, `examples/midi_player`, `examples/ebiten_player`, and `examples/adl_extracted_player` are separate nested modules
- [ ] explain that their dependencies are intentionally isolated from the root library module
- [ ] decide which example modules are expected to build in CI and which are local/manual only
- [ ] keep root library documentation focused on importable packages first, with example apps as secondary material

### 5. Clean repository publication hygiene

- [ ] decide whether `.github/workflows/ci.yml` is intended to be committed and published
- [ ] remove or relocate stray planning/scratch artifacts that should not be part of the release-facing repo surface
- [ ] ensure generated binaries and local outputs remain ignored and untracked
- [ ] review top-level files so the repo surface matches what downstream users should see

### 6. Improve release readiness for Go module consumers

- [ ] choose an initial tagging strategy (`v0.x` vs `v1.0.0`)
- [ ] tag releases once licensing/module cleanup is in a publishable state
- [ ] verify pkg.go.dev-facing package docs remain accurate after the documentation cleanup
- [ ] do a clean-clone validation pass before the first public release tag

## Suggested Order After Licensing Track

1. Remove private-asset-dependent tests
2. Update docs/examples to stop implying private files ship with the repo
3. Align Go version policy across metadata and CI
4. Clarify nested example module expectations
5. Clean publication hygiene
6. Tag the module once the repo is publishable

## Success Criteria

- [ ] `go test ./...` passes without private assets
- [ ] docs do not imply copyrighted/private sample media is distributed
- [ ] module consumers can understand what is part of the root module vs standalone examples
- [ ] version/support policy is consistent across docs and CI
- [ ] repo surface is clean enough for tagging and public consumption
