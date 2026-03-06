# Using GitHub Actions

This repository builds and deploys the Docker image to [Docker Hub](https://hub.docker.com/r/dunglas/frankenphp) on
every approved pull request or on your own fork once setup.

## Setting up GitHub Actions

In the repository settings, under secrets, add the following secrets:

- `REGISTRY_LOGIN_SERVER`: The Docker registry to use (e.g. `docker.io`).
- `REGISTRY_USERNAME`: The username to use to log in to the registry (e.g. `dunglas`).
- `REGISTRY_PASSWORD`: The password to use to log in to the registry (e.g. an access key).
- `IMAGE_NAME`: The name of the image (e.g. `dunglas/frankenphp`).

## Building and Pushing the Image

1. Create a Pull Request or push to your fork.
2. GitHub Actions will build the image and run any tests.
3. If the build is successful, the image will be pushed to the registry using the `pr-x`, where `x` is the PR number, as the tag.

## Deploying the Image

1. Once the Pull Request is merged, GitHub Actions will again run the tests and build a new image.
2. If the build is successful, the `main` tag will be updated in the Docker registry.

## Releases

1. Create a new tag in the repository.
2. GitHub Actions will build the image and run any tests.
3. If the build is successful, the image will be pushed to the registry using the tag name as the tag (e.g. `v1.2.3` and `v1.2` will be created).
4. The `latest` tag will also be updated.
