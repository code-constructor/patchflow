# Contributing

Thanks for taking the time to contribute to Patchflow.

## Development setup

- Go 1.24 or newer and Git are the only requirements.
- `bin/dev` starts the local server from source.
- `bin/ci` runs formatting checks, `go vet`, the test suite, the documentation
  check, and a production build. Run it before opening a pull request.

## Guidelines

- Follow the product and technical direction in `AGENTS.md`.
- Every named Go function and method, including tests and unexported helpers,
  needs a Go Doc comment. Browser controllers use concise JSDoc comments.
- Add focused tests for new behavior.
- Keep the application runnable with the compiled binary and Git alone. Do not
  add Node, a bundler, a database, or a network dependency without discussing
  it in an issue first.
- Treat every filesystem path from requests or artifacts as untrusted.

## Pull requests

- Open an issue for larger changes before implementing them.
- Keep pull requests focused on one change.
- Describe what changed and why; link the related issue.
