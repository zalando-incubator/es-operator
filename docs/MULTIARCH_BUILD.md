# Multiarch Docker Build Setup

This document explains how to build and push multiarch Docker images for the ES-Operator to GitHub's Container Registry (ghcr.io).

## Overview

The ES-Operator now supports building Docker images for multiple architectures:
- `linux/amd64` (x86_64)
- `linux/arm64` (ARM 64-bit)

## GitHub Actions Workflow

The repository includes a GitHub Actions workflow (`.github/workflows/docker-multiarch.yaml`) that automatically:

1. **Builds multiarch binaries** for both amd64 and arm64
2. **Creates and pushes Docker images** to `ghcr.io/<owner>/<repo>`
3. **Handles authentication** using the built-in `GITHUB_TOKEN`
4. **Scans images for vulnerabilities** using Trivy
5. **Supports multiple triggers**:
   - Push to `master`/`main` branch
   - Git tags (releases)
   - Pull requests (build only, no push)
   - Manual workflow dispatch

### Image Tags

The workflow automatically creates the following tags:
- `latest` - for pushes to the default branch
- `<branch-name>` - for branch pushes
- `v<version>` - for semantic version tags
- `<branch>-<sha>` - for specific commits
- `pr-<number>` - for pull requests

## Local Development

### Prerequisites

1. **Docker Buildx**: Ensure Docker Buildx is installed and configured
   ```bash
   docker buildx version
   docker buildx create --use
   ```

2. **Go**: Version 1.25 or later
   ```bash
   go version
   ```

### Building Locally

#### Internal Builds (for Zalando delivery.yaml)
```bash
# Build for internal use with static base image
make build.docker.internal

# Push to internal registry
make build.push.internal
```

#### Public GHCR Builds (multiarch Amazon Linux 2023)
```bash
# Build binaries for both architectures
make build.multiarch

# Build multiarch Docker image (local only)
make build.docker.ghcr

# Build and push multiarch Docker image to GHCR
make build.push.ghcr
```

### Manual Docker Commands

If you prefer to use Docker commands directly:

```bash
# Build multiarch binaries
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o build/linux/amd64/es-operator .
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o build/linux/arm64/es-operator .

# Build and push multiarch image
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --file Dockerfile.multiarch \
  --tag ghcr.io/<owner>/<repo>:latest \
  --push \
  .
```

## Authentication

### GitHub Actions (Automatic)

The workflow automatically authenticates using the `GITHUB_TOKEN` with the necessary permissions:
- `contents: read` - to checkout code
- `packages: write` - to push to ghcr.io

### Local Development

To push images locally, you need to authenticate with GitHub Container Registry:

```bash
# Create a Personal Access Token with 'write:packages' scope
echo $GITHUB_TOKEN | docker login ghcr.io -u <username> --password-stdin

# Or use GitHub CLI
gh auth token | docker login ghcr.io -u <username> --password-stdin
```

## Files Added/Modified

- `.github/workflows/docker-multiarch.yaml` - GitHub Actions workflow for GHCR
- `Dockerfile.ghcr` - Public multiarch Dockerfile (Amazon Linux 2023)
- `Dockerfile.internal` - Internal Dockerfile (Zalando static base)
- `Dockerfile.e2e` - E2E test Dockerfile (Amazon Linux 2023)
- `docker-context/dnf.conf` - DNF configuration for Amazon Linux builds
- `Makefile` - Updated build targets for new structure
- `delivery.yaml` - Updated to use internal build targets
- `docs/MULTIARCH_BUILD.md` - This documentation

## Security

The workflow includes:
- **Vulnerability scanning** with Trivy
- **Minimal permissions** using GitHub's built-in tokens
- **SARIF upload** for security findings in GitHub Security tab
- **Cache optimization** for faster builds

## Troubleshooting

### Common Issues

1. **Authentication failures**: Ensure the repository has packages write permissions
2. **Platform build failures**: Check that the Go code compiles for both amd64 and arm64
3. **Image size differences**: arm64 images may be slightly different in size

### Debugging

To test the workflow locally, you can use [act](https://github.com/nektos/act):
```bash
# Install act
brew install act  # macOS
# or download from releases

# Run the workflow locally
act push -j build-and-push
```

## Migration from Zalando Registry

If migrating from the existing Zalando registry setup:

1. Update your deployment manifests to use `ghcr.io/<owner>/<repo>:latest`
2. Ensure pull permissions are configured for your cluster
3. The existing CI workflow will continue to work alongside the new multiarch workflow