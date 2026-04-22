# Contributing to nats-iam-broker

Thank you for your interest in contributing to nats-iam-broker! This document provides guidelines and instructions for contributing.

## Code of Conduct

Please be respectful and constructive in all interactions. We are committed to providing a welcoming and inclusive experience for everyone.

## How to Contribute

### Reporting Issues

- Use [GitHub Issues](https://github.com/jr200-labs/nats-iam-broker/issues) to report bugs or request features.
- Before creating a new issue, please check if a similar issue already exists.
- Include as much detail as possible: steps to reproduce, expected vs actual behaviour, logs, and environment details.

### Submitting Changes

1. **Fork** the repository and create a branch from `master`.
2. **Make your changes** — keep commits focused and well-described.
3. **Use Conventional Commit messages** — required, see below. commitlint enforces this locally and in CI.
4. **Add or update tests** for any new or changed functionality.
5. **Run the test suite** to make sure everything passes:
   ```bash
   make test          # unit tests
   make test-integration  # integration tests (embedded NATS + mock OIDC)
   ```
6. **Open a Pull Request** against `master` with a clear description of your changes.

### Development Setup

```bash
# Clone your fork
git clone https://github.com/<your-username>/nats-iam-broker.git
cd nats-iam-broker

# Install dependencies
go mod download

# Install pre-commit hooks (runs sync, golangci-lint, commitlint).
# Requires `uv` — https://docs.astral.sh/uv/. Or `pipx install pre-commit`.
uvx pre-commit install --install-hooks
uvx pre-commit install --hook-type commit-msg

# Run unit tests
make test

# Run integration tests
make test-integration

# Build
make build
```

### Commit Format

Commits must follow [Conventional Commits](https://www.conventionalcommits.org/). This is enforced by `commitlint` (local pre-commit hook + CI) and drives automated version bumps via release-please.

| Prefix | Effect |
|--------|--------|
| `feat:` | minor bump (major on 1.x with `feat!:`) |
| `fix:` / `perf:` / `deps:` / `revert:` | patch bump |
| `refactor:` / `test:` / `build:` | patch bump (shown in changelog) |
| `docs:` / `chore:` / `ci:` / `style:` | no bump (hidden from changelog) |
| `feat!:` or `BREAKING CHANGE:` footer | major bump |

### Releases

Releases are automated via [release-please](https://github.com/googleapis/release-please). **Do not manually tag or push `vX.Y.Z`** — a standing release PR on `master` bumps the version and changelog based on Conventional Commits. Merging that PR creates the tag, GitHub Release, and triggers the docker → helm → publish chain.

### Pull Request Guidelines

- Keep PRs focused on a single change.
- Include tests for new functionality.
- Update documentation if your change affects user-facing behaviour.
- Ensure all CI checks pass before requesting review.

## License

By contributing to nats-iam-broker, you agree that your contributions will be licensed under the [MIT License](LICENSE).
