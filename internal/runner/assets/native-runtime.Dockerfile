# MIT platform-owned recipe. Native tools and libraries retain upstream licenses.
# This is a NEW execution profile; it must not replace a locked v1 rootfs.
ARG BASE_IMAGE
FROM python@sha256:ed86c82274b3c69b52fb5820f358f0bd7df0b603332063cb5c6e32bd220c3e6e AS certificates
FROM ${BASE_IMAGE}
ARG DEBIAN_SNAPSHOT=20260907T000000Z
COPY --from=certificates /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
RUN test "$DEBIAN_SNAPSHOT" = 20260907T000000Z && \
    printf 'deb [check-valid-until=no] https://snapshot.debian.org/archive/debian/%s/ sid main\n' "$DEBIAN_SNAPSHOT" > /etc/apt/sources.list && \
    rm -f /etc/apt/sources.list.d/debian.sources && \
    apt-get -o Acquire::Check-Valid-Until=false update && \
    DEBIAN_FRONTEND=noninteractive apt-get install --no-install-recommends -y \
      ca-certificates gcc g++ libc6-dev cargo rustc make pkg-config \
      python3 python3-numpy python3-scipy libeigen3-dev && \
    rm -rf /var/lib/apt/lists/* /var/cache/apt/archives/*
# Keep the authenticated bootstrap CA outside package-managed paths while the
# base distribution upgrades its TLS libraries and certificate package.
COPY --from=certificates /etc/ssl/certs/ca-certificates.crt /tmp/platform-bootstrap-ca.crt
RUN apt-get -o Acquire::https::CaInfo=/tmp/platform-bootstrap-ca.crt -o Acquire::Check-Valid-Until=false update && \
    DEBIAN_FRONTEND=noninteractive apt-get -o Acquire::https::CaInfo=/tmp/platform-bootstrap-ca.crt dist-upgrade --no-install-recommends -y && \
    rm -rf /var/lib/apt/lists/* /var/cache/apt/archives/* /tmp/platform-bootstrap-ca.crt
COPY candidate-sandbox.c /tmp/candidate-sandbox.c
RUN mkdir -p /usr/local/bin /sl/candidate /sl/broker && \
    gcc -O2 -Wall -Wextra -Werror -static /tmp/candidate-sandbox.c -o /usr/local/bin/sl-candidate-sandbox && \
    rm /tmp/candidate-sandbox.c && \
    ln -s /usr/bin/python3 /usr/local/bin/python3 && \
    find /usr -xdev -type f -perm /6000 -exec chmod a-s '{}' + && \
    dpkg-query -W -f='${binary:Package}\t${Version}\t${source:Package}\t${source:Version}\n' > /usr/local/share/science-ladder-native-packages.tsv
LABEL org.opencontainers.image.source="https://github.com/matbalez/science-ladder"
LABEL org.opencontainers.image.description="Science Ladder isolated native evaluation toolchain; requires enrolled Linux v2 guest controls"
ENV PATH=/usr/local/bin:/usr/bin:/bin LC_ALL=C.UTF-8 PYTHONDONTWRITEBYTECODE=1
ENTRYPOINT ["/usr/local/bin/python3"]
