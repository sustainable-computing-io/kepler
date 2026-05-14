// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package network

import (
	"net/netip"
	"testing"

	ct "github.com/ti-mo/conntrack"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildFlow is a test helper that creates a conntrack Flow with the given tuples.
func buildFlow(proto uint8, origSrc, origDst string, origSPort, origDPort uint16,
	replySrc, replyDst string, replySPort, replyDPort uint16, tcpState *uint8,
) ct.Flow {
	f := ct.Flow{
		TupleOrig: ct.Tuple{
			IP: ct.IPTuple{
				SourceAddress:      netip.MustParseAddr(origSrc),
				DestinationAddress: netip.MustParseAddr(origDst),
			},
			Proto: ct.ProtoTuple{
				Protocol:        proto,
				SourcePort:      origSPort,
				DestinationPort: origDPort,
			},
		},
		TupleReply: ct.Tuple{
			IP: ct.IPTuple{
				SourceAddress:      netip.MustParseAddr(replySrc),
				DestinationAddress: netip.MustParseAddr(replyDst),
			},
			Proto: ct.ProtoTuple{
				Protocol:        proto,
				SourcePort:      replySPort,
				DestinationPort: replyDPort,
			},
		},
	}
	if tcpState != nil {
		f.ProtoInfo.TCP = &ct.ProtoInfoTCP{State: *tcpState}
	}
	return f
}

func tcpState(s uint8) *uint8 { return &s }

func TestExtractNATEntries(t *testing.T) {
	flows := []ct.Flow{
		// Non-NAT: origSrc == replyDst (pod-to-pod, no SNAT)
		buildFlow(6, "10.244.1.1", "10.244.1.3", 45990, 9093,
			"10.244.1.3", "10.244.1.1", 9093, 45990, tcpState(7)),

		// NAT: pod 10.244.0.8 -> 8.8.8.8, SNAT'd to node 172.18.0.3
		buildFlow(6, "10.244.0.8", "8.8.8.8", 41818, 443,
			"8.8.8.8", "172.18.0.3", 443, 41818, tcpState(3)),

		// NAT: pod 10.244.2.5 -> 1.1.1.1 (UDP DNS), SNAT'd to node 172.18.0.3
		buildFlow(17, "10.244.2.5", "1.1.1.1", 12345, 53,
			"1.1.1.1", "172.18.0.3", 53, 12345, nil),

		// Non-NAT: localhost
		buildFlow(6, "127.0.0.1", "127.0.0.1", 40482, 2381,
			"127.0.0.1", "127.0.0.1", 2381, 40482, tcpState(7)),
	}

	entries := extractNATEntries(flows)
	require.Len(t, entries, 2)

	assert.Equal(t, NATEntry{
		PodIP: "10.244.0.8", RemoteIP: "8.8.8.8",
		PodPort: "41818", RemotePort: "443",
		NodeIP: "172.18.0.3", NodePort: "41818",
		Protocol: "tcp", State: "ESTABLISHED",
	}, entries[0])

	assert.Equal(t, NATEntry{
		PodIP: "10.244.2.5", RemoteIP: "1.1.1.1",
		PodPort: "12345", RemotePort: "53",
		NodeIP: "172.18.0.3", NodePort: "12345",
		Protocol: "udp", State: "",
	}, entries[1])
}

func TestExtractNATEntries_NoNAT(t *testing.T) {
	flows := []ct.Flow{
		buildFlow(6, "10.244.1.1", "10.244.1.3", 100, 200,
			"10.244.1.3", "10.244.1.1", 200, 100, tcpState(3)),
	}
	assert.Empty(t, extractNATEntries(flows))
}

func TestExtractNATEntries_Empty(t *testing.T) {
	assert.Empty(t, extractNATEntries(nil))
}

// mockConn implements conntrackConn for testing.
type mockConn struct {
	flows []ct.Flow
	err   error
}

func (m *mockConn) Dump(_ *ct.DumpOptions) ([]ct.Flow, error) {
	return m.flows, m.err
}

func (m *mockConn) Close() error { return nil }

func TestConntrackReader_Refresh(t *testing.T) {
	flows := []ct.Flow{
		buildFlow(6, "10.244.0.8", "8.8.8.8", 41818, 443,
			"8.8.8.8", "172.18.0.3", 443, 41818, tcpState(3)),
	}

	r := &ConntrackReader{
		dial: func() (conntrackConn, error) {
			return &mockConn{flows: flows}, nil
		},
	}

	require.NoError(t, r.Refresh())
	entries := r.NATEntries()
	require.Len(t, entries, 1)
	assert.Equal(t, "10.244.0.8", entries[0].PodIP)
	assert.Equal(t, "172.18.0.3", entries[0].NodeIP)
}

func TestConntrackReader_PodNATMap(t *testing.T) {
	flows := []ct.Flow{
		buildFlow(6, "10.244.0.8", "8.8.8.8", 41818, 443,
			"8.8.8.8", "172.18.0.3", 443, 41818, tcpState(3)),
		buildFlow(17, "10.244.2.5", "1.1.1.1", 12345, 53,
			"1.1.1.1", "172.18.0.3", 53, 12345, nil),
		buildFlow(6, "10.244.0.8", "9.9.9.9", 50000, 80,
			"9.9.9.9", "172.18.0.3", 80, 50000, tcpState(3)),
	}

	r := &ConntrackReader{
		dial: func() (conntrackConn, error) {
			return &mockConn{flows: flows}, nil
		},
	}

	require.NoError(t, r.Refresh())
	m := r.PodNATMap()
	require.Len(t, m, 2)
	assert.Len(t, m["10.244.0.8"], 2)
	assert.Len(t, m["10.244.2.5"], 1)
}

func TestConntrackReader_NATEntries_Copy(t *testing.T) {
	r := NewConntrackReader()
	r.mu.Lock()
	r.entries = []NATEntry{{PodIP: "10.0.0.1", NodeIP: "192.168.1.1"}}
	r.mu.Unlock()

	entries := r.NATEntries()
	entries[0].PodIP = "changed"
	assert.Equal(t, "10.0.0.1", r.NATEntries()[0].PodIP)
}

func TestProtoName(t *testing.T) {
	assert.Equal(t, "tcp", protoName(6))
	assert.Equal(t, "udp", protoName(17))
	assert.Equal(t, "sctp", protoName(132))
	assert.Equal(t, "41", protoName(41))
}

func TestProtoInfoState(t *testing.T) {
	established := ct.Flow{}
	established.ProtoInfo.TCP = &ct.ProtoInfoTCP{State: 3}
	assert.Equal(t, "ESTABLISHED", protoInfoState(&established))

	timeWait := ct.Flow{}
	timeWait.ProtoInfo.TCP = &ct.ProtoInfoTCP{State: 7}
	assert.Equal(t, "TIME_WAIT", protoInfoState(&timeWait))

	udpFlow := ct.Flow{}
	assert.Equal(t, "", protoInfoState(&udpFlow))
}
