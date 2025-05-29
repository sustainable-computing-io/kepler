# Fake Process File System for Testing

## NOTES

### Docker Container

proc/3456208 is the prometheus docker container

### Podman Container

All pids expect 3456208, 1337 are in a podman container.

## Creating new Test Data

Use `copy-pid.sh` to copy of a process running on the host to procfs directory.
An empty file that points to the exe will also be create.

```bash
./copy-pid.sh PID
```
