# How to release

ES-Operator is released as docker image to Zalando's public Docker repository `registry.opensource.zalan.do`. Every merge to master generates a new Docker image, you could go with `registry.opensource.zalan.do/poirot/es-operator:latest` for the latest and greatest version.

When introducing bigger changes, we create tags and write release notes. The release process itself is triggered via the Zalando-internal CI/CD tool.

## Steps

1. [Draft a new release](https://github.com/zalando-incubator/es-operator/releases/new) in Github, creating a new git tag of type `vX.Y.Z`. Document all changes in this release and link the related issues and PRs accordingly.
2. Re-trigger the internal CI/CD pipeline, which publishes of the docker image with the version tag `vX.Y.Z` attached.

