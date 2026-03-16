# Contributing to nats-iam-broker

Thank you for your interest in contributing to nats-iam-broker! This document provides guidelines and instructions for contributing.

## Code of Conduct

Please be respectful and constructive in all interactions. We are committed to providing a welcoming and inclusive experience for everyone.

## How to Contribute

### Reporting Issues

- Use [GitHub Issues](https://github.com/jr200/nats-iam-broker/issues) to report bugs or request features.
- Before creating a new issue, please check if a similar issue already exists.
- Include as much detail as possible: steps to reproduce, expected vs actual behaviour, logs, and environment details.

### Submitting Changes

1. **Fork** the repository and create a branch from `main`.
2. **Make your changes** — keep commits focused and well-described.
3. **Add or update tests** for any new or changed functionality.
4. **Run the test suite** to make sure everything passes:
   ```bash
   go test ./...
   ```
5. **Open a Pull Request** against `main` with a clear description of your changes.

### Development Setup

```bash
# Clone your fork
git clone https://github.com/<your-username>/nats-iam-broker.git
cd nats-iam-broker

# Install dependencies
go mod download

# Run tests
go test ./...

# Build
go build ./...
```

### Pull Request Guidelines

- Keep PRs focused on a single change.
- Include tests for new functionality.
- Update documentation if your change affects user-facing behaviour.
- Ensure all CI checks pass before requesting review.

## License

By contributing to nats-iam-broker, you agree that your contributions will be licensed under the [MIT License](LICENSE).
