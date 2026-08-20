# Text-to-speech

Patchflow offers a read-aloud action on narrative text blocks. Playback is a
presentation aid: it does not change or add data to the review artifact.

The first implementation uses the browser's Web Speech API. It keeps the Go
binary self-contained, avoids storing generated audio, and lets the browser use
voices installed on the developer's machine. Only one block speaks at a time;
each block can be paused, resumed, or stopped independently. Long prose is
split into bounded sentence groups before playback.

Voice availability and quality are controlled by the browser and operating
system. A browser may expose a local voice, a remote voice, or no usable voice.
Patchflow therefore treats browser speech as the zero-configuration provider,
not as a guarantee of high-quality local synthesis.

A future natural-voice provider should remain optional and local-first. The
browser controls stay stable while the Go server proxies a configurable local
TTS process such as Piper or Chatterbox and streams generated audio back to the
requesting block. Review content must never be sent to a hosted speech service
without an explicit opt-in product decision.
