package lannode

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/betamos/zeroconf"
)

func TestNetworkValidation(t *testing.T) {
	for _, config := range []NetworkConfig{
		{ListenPort: -1}, {ListenPort: 65536},
		{Peers: []Peer{{ID: "local", Address: "host:1234"}}},
		{Peers: []Peer{{Address: "host:1234"}}},
		{Peers: []Peer{{ID: "bad/endpoint", Address: "host:1234"}}},
		{Peers: []Peer{{ID: "remote", Address: "host:0"}}},
		{Peers: []Peer{{ID: "remote", Address: "host:65536"}}},
		{Peers: []Peer{{ID: "remote", Address: "host:not-a-port"}}},
		{Peers: []Peer{{ID: "remote", Address: ":1234"}}},
		{Peers: []Peer{{ID: "remote", Address: "https://host:1234"}}},
		{Peers: []Peer{{ID: "remote", Address: "[::1]:1234"}}},
		{Peers: []Peer{{ID: "remote", Address: "one:1234"}, {ID: "remote", Address: "two:1234"}}},
	} {
		if err := config.Validate("local"); err == nil {
			t.Errorf("accepted invalid config: %+v", config)
		}
	}
	for _, config := range []NetworkConfig{{}, {ListenPort: 47322, Peers: []Peer{{ID: "remote", Address: "remote.example.ts.net:47322"}}}} {
		if err := config.Validate("local"); err != nil {
			t.Fatal(err)
		}
	}
}

func TestConfiguredPeerConnectsWithoutDiscoveryAndRejectsWrongPIN(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	// Reserve a free UDP port, then prove the node binds that exact configured port.
	reservation, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := reservation.LocalAddr().(*net.UDPAddr).Port
	reservation.Close()
	received := make(chan Message, 1)
	server, err := Start(ctx, Config{ID: "configured-server", Name: "Server", PIN: "A2B3C4",
		Network:     NetworkConfig{ListenPort: port},
		Sessions:    func(context.Context) ([]Session, error) { return []Session{{ID: "fixture", Status: "idle"}}, nil },
		SendMessage: func(_ context.Context, message Message) error { received <- message; return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	if got := server.bindings.port; got != port {
		t.Fatalf("listener port = %d; want %d", got, port)
	}
	peer := Peer{ID: "configured-server", Address: fmt.Sprintf("127.0.0.1:%d", port)}
	client, err := Start(ctx, Config{ID: "configured-client", Name: "Client", PIN: "A2B3C4", Network: NetworkConfig{Peers: []Peer{peer}}})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	// A LAN advertisement or goodbye must never replace an explicit tailnet endpoint.
	client.handlePeerEvent(zeroconf.Event{Op: zeroconf.OpAdded, Service: &zeroconf.Service{Name: peer.ID, Port: 1}})
	client.handlePeerEvent(zeroconf.Event{Op: zeroconf.OpRemoved, Service: &zeroconf.Service{Name: peer.ID}})
	actual, ok := client.Peer(peer.ID)
	if !ok || actual.Address != peer.Address || actual.Name != peer.ID {
		t.Fatalf("configured peer was removed or changed: %+v", actual)
	}
	sessions, err := client.ListSessions(ctx, actual)
	if err != nil || len(sessions) != 1 || sessions[0].ID != "fixture" {
		t.Fatalf("configured sessions = %+v, %v", sessions, err)
	}
	message := Message{TargetSessionID: "fixture", Text: "fixture-only", Source: "ra2a://configured-client/fixture"}
	if err := client.SendMessage(ctx, actual, message); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-received:
		if got != message {
			t.Fatalf("message = %+v", got)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	client.key = []byte("WRONG1")
	if _, err := client.ListSessions(ctx, actual); err == nil {
		t.Fatal("configured endpoint bypassed PIN authentication")
	}
}
