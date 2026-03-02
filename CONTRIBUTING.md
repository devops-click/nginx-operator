# Contributing to NGINX Operator

Thank you for your interest in contributing to the NGINX Operator! This document provides guidelines and information for contributors.

## Code of Conduct

This project follows the [Contributor Covenant Code of Conduct](https://www.contributor-covenant.org/version/2/1/code_of_conduct/). By participating, you are expected to uphold this code.

## How to Contribute

### Reporting Issues

- Use the [GitHub Issues](https://github.com/devops-click/nginx-operator/issues) tracker
- Search existing issues before creating a new one
- Include: Kubernetes version, operator version, steps to reproduce, expected vs actual behavior
- For security vulnerabilities, see [SECURITY.md](SECURITY.md)

### Submitting Changes

1. **Fork** the repository
2. **Create a branch** from `main`:
   - Features: `feat/<name>`
   - Bug fixes: `fix/<name>`
   - Docs: `docs/<name>`
   - Chores: `chore/<name>`
3. **Write tests** for your changes
4. **Ensure all tests pass**: `make test-all`
5. **Ensure linting passes**: `make lint`
6. **Commit** using [Conventional Commits](https://www.conventionalcommits.org/):
   ```text
   feat(crd): add NginxRateLimit CRD
   fix(reconciler): prevent dual reload race condition
   docs(readme): update installation instructions
   ```
7. **Push** your branch and open a **Pull Request**

### Pull Request Guidelines

- PRs must target the `main` branch
- All CI checks must pass
- At least one maintainer approval is required
- Keep PRs focused — one feature or fix per PR
- Update documentation if your change affects user-facing behavior
- Add entries to `docs/upgrade-guide.md` for breaking changes

## Development Setup

```bash
# Clone your fork
git clone https://github.com/<your-user>/nginx-operator.git
cd nginx-operator

# Add upstream remote
git remote add upstream https://github.com/devops-click/nginx-operator.git

# Install dependencies and tools
go mod download
make controller-gen golangci-lint

# Run tests
make test

# Run linter
make lint

# Build
make build-all
```

## Testing

- **Unit tests**: `make test` — Tests config generation, validation, version
- **Integration tests**: `make test-integration` — Tests controllers with envtest
- **E2E tests**: `make test-e2e` — Full operator in a real cluster
- **Coverage**: `make coverage` — Generates HTML coverage report

All new code must have test coverage. PRs that decrease coverage may be rejected.

## Commit Message Format

We use [Conventional Commits](https://www.conventionalcommits.org/) for automated changelog generation:

```text
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

**Types**: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `chore`, `ci`

**Scopes**: `crd`, `controller`, `config`, `helm`, `ci`, `docs`, `deps`

**Breaking changes**: Add `!` after type/scope or `BREAKING CHANGE:` in footer.

## Release Process

Releases are automated via GitHub Actions when a tag is pushed:

1. Maintainer creates a release branch: `release/vX.Y`
2. Version bumps in `Chart.yaml`
3. Tag: `git tag vX.Y.Z && git push origin vX.Y.Z`
4. CI builds images, pushes to GHCR, publishes Helm chart, creates GitHub Release

## License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.
