# Separate runtime revision: authenticated native-library security update.
# No creator inputs or packages enter this build. The original runtime remains
# addressable by its immutable OCI digest. Debian/CPython licenses are retained.
ARG PYTHON_IMAGE
FROM ${PYTHON_IMAGE} AS build
ARG DEBIAN_SNAPSHOT=20260904T000000Z
RUN test "$DEBIAN_SNAPSHOT" = 20260904T000000Z && \
    printf 'deb [check-valid-until=no] https://snapshot.debian.org/archive/debian/%s/ sid main\n' "$DEBIAN_SNAPSHOT" > /etc/apt/sources.list && \
    rm -f /etc/apt/sources.list.d/debian.sources && \
    apt-get -o Acquire::Check-Valid-Until=false update && \
    DEBIAN_FRONTEND=noninteractive apt-get install --no-install-recommends -y \
      libc6=2.43-4 libc-bin=2.43-4 libsqlite3-0=3.53.4-2 \
      libncursesw6=6.6+20260608-2 libtinfo6=6.6+20260608-2 libuuid1=2.42.2-4
COPY distroless-build.py /platform/distroless-build.py
RUN /usr/local/bin/python3 -I /platform/distroless-build.py /runtime

FROM scratch
COPY --from=build /runtime/ /
LABEL org.opencontainers.image.source="https://github.com/matbalez/science-ladder"
LABEL org.opencontainers.image.description="Science Ladder offline artifact-checker runtime with authenticated 20260904 Debian native-library updates; upstream licenses retained"
ENV PATH=/usr/local/bin LC_ALL=C.UTF-8 PYTHONDONTWRITEBYTECODE=1
ENTRYPOINT ["/usr/local/bin/python3"]
