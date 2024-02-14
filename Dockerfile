FROM docker.io/buildpack-deps:bullseye

RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
    cmake \
    ; rm -rf /var/lib/apt/lists/*

RUN ls -la /usr/lib
RUN ls -la /usr/include