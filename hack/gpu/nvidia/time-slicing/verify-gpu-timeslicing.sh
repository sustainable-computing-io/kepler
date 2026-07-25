#!/bin/bash
set -e

# Get the directory where this script is located (works when run from any directory)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "=== GPU Time-Slicing Verification ==="
echo ""

# Auto-detect ClusterPolicy
if [ -n "${CLUSTER_POLICY:-}" ]; then
	if ! oc get clusterpolicy "$CLUSTER_POLICY" &>/dev/null; then
		echo "✗ ClusterPolicy '$CLUSTER_POLICY' not found"
		exit 1
	fi
else
	CLUSTER_POLICIES=$(oc get clusterpolicy -o name 2>/dev/null)
	POLICY_COUNT=$(echo "$CLUSTER_POLICIES" | grep -c "clusterpolicy" || true)
	if [ "$POLICY_COUNT" -eq 0 ]; then
		echo "✗ No ClusterPolicy found"
		exit 1
	elif [ "$POLICY_COUNT" -gt 1 ]; then
		echo "✗ Multiple ClusterPolicies found. Set CLUSTER_POLICY env var."
		exit 1
	fi
	CLUSTER_POLICY="${CLUSTER_POLICIES//clusterpolicy.nvidia.com\//}"
fi

# Check if time-slicing ConfigMap exists
echo "[1/5] Checking time-slicing ConfigMap..."
if oc get configmap time-slicing-config -n nvidia-gpu-operator &>/dev/null; then
	echo "✓ ConfigMap 'time-slicing-config' exists"
	echo ""
	echo "Configuration:"
	oc get configmap time-slicing-config -n nvidia-gpu-operator -o yaml | grep -A 20 "data:"
else
	echo "✗ ConfigMap 'time-slicing-config' not found"
	echo "  Run $SCRIPT_DIR/apply-gpu-timeslicing.sh first"
	exit 1
fi

echo ""
echo "[2/5] Checking ClusterPolicy '$CLUSTER_POLICY' configuration..."
DEVICE_PLUGIN_CONFIG=$(oc get clusterpolicy "$CLUSTER_POLICY" -o jsonpath='{.spec.devicePlugin.config.name}' 2>/dev/null)
DEVICE_PLUGIN_DEFAULT=$(oc get clusterpolicy "$CLUSTER_POLICY" -o jsonpath='{.spec.devicePlugin.config.default}' 2>/dev/null)

echo "Device Plugin Configuration:"
if [ "$DEVICE_PLUGIN_CONFIG" = "time-slicing-config" ]; then
	echo "  ✓ Config name:    $DEVICE_PLUGIN_CONFIG"
	echo "  ✓ Default config: $DEVICE_PLUGIN_DEFAULT"
else
	echo "  ✗ Not configured for time-slicing"
	echo "    Current config: ${DEVICE_PLUGIN_CONFIG:-none}"
	exit 1
fi

echo ""
echo "DCGM Exporter Configuration:"
echo "---"
oc get clusterpolicy "$CLUSTER_POLICY" -o yaml | grep -A 10 "dcgmExporter:" | sed 's/^/  /'
echo "---"

DCGM_ENABLED=$(oc get clusterpolicy "$CLUSTER_POLICY" -o jsonpath='{.spec.dcgmExporter.enabled}' 2>/dev/null)
DCGM_VIRTUAL_GPUS=$(oc get clusterpolicy "$CLUSTER_POLICY" -o jsonpath='{.spec.dcgmExporter.env[?(@.name=="KUBERNETES_VIRTUAL_GPUS")].value}' 2>/dev/null)
DCGM_SM_ENABLED=$(oc get clusterpolicy "$CLUSTER_POLICY" -o jsonpath='{.spec.dcgmExporter.serviceMonitor.enabled}' 2>/dev/null)
DCGM_SM_INTERVAL=$(oc get clusterpolicy "$CLUSTER_POLICY" -o jsonpath='{.spec.dcgmExporter.serviceMonitor.interval}' 2>/dev/null)

echo ""
echo "Verification:"
if [ "$DCGM_ENABLED" = "true" ]; then
	echo "  ✓ DCGM Exporter enabled"
else
	echo "  ⚠ DCGM Exporter: ${DCGM_ENABLED:-not configured}"
fi

if [ "$DCGM_VIRTUAL_GPUS" = "true" ]; then
	echo "  ✓ KUBERNETES_VIRTUAL_GPUS: true"
else
	echo "  ⚠ KUBERNETES_VIRTUAL_GPUS: ${DCGM_VIRTUAL_GPUS:-not set}"
fi

if [ "$DCGM_SM_ENABLED" = "true" ]; then
	echo "  ✓ ServiceMonitor enabled (interval: ${DCGM_SM_INTERVAL:-default})"
else
	echo "  ⚠ ServiceMonitor: ${DCGM_SM_ENABLED:-not configured}"
fi

echo ""
echo "[3/5] Checking GPU nodes..."
GPU_NODES=$(oc get nodes -l nvidia.com/gpu.present=true -o name 2>/dev/null)
GPU_NODE_COUNT=$(echo "$GPU_NODES" | wc -l)

if [ -z "$GPU_NODES" ]; then
	echo "✗ No GPU nodes found"
	exit 1
fi

echo "✓ Found $GPU_NODE_COUNT GPU node(s):"
for node in ${GPU_NODES//node\//}; do
	INSTANCE_TYPE=$(oc get node "$node" -o jsonpath='{.metadata.labels.node\.kubernetes\.io/instance-type}' 2>/dev/null)
	GPU_PRODUCT=$(oc get node "$node" -o jsonpath='{.metadata.labels.nvidia\.com/gpu\.product}' 2>/dev/null | sed 's/-SHARED//')
	echo "  - $node"
	echo "    Instance: $INSTANCE_TYPE, GPU: $GPU_PRODUCT"
done

echo ""
echo "[4/5] Verifying GPU allocatable capacity..."
echo ""

for node in ${GPU_NODES//node\//}; do
	echo "Node: $node"

	# Get physical GPU count from node label
	PHYSICAL_GPU_COUNT=$(oc get node "$node" -o jsonpath='{.metadata.labels.nvidia\.com/gpu\.count}' 2>/dev/null)
	GPU_ALLOCATABLE=$(oc get node "$node" -o jsonpath='{.status.allocatable.nvidia\.com/gpu}')
	REPLICAS=$(oc get node "$node" -o jsonpath='{.metadata.labels.nvidia\.com/gpu\.replicas}' 2>/dev/null)
	SHARING_STRATEGY=$(oc get node "$node" -o jsonpath='{.metadata.labels.nvidia\.com/gpu\.sharing-strategy}' 2>/dev/null)

	echo "  Physical GPUs:        $PHYSICAL_GPU_COUNT"
	echo "  Allocatable GPUs:     $GPU_ALLOCATABLE"

	if [ -n "$SHARING_STRATEGY" ] && [ "$SHARING_STRATEGY" = "time-slicing" ]; then
		echo "  Sharing Strategy:     $SHARING_STRATEGY"
		echo "  Configured Replicas:  $REPLICAS"

		EXPECTED_ALLOCATABLE=$((PHYSICAL_GPU_COUNT * REPLICAS))
		if [ "$GPU_ALLOCATABLE" -eq "$EXPECTED_ALLOCATABLE" ]; then
			echo "  ✓ Time-slicing is ACTIVE (${PHYSICAL_GPU_COUNT} × ${REPLICAS} = ${GPU_ALLOCATABLE})"
		else
			echo "  ⚠ Expected ${EXPECTED_ALLOCATABLE} allocatable GPUs, but found ${GPU_ALLOCATABLE}"
		fi
	else
		echo "  ⚠ Time-slicing not configured (sharing-strategy: ${SHARING_STRATEGY:-none})"
	fi
	echo ""
done

echo "[5/5] Checking device plugin pods..."
DEVICE_PLUGIN_PODS=$(oc get pods -n nvidia-gpu-operator -l app=nvidia-device-plugin-daemonset -o name 2>/dev/null)

if [ -z "$DEVICE_PLUGIN_PODS" ]; then
	echo "✗ No device plugin pods found"
	exit 1
fi

echo "✓ Device plugin pods running:"
oc get pods -n nvidia-gpu-operator -l app=nvidia-device-plugin-daemonset -o wide

echo ""
echo "=== Summary ==="
echo ""

# Get physical GPU count from node labels, not capacity
TOTAL_PHYSICAL_GPUS=$(oc get nodes -l nvidia.com/gpu.present=true -o json | jq -r '.items[].metadata.labels."nvidia.com/gpu.count" // "1"' | awk '{sum+=$1} END {print sum}')
TOTAL_ALLOCATABLE_GPUS=$(oc get nodes -l nvidia.com/gpu.present=true -o json | jq -r '.items[].status.allocatable."nvidia.com/gpu"' | awk '{sum+=$1} END {print sum}')

echo "Physical GPUs:     $TOTAL_PHYSICAL_GPUS"
echo "Allocatable GPUs:  $TOTAL_ALLOCATABLE_GPUS"

if [ "$TOTAL_ALLOCATABLE_GPUS" -gt "$TOTAL_PHYSICAL_GPUS" ]; then
	REPLICAS=$((TOTAL_ALLOCATABLE_GPUS / TOTAL_PHYSICAL_GPUS))
	echo ""
	echo "✓ Time-slicing is ACTIVE with ${REPLICAS}x replication"
elif [ "$TOTAL_ALLOCATABLE_GPUS" -eq "$TOTAL_PHYSICAL_GPUS" ]; then
	# Check if any node has time-slicing configured
	TIME_SLICING_NODES=$(oc get nodes -l nvidia.com/gpu.sharing-strategy=time-slicing -o name 2>/dev/null | wc -l)
	if [ "$TIME_SLICING_NODES" -gt 0 ]; then
		echo ""
		echo "✓ Time-slicing is configured (may need time for device plugin to update)"
	else
		echo ""
		echo "⚠ Time-slicing may not be active"
	fi
else
	echo ""
	echo "⚠ Unexpected: allocatable < physical"
fi

echo ""
echo "To test GPU allocation:"
echo "  oc run gpu-test --image=nvidia/cuda:12.2.0-base-ubuntu22.04 --limits=nvidia.com/gpu=1 --command -- sleep infinity"
echo "  oc exec gpu-test -- nvidia-smi"
echo ""
