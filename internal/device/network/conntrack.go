// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package network

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"syscall"

	"github.com/mdlayher/netlink"
	ct "github.com/ti-mo/conntrack"
)

// Protocol numbers (IANA).
const (
	protoTCP  = syscall.IPPROTO_TCP
	protoUDP  = syscall.IPPROTO_UDP
	protoSCTP = syscall.IPPROTO_SCTP
)

// NATEntry represents a single conntrack NAT flow mapping a pod IP to the
// node's external (SNAT'd) IP.
type NATEntry struct {
	PodIP      string
	RemoteIP   string
	PodPort    string
	RemotePort string
	NodeIP     string
	NodePort   string
	Protocol   string
	State      string
}

// conntrackDialer abstracts conntrack connection creation for testing.
type conntrackDialer func() (conntrackConn, error)

// conntrackConn abstracts the conntrack connection interface for testing.
type conntrackConn interface {
	Dump(opts *ct.DumpOptions) ([]ct.Flow, error)
	Close() error
}

// ConntrackReader reads NAT entries from the kernel conntrack table via
// netlink. This avoids forking external processes (nsenter/conntrack) and
// works from any privileged pod with CAP_NET_ADMIN.
type ConntrackReader struct {
	mu      sync.RWMutex
	entries []NATEntry
	dial    conntrackDialer
}

// NewConntrackReader creates a new ConntrackReader that queries conntrack
// via netlink.
func NewConntrackReader() *ConntrackReader {
	return &ConntrackReader{
		dial: defaultDial,
	}
}

// hostNetNS is the path to the host's network namespace. Kepler runs with
// hostPID: true so PID 1 belongs to the host.
const hostNetNS = "/proc/1/ns/net"

func defaultDial() (conntrackConn, error) {
	f, err := os.Open(hostNetNS)
	if err != nil {
		// Fallback: if we can't open the host netns (e.g. in tests),
		// dial in the current namespace.
		return ct.Dial(nil)
	}
	defer f.Close()

	return ct.Dial(&netlink.Config{
		NetNS: int(f.Fd()),
	})
}

// Refresh dumps the kernel conntrack table via netlink and caches the
// NAT entries.
func (r *ConntrackReader) Refresh() error {
	conn, err := r.dial()
	if err != nil {
		return fmt.Errorf("conntrack dial: %w", err)
	}
	defer conn.Close()

	flows, err := conn.Dump(nil)
	if err != nil {
		return fmt.Errorf("conntrack dump: %w", err)
	}

	entries := extractNATEntries(flows)

	r.mu.Lock()
	r.entries = entries
	r.mu.Unlock()
	return nil
}

// NATEntries returns the most recently cached NAT entries.
func (r *ConntrackReader) NATEntries() []NATEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]NATEntry, len(r.entries))
	copy(out, r.entries)
	return out
}

// PodNATMap returns a map of pod IP to all its NAT entries.
func (r *ConntrackReader) PodNATMap() map[string][]NATEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	m := make(map[string][]NATEntry, len(r.entries))
	for _, e := range r.entries {
		m[e.PodIP] = append(m[e.PodIP], e)
	}
	return m
}

// extractNATEntries filters conntrack flows to only those where SNAT
// occurred (original source != reply destination).
func extractNATEntries(flows []ct.Flow) []NATEntry {
	entries := make([]NATEntry, 0, len(flows)/4) // most flows are not NAT'd

	for i := range flows {
		f := &flows[i]

		origSrc := f.TupleOrig.IP.SourceAddress
		replyDst := f.TupleReply.IP.DestinationAddress

		// NAT happened if the original source differs from the reply destination.
		if origSrc == replyDst {
			continue
		}

		entries = append(entries, NATEntry{
			PodIP:      origSrc.String(),
			RemoteIP:   f.TupleOrig.IP.DestinationAddress.String(),
			PodPort:    strconv.FormatUint(uint64(f.TupleOrig.Proto.SourcePort), 10),
			RemotePort: strconv.FormatUint(uint64(f.TupleOrig.Proto.DestinationPort), 10),
			NodeIP:     replyDst.String(),
			NodePort:   strconv.FormatUint(uint64(f.TupleReply.Proto.DestinationPort), 10),
			Protocol:   protoName(f.TupleOrig.Proto.Protocol),
			State:      protoInfoState(f),
		})
	}

	return entries
}

func protoName(p uint8) string {
	switch p {
	case protoTCP:
		return "tcp"
	case protoUDP:
		return "udp"
	case protoSCTP:
		return "sctp"
	default:
		return strconv.Itoa(int(p))
	}
}

// TCP conntrack states (from linux/netfilter/nf_conntrack_tcp.h).
var tcpStateNames = [...]string{
	0: "NONE",
	1: "SYN_SENT",
	2: "SYN_RECV",
	3: "ESTABLISHED",
	4: "FIN_WAIT",
	5: "CLOSE_WAIT",
	6: "LAST_ACK",
	7: "TIME_WAIT",
	8: "CLOSE",
	9: "LISTEN",
}

func protoInfoState(f *ct.Flow) string {
	if f.ProtoInfo.TCP == nil {
		return ""
	}
	s := f.ProtoInfo.TCP.State
	if int(s) < len(tcpStateNames) {
		return tcpStateNames[s]
	}
	return strconv.Itoa(int(s))
}
