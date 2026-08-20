import { Controller } from "@hotwired/stimulus"

export default class extends Controller {
  static targets = ["content", "loadingIcon", "pauseIcon", "playIcon", "status", "stopButton", "toggle"]

  /** Prepares one independent audio player and listens for another block taking over. */
  connect() {
    this.handleExternalStart = this.handleExternalStart.bind(this)
    window.addEventListener("patchflow:speech-start", this.handleExternalStart)
    this.runID = 0
    this.active = false
    this.paused = false
    this.audioURL = null
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
    if (this.audio && !this.audio.paused) this.pause()
  }

  /** Requests locally synthesized audio and begins playback when it arrives. */
  async start() {
    const text = this.readableText()
    if (!text) {
      this.fail("This block has no readable text")
      return
    }

    window.dispatchEvent(new CustomEvent("patchflow:speech-start", { detail: { source: this } }))
    this.releaseAudio()
    const runID = ++this.runID
    this.active = true
    this.paused = false
    this.abortController = new AbortController()
    this.updateState("loading", "Generating local speech…")

    try {
      const response = await fetch("/speech", {
        method: "POST",
        headers: { "Accept": "audio/wav", "Content-Type": "application/json" },
        body: JSON.stringify({ text }),
        signal: this.abortController.signal
      })
      if (!response.ok) {
        const message = (await response.text()).trim()
        throw new Error(message || "Local speech synthesis failed")
      }
      const audio = new Audio()
      const audioURL = URL.createObjectURL(await response.blob())
      if (runID !== this.runID) {
        URL.revokeObjectURL(audioURL)
        return
      }
      this.audio = audio
      this.audioURL = audioURL
      audio.preload = "auto"
      audio.src = audioURL
      audio.onended = () => {
        if (runID === this.runID) this.finish()
      }
      audio.onerror = () => {
        if (runID === this.runID) this.fail("The generated audio could not be played")
      }
      await audio.play()
      if (runID !== this.runID) return
      this.abortController = null
      this.updateState("playing", "Pause reading")
    } catch (error) {
      if (runID !== this.runID || error.name === "AbortError") return
      this.fail(this.errorMessage(error))
    }
  }

  /** Pauses the current audio without discarding the generated block narration. */
  pause() {
    if (!this.audio) return
    this.audio.pause()
    this.paused = true
    this.updateState("paused", "Resume reading")
  }

  /** Resumes already generated audio from its current position. */
  async resume() {
    if (!this.audio) return
    try {
      await this.audio.play()
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
    this.releaseAudio()
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

  /** Releases the current media element and its temporary object URL. */
  releaseAudio() {
    if (this.audio) {
      this.audio.onended = null
      this.audio.onerror = null
      this.audio.pause()
      this.audio.removeAttribute("src")
      this.audio.load()
    }
    this.audio = null
    if (this.audioURL) URL.revokeObjectURL(this.audioURL)
    this.audioURL = null
  }

  /** Synchronizes loading, playback, error, and accessibility feedback. */
  updateState(state, message) {
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

  /** Finishes successful playback and releases the generated audio. */
  finish() {
    this.active = false
    this.paused = false
    this.releaseAudio()
    this.updateState("idle", "Read this block aloud")
  }

  /** Ends failed playback while keeping an actionable inline error visible. */
  fail(message) {
    this.runID += 1
    this.active = false
    this.paused = false
    if (this.abortController) this.abortController.abort()
    this.abortController = null
    this.releaseAudio()
    this.updateState("error", message)
  }
}
