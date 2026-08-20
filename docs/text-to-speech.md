# Text-to-speech

Patchflow offers a read-aloud action on narrative text blocks. Playback is a
presentation aid: it does not change or add data to the review artifact.

Speech does not use the browser Web Speech API. Browser and operating-system
voice availability is too inconsistent for the Omarchy Chromium app and would
make playback quality depend on global workstation configuration.

Instead, the browser posts rendered block text to Patchflow's same-origin
`POST /speech` endpoint. The Go server validates and bounds the text, calls an
explicitly configured local Piper HTTP server, and returns the generated WAV
document. The browser owns playback, pause, resume, stop, and cancellation.
Only one block plays at a time. Generated audio is held in memory for the
request and browser object URL; Patchflow does not persist it.

The development Compose stack builds Piper 1.6.0 with an English high-quality
voice and configures the Go server automatically. Set `PATCHFLOW_TTS_VOICE`
before building to choose another Piper voice:

```sh
PATCHFLOW_TTS_VOICE=de_DE-thorsten-high docker compose up -d --build
```

A native Patchflow process remains independent of Docker. Start any local Piper
HTTP server and connect it explicitly:

```sh
patchflow serve \
  --tts-url http://127.0.0.1:5000 \
  --tts-voice en_US-lessac-high
```

If no provider URL is configured, Patchflow does not render speech controls.
Review content must never be sent to a hosted speech service without an
explicit opt-in product decision. A future provider such as Chatterbox can
implement the same internal synthesis boundary without changing review
artifacts or browser controls.
