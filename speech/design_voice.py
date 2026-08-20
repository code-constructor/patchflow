"""Generate Patchflow's original local assistant persona with Qwen3-TTS VoiceDesign."""

from __future__ import annotations

import hashlib
import io
import json
import os
import wave
from dataclasses import dataclass
from pathlib import Path

import numpy as np
import torch
from qwen_tts import Qwen3TTSModel


DEFAULT_VOICE_INSTRUCTION = (
    "Create an original mature American female artificial-intelligence voice for a futuristic "
    "technical interface. Calm, highly intelligent, precise, subtly warm, confident, and composed. "
    "Use clear diction, a measured conversational pace, a low-to-mid pitch, restrained emotion, "
    "faint synthetic polish, and a trace of dry wit. The voice must be distinct and must not imitate "
    "or resemble any real person or existing fictional character."
)
DEFAULT_REFERENCE_TEXT = (
    "All systems are stable. I have mapped the change and prepared the next step for your review. "
    "We can proceed when you are ready."
)


@dataclass(frozen=True)
class Settings:
    """Settings contains every reproducible input to the designed voice."""

    model: str
    device: str
    dtype: str
    voice_dir: Path
    instruction: str
    reference_text: str

    @classmethod
    def from_environment(cls) -> "Settings":
        """from_environment loads stable defaults with optional local persona overrides."""

        return cls(
            model=os.getenv("PATCHFLOW_TTS_DESIGN_MODEL", "Qwen/Qwen3-TTS-12Hz-1.7B-VoiceDesign"),
            device=os.getenv("PATCHFLOW_TTS_DEVICE", "cuda:0"),
            dtype=os.getenv("PATCHFLOW_TTS_DTYPE", "bfloat16"),
            voice_dir=Path(os.getenv("PATCHFLOW_TTS_VOICE_DIR", "/models/voice")),
            instruction=os.getenv("PATCHFLOW_TTS_VOICE_INSTRUCTION", DEFAULT_VOICE_INSTRUCTION),
            reference_text=os.getenv("PATCHFLOW_TTS_REFERENCE_TEXT", DEFAULT_REFERENCE_TEXT),
        )


def main() -> None:
    """Reuse a matching reference or generate it once before the runtime service starts."""

    settings = Settings.from_environment()
    settings.voice_dir.mkdir(parents=True, exist_ok=True)
    reference_path = settings.voice_dir / "reference.wav"
    metadata_path = settings.voice_dir / "metadata.json"
    fingerprint = voice_fingerprint(settings)
    if reference_path.is_file() and stored_fingerprint(metadata_path) == fingerprint:
        print(f"Reusing designed Patchflow voice from {reference_path}", flush=True)
        return

    print(f"Designing original Patchflow voice with {settings.model}", flush=True)
    model = Qwen3TTSModel.from_pretrained(
        settings.model,
        device_map=settings.device,
        dtype=torch_dtype(settings.dtype),
        attn_implementation="sdpa",
    )
    wavs, sample_rate = model.generate_voice_design(
        text=settings.reference_text,
        language="English",
        instruct=settings.instruction,
    )
    reference_path.write_bytes(encode_wav(wavs[0], sample_rate))
    metadata_path.write_text(json.dumps({"fingerprint": fingerprint}, indent=2) + "\n", encoding="utf-8")
    print(f"Designed Patchflow voice written to {reference_path}", flush=True)


def voice_fingerprint(settings: Settings) -> str:
    """Hash every model and prompt input that defines the generated persona."""

    value = "\n".join((settings.model, settings.instruction, settings.reference_text))
    return hashlib.sha256(value.encode("utf-8")).hexdigest()


def stored_fingerprint(metadata_path: Path) -> str:
    """Read prior voice metadata while treating malformed local state as a cache miss."""

    try:
        return str(json.loads(metadata_path.read_text(encoding="utf-8"))["fingerprint"])
    except (OSError, KeyError, TypeError, ValueError, json.JSONDecodeError):
        return ""


def torch_dtype(name: str) -> torch.dtype:
    """Map a stable configuration name to one supported Torch inference dtype."""

    values = {"bfloat16": torch.bfloat16, "float16": torch.float16, "float32": torch.float32}
    try:
        return values[name]
    except KeyError as error:
        raise ValueError(f"Unsupported PATCHFLOW_TTS_DTYPE: {name}") from error


def encode_wav(audio: np.ndarray, sample_rate: int) -> bytes:
    """Normalize a generated floating waveform into a standard mono PCM document."""

    samples = np.asarray(audio, dtype=np.float32).reshape(-1)
    pcm = (np.clip(samples, -1.0, 1.0) * 32767.0).astype("<i2").tobytes()
    output = io.BytesIO()
    with wave.open(output, "wb") as wav_file:
        wav_file.setnchannels(1)
        wav_file.setsampwidth(2)
        wav_file.setframerate(int(sample_rate))
        wav_file.writeframes(pcm)
    return output.getvalue()


if __name__ == "__main__":
    main()
