package speech

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHTTPClientStreamsBoundedPCM verifies segmented provider requests and incremental media output.
func TestHTTPClientStreamsBoundedPCM(t *testing.T) {
	var requested []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/synthesize" || r.Method != http.MethodPost || !strings.Contains(r.Header.Get("Accept"), pcmStreamMediaType) {
			t.Errorf("unexpected provider request: %s %s", r.Method, r.URL.Path)
			return
		}
		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode provider payload: %v", err)
			return
		}
		requested = append(requested, payload["text"])
		if payload["voice"] != "en_US-lessac-high" {
			t.Errorf("unexpected provider payload: %#v", payload)
			return
		}
		w.Header().Set("Content-Type", pcmStreamMediaType)
		w.Header().Set("X-Patchflow-Sample-Rate", "24000")
		w.Header().Set("X-Patchflow-Channels", "1")
		w.Header().Set("X-Patchflow-Bits-Per-Sample", "16")
		writeTestSpeechFrame(w, []byte{byte(len(requested)), 0})
		writeTestSpeechFrame(w, []byte{byte(len(requested) + 10), 0})
		_ = binary.Write(w, binary.LittleEndian, uint32(0))
	}))
	defer server.Close()

	client, err := NewHTTPClient(server.URL+"/api", "en_US-lessac-high", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	var chunks []Chunk
	err = client.Stream(context.Background(), "Explain the boundary. Then inspect the adapter.", func(chunk Chunk) error {
		chunks = append(chunks, chunk)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 4 || len(requested) != 2 || requested[0] != "Explain the boundary." || requested[1] != "Then inspect the adapter." {
		t.Fatalf("unexpected stream: requests=%#v chunks=%#v", requested, chunks)
	}
	if chunks[0].SampleRate != 24_000 || chunks[0].Channels != 1 || chunks[0].BitsPerSample != 16 || !bytes.Equal(chunks[3].Data, []byte{12, 0}) {
		t.Fatalf("unexpected PCM format: %#v", chunks)
	}
}

// TestHTTPClientRejectsUnsafeConfigurationAndResponses protects the local proxy boundary.
func TestHTTPClientRejectsUnsafeConfigurationAndResponses(t *testing.T) {
	for _, endpoint := range []string{"", "file:///tmp/provider", "http://user:secret@localhost:5000", "http://localhost:5000?voice=x"} {
		if _, err := NewHTTPClient(endpoint, "", nil); err == nil {
			t.Errorf("accepted invalid provider URL %q", endpoint)
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("not audio"))
	}))
	defer server.Close()
	client, _ := NewHTTPClient(server.URL, "", server.Client())
	if err := client.Stream(context.Background(), "text", func(Chunk) error { return nil }); err == nil || !strings.Contains(err.Error(), "invalid WAV") {
		t.Fatalf("unexpected malformed-response error: %v", err)
	}
}

// TestHTTPClientAcceptsWAVFallback keeps generic local providers usable without framed streaming.
func TestHTTPClientAcceptsWAVFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "audio/wav")
		_, _ = w.Write(testWAV([]byte{7, 0}))
	}))
	defer server.Close()
	client, _ := NewHTTPClient(server.URL, "", server.Client())
	var received Chunk
	if err := client.Stream(context.Background(), "Fallback.", func(chunk Chunk) error { received = chunk; return nil }); err != nil {
		t.Fatal(err)
	}
	if received.SampleRate != 22_050 || !bytes.Equal(received.Data, []byte{7, 0}) {
		t.Fatalf("unexpected WAV fallback: %#v", received)
	}
}

// TestHTTPClientReportsProviderStreamFailure preserves errors sent after HTTP headers.
func TestHTTPClientReportsProviderStreamFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", pcmStreamMediaType)
		w.Header().Set("X-Patchflow-Sample-Rate", "24000")
		w.Header().Set("X-Patchflow-Channels", "1")
		w.Header().Set("X-Patchflow-Bits-Per-Sample", "16")
		_ = binary.Write(w, binary.LittleEndian, pcmStreamErrorFrame)
	}))
	defer server.Close()
	client, _ := NewHTTPClient(server.URL, "", server.Client())
	err := client.Stream(context.Background(), "Fail safely.", func(Chunk) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "stopped before synthesis completed") {
		t.Fatalf("unexpected provider stream error: %v", err)
	}
}

// TestSpeechSegmentsBoundsLongProse verifies sentence, clause, and hard-length boundaries.
func TestSpeechSegmentsBoundsLongProse(t *testing.T) {
	segments := speechSegments("First sentence. This clause deliberately contains enough repeated words to cross the preferred phrase threshold without becoming an excessively long synthesis request, and the remaining explanation follows. " + strings.Repeat("bounded ", 40))
	if len(segments) < 4 || segments[0] != "First sentence." {
		t.Fatalf("unexpected speech segments: %#v", segments)
	}
	for _, segment := range segments {
		if len([]rune(segment)) > maxSegmentCharacters {
			t.Errorf("segment exceeds %d characters: %q", maxSegmentCharacters, segment)
		}
	}
}

// testWAV builds the smallest PCM RIFF document needed by provider tests.
func testWAV(pcm []byte) []byte {
	buffer := bytes.NewBuffer(nil)
	buffer.WriteString("RIFF")
	_ = binary.Write(buffer, binary.LittleEndian, uint32(36+len(pcm)))
	buffer.WriteString("WAVEfmt ")
	_ = binary.Write(buffer, binary.LittleEndian, uint32(16))
	_ = binary.Write(buffer, binary.LittleEndian, uint16(1))
	_ = binary.Write(buffer, binary.LittleEndian, uint16(1))
	_ = binary.Write(buffer, binary.LittleEndian, uint32(22_050))
	_ = binary.Write(buffer, binary.LittleEndian, uint32(44_100))
	_ = binary.Write(buffer, binary.LittleEndian, uint16(2))
	_ = binary.Write(buffer, binary.LittleEndian, uint16(16))
	buffer.WriteString("data")
	_ = binary.Write(buffer, binary.LittleEndian, uint32(len(pcm)))
	buffer.Write(pcm)
	return buffer.Bytes()
}

// writeTestSpeechFrame writes one provider-protocol frame for streaming client tests.
func writeTestSpeechFrame(w http.ResponseWriter, pcm []byte) {
	_ = binary.Write(w, binary.LittleEndian, uint32(len(pcm)))
	_, _ = w.Write(pcm)
}
