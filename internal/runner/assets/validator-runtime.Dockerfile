# Platform-owned recipe. The inherited Python/Debian components retain their
# respective upstream licenses. Only this recipe and Science Ladder code are MIT.
ARG PYTHON_IMAGE
FROM ${PYTHON_IMAGE} AS build
COPY distroless-build.py /platform/distroless-build.py
RUN /usr/local/bin/python3 -I /platform/distroless-build.py /runtime

FROM scratch
COPY --from=build /runtime/ /
LABEL org.opencontainers.image.source="https://github.com/matbalez/science-ladder"
LABEL org.opencontainers.image.description="Science Ladder offline artifact-checker runtime; upstream component licenses retained"
ENV PATH=/usr/local/bin LC_ALL=C.UTF-8 PYTHONDONTWRITEBYTECODE=1
ENTRYPOINT ["/usr/local/bin/python3"]
