package lannode

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"

	"github.com/plgd-dev/go-coap/v3/message"
)

const sessionCompressionQuery = "encoding=gzip"
const maxSessionResponseBytes = 16 << 20

// Compression is requested explicitly; old clients retain the JSON response and
// new clients can still read JSON returned by peers that ignore the query.
func encodeSessions(sessions []Session, compressed bool) ([]byte, message.MediaType, error) {
	payload, err := json.Marshal(sessionsResponse{ProtocolVersion: ProtocolVersion, Sessions: sessions})
	if err != nil || !compressed || len(payload) < 1024 {
		return payload, message.AppJSON, err
	}
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	if _, err := writer.Write(payload); err != nil {
		return nil, message.AppJSON, err
	}
	if err := writer.Close(); err != nil {
		return nil, message.AppJSON, err
	}
	return buffer.Bytes(), message.AppOctets, nil
}

func decodeSessions(body io.Reader, format message.MediaType) ([]Session, error) {
	switch format {
	case message.AppJSON:
	case message.AppOctets:
		reader, err := gzip.NewReader(body)
		if err != nil {
			return nil, fmt.Errorf("open compressed session response: %w", err)
		}
		defer reader.Close()
		body = reader
	default:
		return nil, fmt.Errorf("unsupported session response format %v", format)
	}
	payload, err := io.ReadAll(io.LimitReader(body, maxSessionResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(payload) > maxSessionResponseBytes {
		return nil, fmt.Errorf("session response exceeds %d bytes", maxSessionResponseBytes)
	}
	var decoded sessionsResponse
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return nil, err
	}
	return decoded.Sessions, nil
}
