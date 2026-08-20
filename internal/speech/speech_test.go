package speech

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestPiperClientSynthesizesBoundedWAV verifies the provider request and returned media contract.
func TestPiperClientSynthesizesBoundedWAV(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/synthesize" || r.Method != http.MethodPost || r.Header.Get("Accept") != "audio/wav" {
			t.Errorf("unexpected provider request: %s %s", r.Method, r.URL.Path)
			return
		}
		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode provider payload: %v", err)
			return
		}
		if payload["text"] != "Explain the boundary." || payload["voice"] != "en_US-lessac-high" {
			t.Errorf("unexpected provider payload: %#v", payload)
			return
		}
		w.Header().Set("Content-Type", "audio/wav")
		_, _ = w.Write([]byte("RIFFxxxxWAVEtest"))
	}))
	defer server.Close()

	client, err := NewPiperClient(server.URL+"/api", "en_US-lessac-high", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	audio, err := client.Synthesize(context.Background(), "Explain the boundary.")
	if err != nil || audio.ContentType != "audio/wav" || string(audio.Data) != "RIFFxxxxWAVEtest" {
		t.Fatalf("unexpected synthesized audio: %#v %v", audio, err)
	}
}

// TestPiperClientRejectsUnsafeConfigurationAndResponses protects the local proxy boundary.
func TestPiperClientRejectsUnsafeConfigurationAndResponses(t *testing.T) {
	for _, endpoint := range []string{"", "file:///tmp/provider", "http://user:secret@localhost:5000", "http://localhost:5000?voice=x"} {
		if _, err := NewPiperClient(endpoint, "", nil); err == nil {
			t.Errorf("accepted invalid provider URL %q", endpoint)
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("not audio"))
	}))
	defer server.Close()
	client, _ := NewPiperClient(server.URL, "", server.Client())
	if _, err := client.Synthesize(context.Background(), "text"); err == nil || !strings.Contains(err.Error(), "invalid WAV") {
		t.Fatalf("unexpected malformed-response error: %v", err)
	}
}
