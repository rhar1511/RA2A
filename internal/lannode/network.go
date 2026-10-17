package lannode

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// NetworkConfig optionally supplies stable endpoints across networks without mDNS.
// Zero values retain ephemeral listening and ordinary LAN discovery.
type NetworkConfig struct {
	ListenPort int    `json:"listenPort,omitempty"`
	Peers      []Peer `json:"peers,omitempty"`
}

func (config NetworkConfig) Validate(localID string) error {
	if config.ListenPort < 0 || config.ListenPort > 65535 {
		return fmt.Errorf("listenPort must be between 0 and 65535")
	}
	seen := map[string]bool{}
	for _, peer := range config.Peers {
		if peer.ID == "" || strings.ContainsAny(peer.ID, "/?# \t\r\n") || peer.ID == localID || seen[peer.ID] {
			return fmt.Errorf("configured peer IDs must be unique, nonempty and different from this node")
		}
		host, port, err := net.SplitHostPort(peer.Address)
		value, portErr := strconv.Atoi(port)
		if err != nil || host == "" || strings.ContainsAny(host, "/ \t\r\n") || portErr != nil || value < 1 || value > 65535 {
			return fmt.Errorf("configured peer %q requires a host and numeric UDP port", peer.ID)
		}
		if ip := net.ParseIP(host); ip != nil && ip.To4() == nil {
			return fmt.Errorf("configured peer %q requires an IPv4 address or hostname", peer.ID)
		}
		seen[peer.ID] = true
	}
	return nil
}
