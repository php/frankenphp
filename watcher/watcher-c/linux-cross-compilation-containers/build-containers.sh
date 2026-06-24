#! /usr/bin/env bash
set -e
cd "$(dirname "$0")"
docker build -t pahole --platform linux/amd64 . -f Dockerfile.pahole
docker build -t meson-builder-x86_64-unknown-linux-gnu --build-arg BUILD_DIR=x86_64-unknown-linux-gnu --platform linux/amd64 .
docker build -t meson-builder-aarch64-unknown-linux-gnu --build-arg BUILD_DIR=aarch64-unknown-linux-gnu --platform linux/arm64 .
docker build -t meson-builder-armv7-unknown-linux-gnueabihf --build-arg BUILD_DIR=armv7-unknown-linux-gnueabihf --platform linux/arm/v7 .
