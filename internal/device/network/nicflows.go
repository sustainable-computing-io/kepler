// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package network

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

const nicFlowsFile = "nic_flows.json"

// NICFlow represents a single flow captured by the eBPF program. The program
// only emits entries for traffic that actually traversed the physical NIC,
// correlated with the bridge hook that caught the same flow pre-NAT.
//
//   - src_ip / dst_ip        : addresses seen on the physical NIC (post-SNAT)
//   - src_port / dst_port    : ports seen on the physical NIC (post-SNAT)
//   - bridge_src_ip / _dst_ip: addresses seen at the bridge (pre-SNAT),
//     typically the Kind node IP or pod IP
//   - bridge_src_port / _dst_port: ports seen at the bridge (pre-SNAT),
//     critical for disambiguating pods on the same Kind node since the
//     source port uniquely identifies a connection per pod
type NICFlow struct {
	Direction     string `json:"direction"` // "TX" or "RX"
	SrcIP         string `json:"src_ip"`
	DstIP         string `json:"dst_ip"`
	SrcPort       uint16 `json:"src_port"`
	DstPort       uint16 `json:"dst_port"`
	BridgeSrcIP   string `json:"bridge_src_ip,omitempty"`
	BridgeDstIP   string `json:"bridge_dst_ip,omitempty"`
	BridgeSrcPort uint16 `json:"bridge_src_port,omitempty"`
	BridgeDstPort uint16 `json:"bridge_dst_port,omitempty"`
	BridgePackets uint64 `json:"bridge_packets,omitempty"`
	BridgeBytes   uint64 `json:"bridge_bytes,omitempty"`
	Packets       uint64 `json:"packets"`
	Bytes         uint64 `json:"bytes"`
}

// NICFlowsData is the top-level structure written by the eBPF NIC program
// to nic_flows.json.
type NICFlowsData struct {
	Timestamp string    `json:"timestamp"`
	Flows     []NICFlow `json:"flows"`
}

// ReadNICFlows reads and parses the nic_flows.json file from the given
// powercap directory.
func ReadNICFlows(dir string) (*NICFlowsData, error) {
	if dir == "" {
		dir = defaultOutputDir
	}
	path := filepath.Join(dir, nicFlowsFile)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read nic flows: %w", err)
	}

	var fd NICFlowsData
	if err := json.Unmarshal(data, &fd); err != nil {
		return nil, fmt.Errorf("parse nic flows: %w", err)
	}

	return &fd, nil
}

// PodNICUsage holds the aggregated NIC bytes attributed to a single pod.
type PodNICUsage struct {
	PodIP   string
	TxBytes uint64
	RxBytes uint64
}

// flowKey is the (ip, port) tuple used to join eBPF bridge fields with
// conntrack NAT entries. The port is essential when multiple pods on the
// same node SNAT to a shared node IP — each connection has a unique port.
type flowKey struct {
	IP   string
	Port string
}

// AttributeFlowsToPods resolves NIC flows to original pod IPs using
// (bridge_ip, bridge_port) keys looked up in conntrack on this node.
//
//	eBPF NIC flow (bridge_src_ip:bridge_src_port)
//	     │
//	     ▼ conntrack lookup: node_ip:node_port → original pod_ip
//	     │
//	     ▼
//	     pod_ip
//
// Flows that don't resolve via local conntrack are skipped — they either
// belong to another node (the shared nic_flows.json mixes flows from all
// Kind nodes) or their conntrack entry has expired.
func AttributeFlowsToPods(flows []NICFlow, natEntries []NATEntry) map[string]*PodNICUsage {
	// Build (node_ip, node_port) → pod_ip lookup from conntrack.
	// Port-scoped to disambiguate multiple pods sharing a node IP.
	nodeKeyToPod := make(map[flowKey]string, len(natEntries))
	for _, e := range natEntries {
		nodeKeyToPod[flowKey{IP: e.NodeIP, Port: e.NodePort}] = e.PodIP
	}

	usage := make(map[string]*PodNICUsage)

	for i := range flows {
		f := &flows[i]

		podIP := resolvePodIP(f, nodeKeyToPod)
		if podIP == "" {
			continue
		}

		u, exists := usage[podIP]
		if !exists {
			u = &PodNICUsage{PodIP: podIP}
			usage[podIP] = u
		}

		switch f.Direction {
		case "TX":
			u.TxBytes += f.Bytes
		case "RX":
			u.RxBytes += f.Bytes
		}
	}

	return usage
}

// resolvePodIP determines the original pod IP for a NIC flow by looking up
// (bridge_ip, bridge_port) in the conntrack NAT table on the node where
// Kepler is running.
//
// Only flows that resolve to a pod via conntrack on THIS node are attributed.
// Flows whose (bridge_ip, bridge_port) isn't in local conntrack are skipped —
// they belong to another node's pod (shared nic_flows.json) or the conntrack
// entry has expired.
//
// The port disambiguates pods that SNAT to the same node IP; ignoring it
// causes misattribution when multiple pods share a Kind node IP.
func resolvePodIP(f *NICFlow, nodeKeyToPod map[flowKey]string) string {
	var bridgeIP string
	var bridgePort uint16

	switch f.Direction {
	case "TX":
		bridgeIP = f.BridgeSrcIP
		bridgePort = f.BridgeSrcPort
	case "RX":
		bridgeIP = f.BridgeDstIP
		bridgePort = f.BridgeDstPort
	default:
		return ""
	}

	if bridgeIP == "" {
		return ""
	}

	portStr := strconv.FormatUint(uint64(bridgePort), 10)
	return nodeKeyToPod[flowKey{IP: bridgeIP, Port: portStr}]
}
