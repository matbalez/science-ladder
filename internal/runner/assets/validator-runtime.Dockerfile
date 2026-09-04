# Platform-owned recipe. The inherited Python/Debian components retain their
# respective upstream licenses. Only this recipe and Science Ladder code are MIT.
ARG PYTHON_IMAGE
FROM ${PYTHON_IMAGE}
LABEL org.opencontainers.image.source="https://github.com/matbalez/science-ladder"
LABEL org.opencontainers.image.description="Science Ladder offline artifact-checker runtime; upstream component licenses retained"
# The quarantine builder installs hash-verified wheels by bounded extraction.
# No installer or its bundled libraries are needed inside the checker runtime.
RUN /usr/local/bin/python3 -I -c 'import pathlib,shutil,sysconfig; root=pathlib.Path(sysconfig.get_path("purelib")); targets=[root/"pip",*root.glob("pip-*.dist-info")]; [shutil.rmtree(p) for p in targets if p.is_dir()]; scripts=pathlib.Path(sysconfig.get_path("scripts")); [p.unlink() for p in scripts.glob("pip*") if p.is_file()]'
