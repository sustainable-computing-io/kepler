<!-- SPDX-FileCopyrightText: 2025 The Kepler Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# Podman Compose Setup for Kepler Development

This directory contains a podman-compose compatible version of the development environment.

## Why a Separate File?

The standard `compose.yaml` uses Docker Compose v2.20+ features like the `include` directive, which podman-compose doesn't fully support. The `compose-podman.yaml` file merges all services from:

- `../compose.yaml` (base Kepler services)
- `../monitoring/compose.yaml` (Prometheus & Grafana)
- `./override.yaml` (dev-specific overrides)

into a single file that works with podman-compose.

## Usage

### With Podman Compose

```bash
cd compose/dev

# Start all services
podman-compose -f compose-podman.yaml up -d

# View logs
podman-compose -f compose-podman.yaml logs -f kepler-dev

# Stop and clean up
podman-compose -f compose-podman.yaml down --volumes
```

### With Docker Compose (Standard)

For Docker Compose users, continue using the standard file:

```bash
cd compose/dev

# Start all services
docker compose up -d

# View logs
docker compose logs -f kepler-dev

# Stop and clean up
docker compose down --volumes
```

## Access Points

Once running, you can access:

- **Kepler Metrics**: <http://localhost:28283/metrics>
- **Prometheus**: <http://localhost:29090>
- **Grafana**: <http://localhost:23000> (credentials: admin/admin)
- **Scaphandre**: <http://localhost:28880/metrics> (not available on ARM)
- **Node Exporter**: <http://localhost:29100/metrics>
- **Sushy Static (Redfish Mock)**: <http://localhost:28001>

## Configuration

### Grafana User ID

Update the `user` field in the `grafana` service to match your user ID:

```bash
id -u  # Get your user ID
```

Then edit `compose-podman.yaml` and change:

```yaml
grafana:
  user: "1000"  # Change to your user ID
```

### Kubernetes Integration

To test with a Kind cluster:

1. Create a Kind cluster and get kubeconfig:

   ```bash
   make cluster-up
   cp ~/.kube/config ./shared/kube/kubeconfig
   ```

2. Update the kubeconfig to use `kind-control-plane:6443` as the server address

3. Uncomment the `kind` network in `compose-podman.yaml`:

   ```yaml
   networks:
     - kind

   networks:
     kind:
       external: true
   ```

## Differences from compose.yaml

The main differences in `compose-podman.yaml`:

1. **No `include` directive** - All services are defined in one file
2. **Merged overrides** - Dev-specific configurations are already applied
3. **Explicit network connections** - All services explicitly list their networks
4. **Hardcoded healthcheck intervals** - Environment variables replaced with default values
5. **No `extra_hosts: host.docker.internal:host-gateway`** - This Docker-specific feature is not supported by Podman. If you need to access host services from containers, use `host.containers.internal` or the host's actual IP address instead.

## Platform-Specific Notes

### Apple Silicon / ARM Macs

**Scaphandre Not Available**: The `hubblo/scaphandre` image is not available for ARM architecture. On Apple Silicon Macs, the scaphandre service will fail to start.

**Solutions**:

1. **Comment out scaphandre** in `compose-podman.yaml`:

   ```yaml
   # scaphandre:
   #   image: hubblo/scaphandre
   #   ...
   ```

2. **Remove from Prometheus scrape config** in `prometheus/scrape-configs/dev.yaml` if you commented it out

3. **Use Kepler and node-exporter only** - These work on ARM and provide comprehensive metrics

## Troubleshooting

### Permission Issues

If you encounter permission errors with `/proc` or `/sys`:

```bash
# Run with sudo (not recommended for production)
sudo podman-compose -f compose-podman.yaml up

# Or configure rootless podman properly
```

### Build Context Issues

If builds fail with context errors, ensure you're in the `compose/dev` directory:

```bash
pwd  # Should show: /path/to/kepler/compose/dev
```

### Network Issues

If services can't communicate:

```bash
# Check networks
podman network ls

# Recreate networks
podman-compose -f compose-podman.yaml down
podman-compose -f compose-podman.yaml up
```

## Maintenance

When updating the standard compose files, remember to sync changes to `compose-podman.yaml`:

1. Check for changes in `compose.yaml`
2. Check for changes in `../monitoring/compose.yaml`
3. Check for changes in `override.yaml`
4. Merge all changes into `compose-podman.yaml`

Consider using a script to automate this process in the future.
