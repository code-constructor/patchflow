// Package speech connects Patchflow to an explicitly configured local text-to-speech provider.
package speech

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const maxAudioBytes int64 = 64 << 20
const maxAudioFrameBytes = 8 << 20
const maxSegmentCharacters = 180
const preferredClauseCharacters = 100
const pcmStreamMediaType = "application/vnd.patchflow.pcm-stream"
const pcmStreamErrorFrame = ^uint32(0)

// Chunk is one signed little-endian PCM segment that can be played before later text is synthesized.
type Chunk struct {
	SampleRate    int
	Channels      int
	BitsPerSample int
	Data          []byte
}

// Synthesizer emits playable audio incrementally in narrative order.
type Synthesizer interface {
	Stream(context.Context, string, func(Chunk) error) error
}

// HTTPClient calls a local HTTP speech provider one bounded text segment at a time.
type HTTPClient struct {
	endpoint *url.URL
	voice    string
	client   *http.Client
}

// NewHTTPClient validates one local speech endpoint and creates a bounded client.
func NewHTTPClient(endpoint, voice string, client *http.Client) (*HTTPClient, error) {
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
	return &HTTPClient{endpoint: parsed, voice: strings.TrimSpace(voice), client: client}, nil
}

// Stream synthesizes short phrases sequentially and forwards provider chunks immediately.
func (p *HTTPClient) Stream(ctx context.Context, text string, emit func(Chunk) error) error {
	segments := speechSegments(text)
	if len(segments) == 0 {
		return errors.New("speech text does not contain a playable segment")
	}
	var format *Chunk
	for _, segment := range segments {
		err := p.streamSegment(ctx, segment, func(chunk Chunk) error {
			if format == nil {
				format = &Chunk{SampleRate: chunk.SampleRate, Channels: chunk.Channels, BitsPerSample: chunk.BitsPerSample}
			} else if chunk.SampleRate != format.SampleRate || chunk.Channels != format.Channels || chunk.BitsPerSample != format.BitsPerSample {
				return errors.New("local speech provider changed audio format during playback")
			}
			return emit(chunk)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// streamSegment requests one phrase and accepts either framed PCM or a bounded WAV fallback.
func (p *HTTPClient) streamSegment(ctx context.Context, text string, emit func(Chunk) error) error {
	payload := struct {
		Text  string `json:"text"`
		Voice string `json:"voice,omitempty"`
	}{Text: text, Voice: p.voice}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode speech request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint.String(), bytes.NewReader(encoded))
	if err != nil {
		return fmt.Errorf("create speech request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", pcmStreamMediaType+", audio/wav")
	response, err := p.client.Do(request)
	if err != nil {
		return fmt.Errorf("local speech provider is unavailable: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		messageText := strings.TrimSpace(string(message))
		if messageText == "" {
			messageText = response.Status
		}
		return fmt.Errorf("local speech provider rejected the request: %s", messageText)
	}
	mediaType, _, _ := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if mediaType == pcmStreamMediaType {
		return streamPCM(response.Body, response.Header, emit)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxAudioBytes+1))
	if err != nil {
		return fmt.Errorf("read synthesized audio: %w", err)
	}
	if int64(len(data)) > maxAudioBytes {
		return errors.New("synthesized audio exceeds the 64 MB response limit")
	}
	chunk, err := decodePCM(data)
	if err != nil {
		return err
	}
	return emit(chunk)
}

// streamPCM validates and decodes the provider's framed little-endian PCM protocol.
func streamPCM(reader io.Reader, headers http.Header, emit func(Chunk) error) error {
	sampleRate, err := positiveHeader(headers, "X-Patchflow-Sample-Rate")
	if err != nil {
		return err
	}
	channels, err := positiveHeader(headers, "X-Patchflow-Channels")
	if err != nil {
		return err
	}
	bitsPerSample, err := positiveHeader(headers, "X-Patchflow-Bits-Per-Sample")
	if err != nil || bitsPerSample != 16 {
		return errors.New("local speech provider returned unsupported PCM headers")
	}
	var total int64
	for {
		var length uint32
		if err := binary.Read(reader, binary.LittleEndian, &length); err != nil {
			return fmt.Errorf("read local speech stream frame: %w", err)
		}
		switch length {
		case 0:
			return nil
		case pcmStreamErrorFrame:
			return errors.New("local speech provider stopped before synthesis completed")
		}
		if length > maxAudioFrameBytes || length%2 != 0 {
			return errors.New("local speech provider returned an invalid PCM frame")
		}
		total += int64(length)
		if total > maxAudioBytes {
			return errors.New("synthesized audio exceeds the 64 MB response limit")
		}
		data := make([]byte, int(length))
		if _, err := io.ReadFull(reader, data); err != nil {
			return fmt.Errorf("read local speech stream audio: %w", err)
		}
		if err := emit(Chunk{SampleRate: sampleRate, Channels: channels, BitsPerSample: bitsPerSample, Data: data}); err != nil {
			return err
		}
	}
}

// positiveHeader parses one required positive integer from the provider response.
func positiveHeader(headers http.Header, name string) (int, error) {
	value, err := strconv.Atoi(headers.Get(name))
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("local speech provider returned invalid %s", name)
	}
	return value, nil
}

// decodePCM extracts one uncompressed PCM data chunk from a bounded RIFF/WAVE document.
func decodePCM(wav []byte) (Chunk, error) {
	if len(wav) < 12 || !bytes.Equal(wav[:4], []byte("RIFF")) || !bytes.Equal(wav[8:12], []byte("WAVE")) {
		return Chunk{}, errors.New("local speech provider returned an invalid WAV document")
	}
	var format Chunk
	var pcm []byte
	for offset := 12; offset+8 <= len(wav); {
		chunkSize := int(binary.LittleEndian.Uint32(wav[offset+4 : offset+8]))
		start := offset + 8
		end := start + chunkSize
		if chunkSize < 0 || end < start || end > len(wav) {
			return Chunk{}, errors.New("local speech provider returned a truncated WAV document")
		}
		switch string(wav[offset : offset+4]) {
		case "fmt ":
			if chunkSize < 16 || binary.LittleEndian.Uint16(wav[start:start+2]) != 1 {
				return Chunk{}, errors.New("local speech provider returned unsupported WAV encoding")
			}
			format.Channels = int(binary.LittleEndian.Uint16(wav[start+2 : start+4]))
			format.SampleRate = int(binary.LittleEndian.Uint32(wav[start+4 : start+8]))
			format.BitsPerSample = int(binary.LittleEndian.Uint16(wav[start+14 : start+16]))
		case "data":
			pcm = append([]byte(nil), wav[start:end]...)
		}
		offset = end + chunkSize%2
	}
	if format.SampleRate <= 0 || format.Channels <= 0 || format.BitsPerSample != 16 || len(pcm) == 0 || len(pcm)%2 != 0 {
		return Chunk{}, errors.New("local speech provider returned unsupported PCM audio")
	}
	format.Data = pcm
	return format, nil
}

// speechSegments creates prompt phrases that bound time-to-first-audio while preserving sentence cadence.
func speechSegments(text string) []string {
	words := strings.Fields(text)
	segments := make([]string, 0, len(words)/20+1)
	current := make([]string, 0, 24)
	characters := 0
	flush := func() {
		if len(current) == 0 {
			return
		}
		segments = append(segments, strings.Join(current, " "))
		current = current[:0]
		characters = 0
	}
	for _, word := range words {
		wordCharacters := utf8.RuneCountInString(word)
		if len(current) > 0 && characters+1+wordCharacters > maxSegmentCharacters {
			flush()
		}
		if len(current) > 0 {
			characters++
		}
		current = append(current, word)
		characters += wordCharacters
		if endsSentence(word) || (characters >= preferredClauseCharacters && endsClause(word)) {
			flush()
		}
	}
	flush()
	return segments
}

// endsSentence reports whether a token ends a natural sentence-level synthesis phrase.
func endsSentence(word string) bool {
	return strings.ContainsAny(strings.TrimRight(word, "\"')]}"), ".?!")
}

// endsClause reports whether a sufficiently long phrase can be split without an abrupt word boundary.
func endsClause(word string) bool {
	return strings.ContainsAny(strings.TrimRight(word, "\"')]}"), ",;:")
}
