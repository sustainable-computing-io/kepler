#!/bin/bash
# Enable GPU Time-Slicing on OpenShift
# This script:
# 1. Creates the time-slicing ConfigMap with replicas: 4
# 2. Patches the existing ClusterPolicy to use the ConfigMap
# 3. Waits for device plugin to restart
# 4. Verifies the configuration

set -e

# Get the directory where this script is located (works when run from any directory)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "=== Enabling GPU Time-Slicing ==="
echo ""

# Prerequisites check
echo "Checking prerequisites..."
echo ""

# Check 1: NVIDIA GPU Operator
echo "  [1/2] Checking for NVIDIA GPU Operator..."
if ! oc get namespace nvidia-gpu-operator &>/dev/null; then
	echo "  ✗ Namespace 'nvidia-gpu-operator' not found"
	echo ""
	echo "ERROR: NVIDIA GPU Operator is not installed."
	echo "Install it first: https://docs.nvidia.com/datacenter/cloud-native/gpu-operator/latest/getting-started.html"
	exit 1
fi

GPU_OPERATOR_PODS=$(oc get pods -n nvidia-gpu-operator -l app=gpu-operator -o name 2>/dev/null | wc -l)
if [ "$GPU_OPERATOR_PODS" -eq 0 ]; then
	echo "  ✗ No GPU Operator pods found in nvidia-gpu-operator namespace"
	echo ""
	echo "ERROR: NVIDIA GPU Operator is not running."
	exit 1
fi
echo "  ✓ NVIDIA GPU Operator is installed and running"

# Check 2: ClusterPolicy (auto-detect or use env var)
echo "  [2/2] Checking for ClusterPolicy..."

if [ -n "${CLUSTER_POLICY:-}" ]; then
	# User specified policy via env var
	if ! oc get clusterpolicy "$CLUSTER_POLICY" &>/dev/null; then
		echo "  ✗ ClusterPolicy '$CLUSTER_POLICY' not found"
		exit 1
	fi
	echo "  ✓ Using specified ClusterPolicy '$CLUSTER_POLICY'"
else
	# Auto-detect
	CLUSTER_POLICIES=$(oc get clusterpolicy -o name 2>/dev/null)
	POLICY_COUNT=$(echo "$CLUSTER_POLICIES" | grep -c "clusterpolicy" || true)

	if [ "$POLICY_COUNT" -eq 0 ]; then
		echo "  ✗ No ClusterPolicy found"
		echo ""
		echo "ERROR: ClusterPolicy is required but not found."
		echo "Create a ClusterPolicy first or check if the GPU Operator installation completed."
		exit 1
	elif [ "$POLICY_COUNT" -gt 1 ]; then
		echo "  ✗ Multiple ClusterPolicies found:"
		echo "    - ${CLUSTER_POLICIES//clusterpolicy.nvidia.com\//}"
		echo ""
		echo "ERROR: Cannot auto-detect which ClusterPolicy to use."
		echo "Set CLUSTER_POLICY env var: CLUSTER_POLICY=<name> $0"
		exit 1
	fi

	# Extract policy name (remove 'clusterpolicy.nvidia.com/' prefix)
	CLUSTER_POLICY="${CLUSTER_POLICIES//clusterpolicy.nvidia.com\//}"
	echo "  ✓ ClusterPolicy '$CLUSTER_POLICY' found (auto-detected)"
fi
echo ""

# Check if time-slicing is already enabled
echo "Checking if time-slicing is already enabled..."
CURRENT_CONFIG=$(oc get clusterpolicy "$CLUSTER_POLICY" -o jsonpath='{.spec.devicePlugin.config.name}' 2>/dev/null || true)
if [ "$CURRENT_CONFIG" = "time-slicing-config" ]; then
	# Verify ConfigMap also exists
	if oc get configmap time-slicing-config -n nvidia-gpu-operator &>/dev/null; then
		echo "✓ Time-slicing is already enabled on ClusterPolicy '$CLUSTER_POLICY'"
		echo ""
		echo "To verify configuration, run:"
		echo "  $SCRIPT_DIR/verify-gpu-timeslicing.sh"
		exit 0
	fi
fi
echo ""

# Step 1: Apply the ConfigMap
echo "Step 1: Creating time-slicing ConfigMap with replicas: 4..."
oc apply -f "$SCRIPT_DIR/enable-gpu-timeslicing.yaml"
echo "✓ ConfigMap created"
echo ""

# Step 2: Patch the ClusterPolicy (don't replace it!)
echo "Step 2: Patching ClusterPolicy '$CLUSTER_POLICY' to use time-slicing config..."
oc patch clusterpolicy "$CLUSTER_POLICY" --type=merge -p '
spec:
  devicePlugin:
    config:
      name: time-slicing-config
      default: "any"
'
echo "✓ ClusterPolicy patched"
echo ""

# Step 3: Wait for device plugin to restart
echo "Step 3: Waiting for device plugin pods to restart..."
sleep 5

# Delete device plugin pods to force config reload
echo "Restarting device plugin pods..."
oc delete pods -n nvidia-gpu-operator -l app=nvidia-device-plugin-daemonset

# Wait for new pods to be ready
echo "Waiting for device plugin pods to be ready..."
oc wait --for=condition=Ready pod -l app=nvidia-device-plugin-daemonset -n nvidia-gpu-operator --timeout=120s
echo "✓ Device plugin restarted"
echo ""

# Step 4: Verify configuration
echo "Step 4: Verifying time-slicing configuration..."
echo ""

# Check replicas label on nodes
echo "Checking GPU replicas on nodes:"
oc get nodes -l nvidia.com/gpu.present=true -o json |
	jq -r '.items[] | "\(.metadata.name): replicas=\(.metadata.labels."nvidia.com/gpu.replicas")"'
echo ""

# Check GPU capacity
echo "Checking GPU capacity (should be 4 per node):"
oc get nodes -l nvidia.com/gpu.present=true -o json |
	jq -r '.items[] | "\(.metadata.name): capacity=\(.status.allocatable."nvidia.com/gpu")"'
echo ""

# Verify in ClusterPolicy
echo "Checking ClusterPolicy configuration:"
oc get clusterpolicy "$CLUSTER_POLICY" -o jsonpath='{.spec.devicePlugin.config}' | jq .
echo ""

echo "==================================="
echo "✓ GPU Time-Slicing Enabled!"
echo "==================================="
echo ""
echo "Verification:"
echo "  - Replicas should show: 4"
echo "  - Capacity should show: 4 (per node)"
echo ""
echo "Now you can deploy workloads with fractional GPU requests:"
echo "  nvidia.com/gpu: 1  # 1/4 of physical GPU (25%)"
echo "  nvidia.com/gpu: 2  # 1/2 of physical GPU (50%)"
echo "  nvidia.com/gpu: 3  # 3/4 of physical GPU (75%)"
echo "  nvidia.com/gpu: 4  # Full physical GPU (100%)"
echo ""
echo "Test with:"
echo "  oc run gpu-test --image=nvidia/cuda:12.2.0-base-ubuntu22.04 --limits=nvidia.com/gpu=1 --restart=Never -- sleep 300"
echo "  oc exec gpu-test -- nvidia-smi"
echo ""
