# Contributing to SpiceSynth

Thank you for your interest in contributing to SpiceSynth.

## Getting Started

1. Fork and clone the repository.
2. Ensure you have Go 1.21+ and a C compiler installed (see the [README](README.md#prerequisites)).
3. Run the tests to verify your setup:
   ```bash
   go test ./...
   ```

## Development Workflow

1. Create a branch for your change.
2. Make your changes, adding or updating tests as needed.
3. Run the full test suite:
   ```bash
   go test ./...
   ```
4. Verify the examples still work:
   ```bash
   go run examples/demo/main.go
   ```
5. Open a pull request with a clear description of what you changed and why.

## Code Style

- Follow standard Go conventions (`gofmt`, `go vet`).
- All exported types and functions must have godoc comments.
- Do not add external Go dependencies to the core library (`chip/`, `voice/`, `sequencer/`, `stream/`, `patches/`). Examples may use external dependencies.

## Reporting Issues

Open an issue on GitHub with:
- A clear description of the problem or feature request.
- Steps to reproduce (for bugs).
- Your Go version, OS, and C compiler version.

## License

By contributing, you agree that your contributions will be licensed under LGPL-2.1-or-later.
