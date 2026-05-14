// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package network

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testNICFlowsJSON = `{
  "timestamp": "2026-04-20T15:30:40+02:00",
  "flows": [
    {
      "direction": "TX",
      "src_ip": "192.168.202.132",
      "dst_ip": "163.70.128.35",
      "src_port": 43798,
      "dst_port": 443,
      "bridge_src_ip": "172.18.0.4",
      "bridge_dst_ip": "163.70.128.35",
      "bridge_src_port": 43304,
      "bridge_dst_port": 443,
      "bridge_packets": 45,
      "bridge_bytes": 65340,
      "packets": 45,
      "bytes": 65340
    },
    {
      "direction": "RX",
      "src_ip": "163.70.128.35",
      "dst_ip": "192.168.202.132",
      "src_port": 443,
      "dst_port": 43798,
      "bridge_src_ip": "163.70.128.35",
      "bridge_dst_ip": "172.18.0.4",
      "bridge_src_port": 443,
      "bridge_dst_port": 43304,
      "packets": 127,
      "bytes": 411663
    }
  ]
}`

func TestReadNICFlows(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, nicFlowsFile), []byte(testNICFlowsJSON), 0o644))

	fd, err := ReadNICFlows(dir)
	require.NoError(t, err)

	require.Len(t, fd.Flows, 2)
	assert.Equal(t, "TX", fd.Flows[0].Direction)
	assert.Equal(t, uint16(43304), fd.Flows[0].BridgeSrcPort)
	assert.Equal(t, uint64(65340), fd.Flows[0].Bytes)
}

func TestReadNICFlows_Missing(t *testing.T) {
	_, err := ReadNICFlows(t.TempDir())
	assert.Error(t, err)
}

func TestAttributeFlowsToPods(t *testing.T) {
	// Flow: Pod A (10.244.0.8) → internet
	// eBPF bridge fields: bridge_src_ip=172.18.0.4, bridge_src_port=43304
	// Conntrack: (172.18.0.4, 43304) → pod 10.244.0.8
	flows := []NICFlow{
		// Pod A TX
		{Direction: "TX", SrcIP: "192.168.202.132", DstIP: "163.70.128.35",
			SrcPort: 43798, DstPort: 443,
			BridgeSrcIP: "172.18.0.4", BridgeSrcPort: 43304,
			BridgeDstIP: "163.70.128.35", BridgeDstPort: 443,
			Bytes: 5000},
		// Pod A RX (reply — bridge_dst_port is pod's original src port)
		{Direction: "RX", SrcIP: "163.70.128.35", DstIP: "192.168.202.132",
			SrcPort: 443, DstPort: 43798,
			BridgeSrcIP: "163.70.128.35", BridgeSrcPort: 443,
			BridgeDstIP: "172.18.0.4", BridgeDstPort: 43304,
			Bytes: 82000},
		// Pod B on same node IP but different port
		{Direction: "TX", SrcIP: "192.168.202.132", DstIP: "1.1.1.1",
			SrcPort: 12345, DstPort: 53,
			BridgeSrcIP: "172.18.0.4", BridgeSrcPort: 50001,
			Bytes: 100},
	}

	natEntries := []NATEntry{
		// Pod A: 172.18.0.4:43304 → 10.244.0.8
		{PodIP: "10.244.0.8", NodeIP: "172.18.0.4", NodePort: "43304"},
		// Pod B: 172.18.0.4:50001 → 10.244.2.5
		{PodIP: "10.244.2.5", NodeIP: "172.18.0.4", NodePort: "50001"},
	}

	usage := AttributeFlowsToPods(flows, natEntries)

	require.Len(t, usage, 2)
	assert.Equal(t, uint64(5000), usage["10.244.0.8"].TxBytes)
	assert.Equal(t, uint64(82000), usage["10.244.0.8"].RxBytes)
	assert.Equal(t, uint64(100), usage["10.244.2.5"].TxBytes)
}

func TestAttributeFlowsToPods_NoBridgeFields(t *testing.T) {
	flows := []NICFlow{
		{Direction: "TX", SrcIP: "192.168.1.1", DstIP: "8.8.8.8", Bytes: 100},
	}
	usage := AttributeFlowsToPods(flows, nil)
	assert.Empty(t, usage)
}

func TestAttributeFlowsToPods_NoConntrackMatch(t *testing.T) {
	// Bridge IP exists but conntrack has no matching entry — skipped.
	// This is the "another node's pod" case: shared nic_flows.json has
	// flows from other Kind nodes, and we must not attribute them here.
	flows := []NICFlow{
		{Direction: "TX", SrcIP: "192.168.1.1", DstIP: "8.8.8.8",
			BridgeSrcIP: "10.244.0.99", BridgeSrcPort: 9999,
			Bytes: 100},
	}
	usage := AttributeFlowsToPods(flows, nil)
	assert.Empty(t, usage)
}

func TestAttributeFlowsToPods_Empty(t *testing.T) {
	assert.Empty(t, AttributeFlowsToPods(nil, nil))
}

func TestAttributeFlowsToPods_MultiplePortsSameNodeIP(t *testing.T) {
	// Two different pods on the same Kind node — same bridge_src_ip but
	// different bridge_src_port. Port disambiguation is the key feature.
	flows := []NICFlow{
		{Direction: "TX", BridgeSrcIP: "172.18.0.4", BridgeSrcPort: 10000, Bytes: 1000},
		{Direction: "TX", BridgeSrcIP: "172.18.0.4", BridgeSrcPort: 20000, Bytes: 2000},
	}
	natEntries := []NATEntry{
		{PodIP: "10.244.0.8", NodeIP: "172.18.0.4", NodePort: "10000"},
		{PodIP: "10.244.1.9", NodeIP: "172.18.0.4", NodePort: "20000"},
	}

	usage := AttributeFlowsToPods(flows, natEntries)
	require.Len(t, usage, 2)
	assert.Equal(t, uint64(1000), usage["10.244.0.8"].TxBytes)
	assert.Equal(t, uint64(2000), usage["10.244.1.9"].TxBytes)
}

func TestAttributeFlowsToPods_EnergyAttribution(t *testing.T) {
	// 60/40 byte split between two pods on different node IPs
	flows := []NICFlow{
		{Direction: "TX", BridgeSrcIP: "172.18.0.4", BridgeSrcPort: 43304, Bytes: 40000},
		{Direction: "RX", BridgeDstIP: "172.18.0.4", BridgeDstPort: 43304, Bytes: 20000},
		{Direction: "TX", BridgeSrcIP: "172.18.0.5", BridgeSrcPort: 55555, Bytes: 30000},
		{Direction: "RX", BridgeDstIP: "172.18.0.5", BridgeDstPort: 55555, Bytes: 10000},
	}
	natEntries := []NATEntry{
		{PodIP: "10.244.0.8", NodeIP: "172.18.0.4", NodePort: "43304"},
		{PodIP: "10.244.2.5", NodeIP: "172.18.0.5", NodePort: "55555"},
	}

	usage := AttributeFlowsToPods(flows, natEntries)

	var totalBytes uint64
	for _, u := range usage {
		totalBytes += u.TxBytes + u.RxBytes
	}
	assert.Equal(t, uint64(100000), totalBytes)

	podABytes := usage["10.244.0.8"].TxBytes + usage["10.244.0.8"].RxBytes
	assert.Equal(t, uint64(60000), podABytes)
	assert.InDelta(t, 0.6, float64(podABytes)/float64(totalBytes), 0.001)

	podBBytes := usage["10.244.2.5"].TxBytes + usage["10.244.2.5"].RxBytes
	assert.Equal(t, uint64(40000), podBBytes)
	assert.InDelta(t, 0.4, float64(podBBytes)/float64(totalBytes), 0.001)
}
