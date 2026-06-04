FROM debian:latest
ARG BUILD_DIR
RUN apt-get update -yqq && apt-get install -yqq build-essential meson ninja-build
COPY build.sh /usr/local/bin/build.sh
ENV BUILD_DIR=$BUILD_DIR
ENTRYPOINT ["build.sh"]
