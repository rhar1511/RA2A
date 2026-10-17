package lannode

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/plgd-dev/go-coap/v3/message"
)

func TestLargeSessionDirectoryAndLegacyClient(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	want := make([]Session, 6200)
	for i := range want {
		want[i] = Session{ID: fmt.Sprintf("thread-%d", i), Title: strings.Repeat("historical session ", 8), Status: "idle"}
	}
	node, err := Start(ctx, Config{ID: "large-directory", Name: "Directory", PIN: "A2B3C4",
		Sessions: func(context.Context) ([]Session, error) { return want, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer node.Close()
	peer, _ := node.Peer("large-directory")
	got, err := node.ListSessions(ctx, peer)
	if err != nil || len(got) != len(want) {
		t.Fatalf("directory count=%d err=%v", len(got), err)
	}
	if !reflect.DeepEqual(got[6199], want[6199]) {
		t.Fatal("directory truncated or changed")
	}

	// A v0.0.18 client does not negotiate compression and must still get JSON.
	client, err := node.dialPeer(ctx, peer)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	compressed, err := client.Get(ctx, "/v1/sessions", message.Option{ID: message.URIQuery, Value: []byte(sessionCompressionQuery)})
	if err != nil {
		t.Fatal(err)
	}
	compressedFormat, _ := compressed.ContentFormat()
	wire, readErr := io.ReadAll(compressed.Body())
	client.ReleaseMessage(compressed)
	if readErr != nil || compressedFormat != message.AppOctets {
		t.Fatalf("negotiated format=%v err=%v", compressedFormat, readErr)
	}
	plain, _, err := encodeSessions(want, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(wire) >= len(plain)/10 {
		t.Fatalf("directory not compressed enough: wire=%d plain=%d", len(wire), len(plain))
	}
	response, err := client.Get(ctx, "/v1/sessions")
	if err != nil {
		t.Fatal(err)
	}
	defer client.ReleaseMessage(response)
	format, _ := response.ContentFormat()
	if format != message.AppJSON {
		t.Fatalf("legacy format=%v", format)
	}
	var legacy sessionsResponse
	if err := json.NewDecoder(response.Body()).Decode(&legacy); err != nil {
		t.Fatal(err)
	}
	if len(legacy.Sessions) != len(want) {
		t.Fatal("legacy directory changed")
	}
}

func TestSessionDecoderRejectsCorruptionAndExcessiveExpansion(t *testing.T) {
	var oversized bytes.Buffer
	writer := gzip.NewWriter(&oversized)
	if _, err := writer.Write([]byte(strings.Repeat(" ", maxSessionResponseBytes+1))); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	for _, payload := range [][]byte{[]byte("not gzip"), oversized.Bytes()} {
		if _, err := decodeSessions(bytes.NewReader(payload), message.AppOctets); err == nil {
			t.Fatal("invalid compressed response accepted")
		}
	}
	valid, _, err := encodeSessions([]Session{{ID: "legacy", Status: "idle"}}, false)
	if err != nil {
		t.Fatal(err)
	}
	got, err := decodeSessions(bytes.NewReader(valid), message.AppJSON)
	if err != nil || len(got) != 1 || got[0].ID != "legacy" {
		t.Fatalf("legacy server decode=%v %v", got, err)
	}
}
