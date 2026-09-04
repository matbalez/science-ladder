# First-party build tooling only. Supply approved immutable Debian image and
# package snapshot values; this image is built/pinned before processing challenges.
ARG BASE_IMAGE
FROM ${BASE_IMAGE}
ARG DEBIAN_SNAPSHOT
RUN test -n "$DEBIAN_SNAPSHOT" && \
    printf 'deb [check-valid-until=no] https://snapshot.debian.org/archive/debian/%s/ bookworm main\n' "$DEBIAN_SNAPSHOT" > /etc/apt/sources.list && \
    rm -f /etc/apt/sources.list.d/debian.sources && \
    apt-get -o Acquire::Check-Valid-Until=false update && \
    apt-get install --no-install-recommends -y e2fsprogs squashfs-tools tar coreutils ca-certificates && \
    rm -rf /var/lib/apt/lists/*
