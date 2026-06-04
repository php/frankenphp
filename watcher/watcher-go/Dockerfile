# syntax=docker/dockerfile:1
FROM golang:latest

COPY .. /opt/watcher
WORKDIR /opt/watcher

RUN apt-get update && \
    apt-get -y install cmake && \
    cmake -S . -B build -DCMAKE_BUILD_TYPE=Release && \
    cmake --build build && \
    cmake --install build && \
    ldconfig

WORKDIR /usr/src/app
