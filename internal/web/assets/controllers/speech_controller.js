import { Controller } from "@hotwired/stimulus"

const MAX_FRAME_BYTES = 64 * 1024 * 1024
const STREAM_ERROR_FRAME = 0xffffffff

export default class extends Controller {
  static targets = ["content", "loadingIcon", "pauseIcon", "playIcon", "status", "stopButton", "toggle"]

  /** Prepares one independent streaming player and listens for another block taking over. */
  connect() {
    this.handleExternalStart = this.handleExternalStart.bind(this)
    window.addEventListener("patchflow:speech-start", this.handleExternalStart)
    this.runID = 0
    this.active = false
    this.paused = false
    this.sources = new Set()
    this.updateState("idle", "Read this block aloud")
  }

  /** Cancels pending synthesis and playback before Turbo removes this controller. */
  disconnect() {
    window.removeEventListener("patchflow:speech-start", this.handleExternalStart)
    this.stop()
  }

  /** Starts, pauses, or resumes this block according to its current state. */
  toggle() {
    if (!this.active) {
      this.start()
      return
    }
    if (this.paused) {
      this.resume()
      return
    }
    if (this.audioContext && this.state === "playing") this.pause()
  }

  /** Opens Web Audio during the user gesture and requests locally streamed PCM. */
  async start() {
    const text = this.readableText()
    if (!text) {
      this.fail("This block has no readable text")
      return
    }

    const AudioContextClass = window.AudioContext || window.webkitAudioContext
    if (!AudioContextClass) {
      this.fail("This browser cannot play streamed audio")
      return
    }

    window.dispatchEvent(new CustomEvent("patchflow:speech-start", { detail: { source: this } }))
    this.releasePlayback()
    const runID = ++this.runID
    this.active = true
    this.paused = false
    this.streamComplete = false
    this.nextStartTime = 0
    this.abortController = new AbortController()
    this.audioContext = new AudioContextClass()
    this.updateState("loading", "Generating local speech…")

    try {
      await this.audioContext.resume()
      const response = await fetch("/speech", {
        method: "POST",
        headers: { "Accept": "application/vnd.patchflow.pcm-stream", "Content-Type": "application/json" },
        body: JSON.stringify({ text }),
        signal: this.abortController.signal
      })
      if (!response.ok) {
        const message = (await response.text()).trim()
        throw new Error(message || "Local speech synthesis failed")
      }
      await this.consumeStream(response, runID)
      if (runID !== this.runID) return
      this.abortController = null
      this.streamComplete = true
      if (this.sources.size === 0) this.finish()
    } catch (error) {
      if (runID !== this.runID || error.name === "AbortError") return
      this.fail(this.errorMessage(error))
    }
  }

  /** Reads framed PCM from Fetch and schedules each complete frame immediately. */
  async consumeStream(response, runID) {
    const format = this.responseFormat(response)
    const reader = response.body?.getReader()
    if (!reader) throw new Error("This browser cannot read streaming audio")
    let pending = new Uint8Array(0)
    let terminalFrame = false

    while (!terminalFrame) {
      const { value, done } = await reader.read()
      if (done) break
      pending = this.concatenate(pending, value)
      while (pending.byteLength >= 4) {
        const frameLength = new DataView(pending.buffer, pending.byteOffset, 4).getUint32(0, true)
        if (frameLength === 0) {
          pending = pending.slice(4)
          terminalFrame = true
          break
        }
        if (frameLength === STREAM_ERROR_FRAME) throw new Error("Local speech synthesis stopped before the block was complete")
        if (frameLength > MAX_FRAME_BYTES) throw new Error("Local speech synthesis returned an oversized audio frame")
        if (pending.byteLength < frameLength + 4) break
        this.schedulePCM(pending.slice(4, frameLength + 4), format, runID)
        pending = pending.slice(frameLength + 4)
      }
    }

    if (!terminalFrame || pending.byteLength !== 0) throw new Error("Local speech synthesis returned an incomplete audio stream")
    await reader.cancel()
  }

  /** Parses and validates the PCM format advertised by the same-origin Go endpoint. */
  responseFormat(response) {
    const contentType = response.headers.get("Content-Type") || ""
    const sampleRate = Number.parseInt(response.headers.get("X-Patchflow-Sample-Rate") || "", 10)
    const channels = Number.parseInt(response.headers.get("X-Patchflow-Channels") || "", 10)
    const bitsPerSample = Number.parseInt(response.headers.get("X-Patchflow-Bits-Per-Sample") || "", 10)
    if (!contentType.startsWith("application/vnd.patchflow.pcm-stream") || !Number.isInteger(sampleRate) || sampleRate < 8000 || sampleRate > 96000 || channels !== 1 || bitsPerSample !== 16) {
      throw new Error("Local speech synthesis returned an unsupported audio format")
    }
    return { sampleRate, channels, bitsPerSample }
  }

  /** Appends a network fragment while retaining an incomplete frame prefix or payload. */
  concatenate(left, right) {
    if (!left.byteLength) return right
    const combined = new Uint8Array(left.byteLength + right.byteLength)
    combined.set(left)
    combined.set(right, left.byteLength)
    return combined
  }

  /** Converts little-endian signed PCM to an AudioBuffer and queues gap-free playback. */
  schedulePCM(bytes, format, runID) {
    if (runID !== this.runID || !this.audioContext || bytes.byteLength === 0 || bytes.byteLength % 2 !== 0) return
    const frameCount = bytes.byteLength / 2 / format.channels
    const audioBuffer = this.audioContext.createBuffer(format.channels, frameCount, format.sampleRate)
    const samples = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength)
    for (let channel = 0; channel < format.channels; channel += 1) {
      const output = audioBuffer.getChannelData(channel)
      for (let frame = 0; frame < frameCount; frame += 1) {
        output[frame] = samples.getInt16((frame * format.channels + channel) * 2, true) / 32768
      }
    }

    const source = this.audioContext.createBufferSource()
    source.buffer = audioBuffer
    source.connect(this.audioContext.destination)
    const startAt = Math.max(this.nextStartTime, this.audioContext.currentTime + 0.04)
    this.nextStartTime = startAt + audioBuffer.duration
    this.sources.add(source)
    source.onended = () => this.sourceEnded(source, runID)
    source.start(startAt)
    if (this.state === "loading") this.updateState("playing", "Pause reading")
  }

  /** Removes a completed source and closes the player after the terminal stream frame. */
  sourceEnded(source, runID) {
    this.sources.delete(source)
    if (runID === this.runID && this.streamComplete && this.sources.size === 0) this.finish()
  }

  /** Pauses the shared block timeline without discarding queued audio. */
  async pause() {
    if (!this.audioContext) return
    await this.audioContext.suspend()
    this.paused = true
    this.updateState("paused", "Resume reading")
  }

  /** Resumes already buffered and newly arriving audio from its current position. */
  async resume() {
    if (!this.audioContext) return
    try {
      await this.audioContext.resume()
      this.paused = false
      this.updateState("playing", "Pause reading")
    } catch (error) {
      this.fail(this.errorMessage(error))
    }
  }

  /** Stops synthesis or playback and restores the idle controls. */
  stop() {
    this.runID += 1
    this.active = false
    this.paused = false
    if (this.abortController) this.abortController.abort()
    this.abortController = null
    this.releasePlayback()
    this.updateState("idle", "Read this block aloud")
  }

  /** Stops this reader when another narrative block starts speaking. */
  handleExternalStart(event) {
    if (event.detail.source !== this && this.active) this.stop()
  }

  /** Extracts rendered prose while excluding controls and adjacent discussion content. */
  readableText() {
    return this.contentTarget.innerText.replace(/\s+/g, " ").trim().slice(0, 12000)
  }

  /** Stops scheduled sources and closes the current Web Audio context. */
  releasePlayback() {
    for (const source of this.sources || []) {
      source.onended = null
      try { source.stop() } catch (_) { /* The source may already have ended. */ }
    }
    this.sources = new Set()
    if (this.audioContext) this.audioContext.close().catch(() => {})
    this.audioContext = null
    this.nextStartTime = 0
    this.streamComplete = false
  }

  /** Synchronizes loading, playback, error, and accessibility feedback. */
  updateState(state, message) {
    this.state = state
    const loading = state === "loading"
    const playing = state === "playing"
    const paused = state === "paused"
    const active = loading || playing || paused
    this.playIconTarget.hidden = loading || playing
    this.pauseIconTarget.hidden = !playing
    this.loadingIconTarget.hidden = !loading
    this.stopButtonTarget.hidden = !active
    this.toggleTarget.classList.toggle("is-active", active)
    this.toggleTarget.classList.toggle("is-loading", loading)
    this.toggleTarget.classList.toggle("is-error", state === "error")
    this.toggleTarget.setAttribute("aria-pressed", String(playing || paused))
    this.toggleTarget.setAttribute("aria-label", message)
    this.toggleTarget.title = message
    this.statusTarget.textContent = message
    this.statusTarget.hidden = state !== "error"
  }

  /** Returns a concise browser or server failure without exposing an HTML response. */
  errorMessage(error) {
    return String(error?.message || "Local speech synthesis failed").replace(/\s+/g, " ").trim().slice(0, 240)
  }

  /** Finishes successful playback and releases the streamed timeline. */
  finish() {
    this.active = false
    this.paused = false
    this.releasePlayback()
    this.updateState("idle", "Read this block aloud")
  }

  /** Ends failed playback while keeping an actionable inline error visible. */
  fail(message) {
    this.runID += 1
    this.active = false
    this.paused = false
    if (this.abortController) this.abortController.abort()
    this.abortController = null
    this.releasePlayback()
    this.updateState("error", message)
  }
}
