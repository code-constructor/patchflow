"""Stream Chatterbox Turbo PCM for Patchflow's original local assistant voice."""

from __future__ import annotations

import json
import os
import struct
import threading
from dataclasses import dataclass
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from typing import Any, Iterator

import numpy as np
import torch
from chatterbox.tts_turbo import ChatterboxTurboTTS


PCM_STREAM_MEDIA_TYPE = "application/vnd.patchflow.pcm-stream"
PCM_STREAM_ERROR_FRAME = 0xFFFFFFFF


@dataclass(frozen=True)
class Settings:
    """Settings describes the persisted persona and CPU streaming parameters."""

    device: str
    voice_path: Path
    chunk_tokens: int
    threads: int

    @classmethod
    def from_environment(cls) -> "Settings":
        """from_environment loads stable runtime defaults with local overrides."""

        available_threads = os.cpu_count() or 8
        return cls(
            device=os.getenv("PATCHFLOW_TTS_DEVICE", "cpu"),
            voice_path=Path(os.getenv("PATCHFLOW_TTS_VOICE_PATH", "/models/voice/reference.wav")),
            chunk_tokens=int(os.getenv("PATCHFLOW_TTS_CHUNK_TOKENS", "24")),
            threads=int(os.getenv("PATCHFLOW_TTS_THREADS", str(min(16, available_threads)))),
        )


class SpeechEngine:
    """SpeechEngine clones one designed voice and serializes model inference."""

    def __init__(self, settings: Settings) -> None:
        """Load Chatterbox Turbo and prepare the designed voice before serving."""

        if not settings.voice_path.is_file():
            raise FileNotFoundError(f"Designed voice does not exist: {settings.voice_path}")
        if settings.chunk_tokens <= 0 or settings.threads <= 0:
            raise ValueError("chunk tokens and thread count must be positive")
        self.settings = settings
        self.lock = threading.Lock()
        torch.set_num_threads(settings.threads)
        print(
            f"Loading Chatterbox Turbo on {settings.device} with {settings.threads} Torch threads",
            flush=True,
        )
        self.model = ChatterboxTurboTTS.from_pretrained(device=settings.device)
        self.model.prepare_conditionals(str(settings.voice_path))
        print("Patchflow Chatterbox Turbo streaming engine is ready", flush=True)

    @property
    def sample_rate(self) -> int:
        """sample_rate returns the fixed output rate advertised before streaming starts."""

        return int(self.model.sr)

    def stream_pcm(self, text: str) -> Iterator[bytes]:
        """stream_pcm yields signed little-endian PCM as soon as each model chunk exists."""

        with self.lock:
            for chunk in self.model.stream(text, chunk_tokens=self.settings.chunk_tokens):
                samples = chunk.audio.detach().cpu().numpy().reshape(-1)
                yield (np.clip(samples, -1.0, 1.0) * 32767.0).astype("<i2").tobytes()

    def info(self) -> dict[str, Any]:
        """Return non-sensitive runtime details for readiness checks."""

        return {
            "ready": True,
            "provider": "chatterbox-turbo-streaming",
            "device": self.settings.device,
            "voice": "patchflow-ai",
            "sample_rate": self.sample_rate,
        }


class SpeechHandler(BaseHTTPRequestHandler):
    """SpeechHandler exposes the private API consumed by Patchflow's Go proxy."""

    engine: SpeechEngine

    def do_GET(self) -> None:  # noqa: N802
        """Serve readiness information without exposing model or review content."""

        if self.path != "/info":
            self.send_error(HTTPStatus.NOT_FOUND)
            return
        self._send_json(HTTPStatus.OK, self.engine.info())

    def do_POST(self) -> None:  # noqa: N802
        """Validate one phrase and stream framed PCM chunks until completion."""

        if self.path != "/synthesize":
            self.send_error(HTTPStatus.NOT_FOUND)
            return
        try:
            text = self._read_text()
        except (json.JSONDecodeError, TypeError, ValueError) as error:
            self.send_error(HTTPStatus.UNPROCESSABLE_ENTITY, str(error))
            return

        self.send_response(HTTPStatus.OK)
        self.send_header("Content-Type", PCM_STREAM_MEDIA_TYPE)
        self.send_header("X-Patchflow-Sample-Rate", str(self.engine.sample_rate))
        self.send_header("X-Patchflow-Channels", "1")
        self.send_header("X-Patchflow-Bits-Per-Sample", "16")
        self.send_header("Cache-Control", "private, no-store")
        self.end_headers()
        try:
            emitted = False
            for audio in self.engine.stream_pcm(text):
                emitted = True
                self._write_frame(len(audio), audio)
            if not emitted:
                raise RuntimeError("model returned no audio")
            self._write_frame(0)
        except (BrokenPipeError, ConnectionResetError):
            return
        except Exception as error:  # noqa: BLE001
            print(f"Streaming synthesis failed: {error}", flush=True)
            try:
                self._write_frame(PCM_STREAM_ERROR_FRAME)
            except (BrokenPipeError, ConnectionResetError):
                pass

    def log_message(self, format_string: str, *args: Any) -> None:
        """Write concise request logs without dumping synthesis payloads."""

        print(f"{self.address_string()} - {format_string % args}", flush=True)

    def _read_text(self) -> str:
        """Read and bound one JSON synthesis request before response streaming starts."""

        length = int(self.headers.get("Content-Length", "0"))
        if length <= 0 or length > 16 * 1024:
            raise ValueError("request body must contain between 1 and 16384 bytes")
        payload = json.loads(self.rfile.read(length))
        text = str(payload.get("text", "")).strip()
        if not text or len(text) > 1000:
            raise ValueError("text must contain between 1 and 1000 characters")
        return text

    def _write_frame(self, length: int, audio: bytes = b"") -> None:
        """Write one little-endian frame and flush it through the Compose network."""

        self.wfile.write(struct.pack("<I", length))
        if audio:
            self.wfile.write(audio)
        self.wfile.flush()

    def _send_json(self, status: HTTPStatus, payload: dict[str, Any]) -> None:
        """Serialize one small JSON response with explicit caching behavior."""

        encoded = json.dumps(payload).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(encoded)))
        self.send_header("Cache-Control", "private, no-store")
        self.end_headers()
        self.wfile.write(encoded)


def main() -> None:
    """Load the local persona before accepting private Compose-network requests."""

    SpeechHandler.engine = SpeechEngine(Settings.from_environment())
    address = os.getenv("PATCHFLOW_TTS_ADDRESS", "0.0.0.0")
    port = int(os.getenv("PATCHFLOW_TTS_PORT", "5000"))
    server = ThreadingHTTPServer((address, port), SpeechHandler)
    print(f"Patchflow Chatterbox service listening on http://{address}:{port}", flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
