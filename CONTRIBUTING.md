# Contributing to MCP Gateway

Thank you for your interest in contributing to MCP Gateway. This document
explains how to get started, what we expect from contributions, and how the
review process works.

## Developer Certificate of Origin (DCO)

Every commit **must** carry a `Signed-off-by` line. This certifies that you
wrote the patch or have the right to submit it under the project license.

```bash
git commit -s -m "feat: add support for stdio transport"
```

If you forget, amend the commit:

```bash
git commit --amend -s
```

## Prerequisites

| Tool     | Minimum version |
|----------|-----------------|
| Go       | 1.23+           |
| Docker   | 24+             |
| Kind     | 0.20+           |
| Helm     | 3.12+           |
| kubectl  | 1.28+           |

## Quick start

```bash
# 1. Fork and clone
git clone https://github.com/<you>/mcp-gateway.git
cd mcp-gateway

# 2. Build
make build

# 3. Run tests
make test

# 4. Lint
make lint

# 5. Spin up a local cluster and deploy
make kind-up
make dev-deploy
```

## Coding standards

- Follow the conventions documented in `CLAUDE.md`.
- Go code must pass `golangci-lint` without warnings.
- Every exported symbol needs a doc comment.
- Error messages start lowercase and do not end with punctuation.
- Wrap errors with `%w` so callers can use `errors.Is` / `errors.As`.
- Write table-driven tests; use `testify/assert` and `testify/require`.

## Pull request process

1. Create a feature branch from `main`:
   ```bash
   git checkout -b feat/my-feature
   ```
2. Make your changes. Keep commits small and focused.
3. Run the full check suite locally:
   ```bash
   make lint test
   ```
4. Sign off every commit (`git commit -s`).
5. Push your branch and open a pull request against `main`.
6. Fill in the PR template — describe what changed and why.
7. CI must pass before review. Reviewers may request changes; please
   address feedback in new commits (do not force-push during review).
8. Once approved, a maintainer will squash-merge your PR.

## Reporting issues

Open a GitHub issue. Include:
- What you expected to happen.
- What actually happened.
- Steps to reproduce.
- Kubernetes version, Go version, and operator version.

## Code of Conduct

Be respectful. We follow the [Contributor Covenant](https://www.contributor-covenant.org/).
