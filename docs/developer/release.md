# Kepler Release Workflow

This document describes the release workflow for Kepler.

## Notes

- The release is automatically triggered by a tag push
- Container images are stored in Quay.io
- The workflow is fully automated through GitHub Actions

## Archive signatures

The release workflow signs the final binary and Helm chart archives with
Sigstore Cosign. Each archive has a matching `.sigstore.json` bundle. The
signing job verifies both bundles. The release job verifies them again after
it downloads the signed artifacts. A failed check stops release publication.

The signing job has GitHub Actions OIDC access. The release job only needs
read access to verify signatures and write access to publish the GitHub Release.
The build and signing artifacts are retained for seven days so a failed job
can be retried with the original archives. Registry image and OCI chart
signatures are separate work and are not part of this change.

## Release Process Overview

![Release Workflow](assets/release.png)

## After Publishing a Release

Once the release workflow succeeds and both the Helm chart archive and container
image are published, open a follow-up PR to `main`:

1. Update the default `BASELINE_VERSION` in
   [`hack/test-helm.sh`](../../hack/test-helm.sh) to the new release version,
   without the leading `v`.
2. Update the script's `--help` default and the baseline version documented in
   [Helm Validation](e2e-testing.md#helm-validation).
3. Follow that guide to verify both upgrade modes.
