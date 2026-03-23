# Multi-Architecture Builds

Kepler supports building for multiple architectures (amd64, arm64). Both base
images (`golang:1.24` and `ubi9:latest`) are already multi-arch, so Docker
pulls the correct variant automatically.

## Native Build

No changes needed — existing targets work as-is:

```bash
make build
make image
```

## Multi-Arch Container Images

### Build all arches locally

```bash
make image-multi KEPLER_IMAGE=quay.io/<your-registry>/kepler:latest
```

This builds one image per architecture and tags them with an arch suffix
(e.g., `quay.io/<your-registry>/kepler:latest-amd64`, `quay.io/<your-registry>/kepler:latest-arm64`).
Images are loaded into the local Docker daemon — nothing is pushed.

### Push and create manifest

```bash
make push-multi KEPLER_IMAGE=quay.io/<your-registry>/kepler:latest
```

This pushes per-arch images and creates a multi-arch manifest at the base tag.

### Build a single arch

```bash
make image GOARCH=arm64 KEPLER_IMAGE=quay.io/<your-registry>/kepler:latest-arm64
```

## Cross-Compilation

To cross-compile a host binary (outside Docker), set `GOARCH` and `CC`.
Go cannot auto-detect a C cross-compiler, and Kepler requires CGO because
go-nvml loads `libnvidia-ml.so.1` via `dlopen` at runtime:

```bash
GOARCH=arm64 CC=aarch64-linux-gnu-gcc make build
```

The Makefile auto-detects the sysroot on Fedora. On Debian/Ubuntu, install
`gcc-aarch64-linux-gnu`.

> **Note:** Container builds (`make image`, `image-multi`) handle cross-compilation
> automatically — you do not need to set `CC` for Docker builds.

## Makefile Variables

| Variable       | Default            | Description                                         |
|----------------|--------------------|-----------------------------------------------------|
| `GOARCH`       | `$(go env GOARCH)` | Target architecture                                 |
| `CC`           | *(system default)* | C cross-compiler (host builds only, not for Docker) |
| `SYSROOT`      | *(auto-detected)*  | Cross-compiler sysroot (auto-detected on Fedora)    |
| `IMAGE_ARCHES` | `amd64 arm64`      | Architectures built by `image-multi`                |
