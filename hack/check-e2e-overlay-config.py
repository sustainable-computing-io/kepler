#!/usr/bin/env python3
# SPDX-FileCopyrightText: 2025 The Kepler Authors
# SPDX-License-Identifier: Apache-2.0
"""Fail if the e2e overlay ConfigMap drifts from production except fake-gpu-meter.enabled.

Kustomize cannot patch a nested field inside ConfigMap data.config.yaml, so the
overlay replaces that whole key. This check is the drift guard.
"""

from __future__ import annotations

import re
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
PROD_DIR = ROOT / "manifests" / "k8s"
OVERLAY_DIR = ROOT / "manifests" / "overlays" / "e2e"
ENABLED_RE = re.compile(r"(fake-gpu-meter:\n(?: {2})+enabled: )(true|false)")


def kustomize(args: list[str]) -> str:
    return subprocess.check_output(["kubectl", "kustomize", *args], text=True)


def kepler_config_yaml(rendered: str) -> str:
    for doc in re.split(r"(?m)^---\s*$", rendered):
        if not re.search(r"(?m)^kind:\s*ConfigMap\s*$", doc):
            continue
        if not re.search(r"(?m)^  name:\s*kepler\s*$", doc):
            continue
        match = re.search(r"(?m)^  config\.yaml: \|\s*\n((?:(?:    .*)?\n)*)", doc)
        if match is None:
            raise SystemExit("kepler ConfigMap has no config.yaml key")
        return "".join(
            line[4:] if line.startswith("    ") else line
            for line in match.group(1).splitlines(True)
        )
    raise SystemExit("kepler ConfigMap not found in kustomize output")


def enabled_flag(config: str, label: str) -> str:
    match = ENABLED_RE.search(config)
    if match is None:
        raise SystemExit(f"{label}: missing fake-gpu-meter.enabled")
    return match.group(2)


def normalize(config: str) -> str:
    normalized, n = ENABLED_RE.subn(r"\1CHECKED", config, count=1)
    if n != 1:
        raise SystemExit(f"expected one fake-gpu-meter.enabled, found {n}")
    return normalized


def main() -> int:
    prod = kepler_config_yaml(kustomize([str(PROD_DIR)]))
    overlay = kepler_config_yaml(
        kustomize(
            ["--load-restrictor=LoadRestrictionsNone", str(OVERLAY_DIR)]
        )
    )

    prod_enabled = enabled_flag(prod, "production")
    overlay_enabled = enabled_flag(overlay, "overlay")
    if prod_enabled != "false":
        print("production ConfigMap must keep fake-gpu-meter.enabled: false", file=sys.stderr)
        return 1
    if overlay_enabled != "true":
        print("e2e overlay ConfigMap must set fake-gpu-meter.enabled: true", file=sys.stderr)
        return 1

    if normalize(prod) != normalize(overlay):
        print(
            "e2e overlay ConfigMap drifted from manifests/k8s/configmap.yaml; "
            "only fake-gpu-meter.enabled may differ",
            file=sys.stderr,
        )
        return 1

    print("e2e overlay ConfigMap matches production except fake-gpu-meter.enabled: true")
    return 0


if __name__ == "__main__":
    sys.exit(main())
