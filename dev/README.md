# Local Dev Env Setup

This quick tutorial is for developing and testing Kepler locally

# Install bcc-devel and kernel-devel 

Refer to the [builder Dockerfile](https://github.com/sustainable-computing-io/kepler/blob/main/build/Dockerfile.builder)

# Compile 
Go to the root of the repo and do the following:

```bash
 make _build_local
```

If successful, the binary is at `_output/bin/_/kepler`

# Test

Create the k8s role and token, this is only needed once.
```bash
./create_k8s_token.sh
```

Then run the Kepler binary at `_output/bin/_/kepler`

