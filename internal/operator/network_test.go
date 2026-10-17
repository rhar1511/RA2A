package operator

import (
	"reflect"
	"testing"

	"github.com/ceasarXuu/RA2A/internal/lannode"
)

func TestOptionalNetworkConfigurationSurvivesSaveAndLoad(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	config := Config{NodeID: "local", Name: "Local", PIN: "A2B3C4", Codex: "fixture-codex",
		Network: &lannode.NetworkConfig{ListenPort: 47322, Peers: []lannode.Peer{{ID: "remote", Address: "remote.example.ts.net:47322"}}}}
	if err := Save(config); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, config) {
		t.Fatalf("configuration changed: %+v", loaded.Network)
	}
	loaded.Network.Peers[0].ID = "local"
	if err := Save(loaded); err == nil {
		t.Fatal("saved self-referencing peer")
	}
}
