// Package speech connects Patchflow to an explicitly configured local text-to-speech provider.
package speech

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxAudioBytes int64 = 64 << 20

// Audio is one complete synthesized response safe to return through Patchflow.
type Audio struct {
	ContentType string
	Data        []byte
}

// Synthesizer produces playable audio from bounded review prose.
type Synthesizer interface {
	Synthesize(context.Context, string) (*Audio, error)
}

// PiperClient calls Piper's local HTTP synthesis API.
type PiperClient struct {
	endpoint *url.URL
	voice    string
	client   *http.Client
}

// NewPiperClient validates one local Piper endpoint and creates a bounded client.
func NewPiperClient(endpoint, voice string, client *http.Client) (*PiperClient, error) {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("TTS URL must be an absolute http or https URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("TTS URL must use http or https")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil {
		return nil, errors.New("TTS URL must not contain credentials, a query, or a fragment")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/synthesize"
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}
	return &PiperClient{endpoint: parsed, voice: strings.TrimSpace(voice), client: client}, nil
}

// Synthesize requests one WAV document and rejects malformed or oversized provider responses.
func (p *PiperClient) Synthesize(ctx context.Context, text string) (*Audio, error) {
	payload := struct {
		Text  string `json:"text"`
		Voice string `json:"voice,omitempty"`
	}{Text: text, Voice: p.voice}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode speech request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint.String(), bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("create speech request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "audio/wav")
	response, err := p.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("local speech provider is unavailable: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		messageText := strings.TrimSpace(string(message))
		if messageText == "" {
			messageText = response.Status
		}
		return nil, fmt.Errorf("local speech provider rejected the request: %s", messageText)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxAudioBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read synthesized audio: %w", err)
	}
	if int64(len(data)) > maxAudioBytes {
		return nil, errors.New("synthesized audio exceeds the 64 MB response limit")
	}
	if len(data) < 12 || !bytes.Equal(data[:4], []byte("RIFF")) || !bytes.Equal(data[8:12], []byte("WAVE")) {
		return nil, errors.New("local speech provider returned an invalid WAV document")
	}
	return &Audio{ContentType: "audio/wav", Data: data}, nil
}
