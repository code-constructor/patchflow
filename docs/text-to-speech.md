# Text-to-speech

Patchflow offers a read-aloud action on narrative text blocks. Playback is a
presentation aid: it does not change or add data to the review artifact.

## Runtime path

Speech does not use the browser Web Speech API. Browser and operating-system
voice availability is too inconsistent for the Omarchy Chromium app and would
make playback quality depend on global workstation configuration.

Instead, the browser posts rendered block text to Patchflow's same-origin
`POST /speech` endpoint. The Go server validates and divides the text at natural
sentence or clause boundaries. Its explicitly configured local provider emits
framed mono 16-bit PCM, which the Go server validates and flushes onward. The
browser schedules every complete frame through Web Audio without waiting for
the rest of the block. Pause, resume, stop, and cancellation remain local to the
active block. Generated audio is kept in memory and is not persisted.

The provider protocol uses `application/vnd.patchflow.pcm-stream`. Each frame
is a little-endian unsigned 32-bit payload length followed by signed
little-endian PCM. A zero-length frame completes the stream; `0xffffffff`
reports a provider failure after response headers were sent. Format headers
carry the sample rate, channel count, and bit depth. A complete PCM WAV response
remains supported as a fallback for simpler local providers.

## Local voice

The development Compose stack uses two local models:

- [Qwen3-TTS VoiceDesign](https://github.com/QwenLM/Qwen3-TTS) creates a
  reproducible reference clip from a written persona once.
- [Chatterbox Turbo](https://github.com/resemble-ai/chatterbox) clones that
  reference and produces the runtime speech.

The default prompt asks for an original mature female artificial-intelligence
voice: calm, precise, subtly warm, low-to-mid pitched, and lightly synthetic.
It explicitly excludes imitation of a real person or an existing fictional
character. Patchflow does not ship or clone the voice of Cortana or performer
Jen Taylor.

Both services belong to the Compose profile `speech` and stay off unless
`COMPOSE_PROFILES=speech` and `PATCHFLOW_TTS_URL=http://speech:5000` are set,
for example in `.env`. Qwen voice design runs once and writes its result to the
`patchflow-speech-models` Docker volume. It defaults to the CPU; a GPU is
attached through `compose.override.yaml` (see `compose.override.example.yaml`)
together with `PATCHFLOW_VOICE_DESIGN_DEVICE=cuda:0` and
`PATCHFLOW_VOICE_DESIGN_DTYPE=bfloat16`. AMD hosts additionally set
`PATCHFLOW_TORCH_IMAGE` to a `rocm/pytorch` tag; the default image serves CPU
and NVIDIA hosts. Chatterbox then streams on the CPU, which produced lower and
more stable time-to-first-audio on the development machine than its GPU path.
Tune the local trade-off in `.env`:

```sh
PATCHFLOW_TTS_THREADS=16
PATCHFLOW_TTS_CHUNK_TOKENS=24
```

Smaller token chunks can arrive earlier but increase boundary overhead. Larger
chunks improve throughput but delay the first sound. The voice instruction can
be changed with `PATCHFLOW_TTS_VOICE_INSTRUCTION`; changing it invalidates the
persisted reference fingerprint and causes a new reference to be designed.

The current true-streaming Chatterbox implementation is pinned to upstream pull
request 528 because the stable package exposes only complete-waveform
generation. That streaming path does not apply Chatterbox's Perth watermark,
which requires the complete waveform. Use a full-WAV provider when that
provenance watermark is a requirement.

## Privacy and native execution

Both model services are reachable only inside the Compose network. Review text
travels from the browser to Patchflow and then to the local speech container; it
is not sent to a hosted API. Model downloads are the only external requests.

A native Patchflow process remains provider-independent. Start a compatible
local HTTP service and connect it explicitly:

```sh
patchflow serve --tts-url http://127.0.0.1:5000
```

`--tts-voice` is available for providers that select named voices. If no
provider URL is configured, Patchflow does not render speech controls. Review
content must never be sent to a hosted speech service without an explicit
opt-in product decision.
