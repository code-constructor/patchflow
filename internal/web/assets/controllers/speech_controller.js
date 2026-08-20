import { Controller } from "@hotwired/stimulus"

export default class extends Controller {
  static targets = ["content", "playIcon", "pauseIcon", "status", "stopButton", "toggle"]

  /** Prepares one independent narrative reader and listens for another block taking over. */
  connect() {
    this.handleExternalStart = this.handleExternalStart.bind(this)
    window.addEventListener("patchflow:speech-start", this.handleExternalStart)
    this.runID = 0
    this.active = false
    this.paused = false
    this.updateState("idle", "Read this block aloud")
    if (!("speechSynthesis" in window) || !("SpeechSynthesisUtterance" in window)) {
      this.toggleTarget.disabled = true
      this.updateState("unavailable", "Text-to-speech is unavailable in this browser")
    }
  }

  /** Cancels speech owned by a block before Turbo removes its controller. */
  disconnect() {
    window.removeEventListener("patchflow:speech-start", this.handleExternalStart)
    if (this.active) this.stop()
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
    this.pause()
  }

  /** Starts a fresh sentence queue and gives this block exclusive speech ownership. */
  start() {
    const text = this.readableText()
    if (!text) {
      this.updateState("idle", "This block has no readable text")
      return
    }

    window.dispatchEvent(new CustomEvent("patchflow:speech-start", { detail: { source: this } }))
    window.speechSynthesis.cancel()
    this.runID += 1
    this.activeRunID = this.runID
    this.chunks = this.chunkText(text)
    this.chunkIndex = 0
    this.active = true
    this.paused = false
    this.updateState("playing", "Pause reading")
    this.speakNext(this.activeRunID)
  }

  /** Pauses the current browser utterance without losing the remaining sentences. */
  pause() {
    window.speechSynthesis.pause()
    this.paused = true
    this.updateState("paused", "Resume reading")
  }

  /** Resumes a paused browser utterance. */
  resume() {
    window.speechSynthesis.resume()
    this.paused = false
    this.updateState("playing", "Pause reading")
  }

  /** Stops this block, discards its queue, and restores the idle controls. */
  stop() {
    this.runID += 1
    this.active = false
    this.paused = false
    this.chunks = []
    window.speechSynthesis.cancel()
    this.updateState("idle", "Read this block aloud")
  }

  /** Stops this reader when another narrative block starts speaking. */
  handleExternalStart(event) {
    if (event.detail.source !== this && this.active) this.stop()
  }

  /** Speaks the next bounded sentence group and completes when the queue is empty. */
  speakNext(runID) {
    if (!this.active || runID !== this.runID) return
    if (this.chunkIndex >= this.chunks.length) {
      this.finish("Finished reading")
      return
    }

    const utterance = this.buildUtterance(this.chunks[this.chunkIndex])
    utterance.onend = () => {
      if (runID !== this.runID) return
      this.chunkIndex += 1
      this.speakNext(runID)
    }
    utterance.onerror = (event) => {
      if (runID !== this.runID || event.error === "canceled" || event.error === "interrupted") return
      this.finish("Could not read this block with the available voice")
    }
    window.speechSynthesis.speak(utterance)
  }

  /** Builds one browser utterance with a language-matched local voice when available. */
  buildUtterance(text) {
    const utterance = new SpeechSynthesisUtterance(text)
    const language = document.documentElement.lang || navigator.language || "en"
    const voice = this.selectVoice(language)
    utterance.lang = language
    utterance.rate = 0.95
    utterance.pitch = 1
    if (voice) utterance.voice = voice
    return utterance
  }

  /** Chooses a local language match before falling back to any matching browser voice. */
  selectVoice(language) {
    const voices = window.speechSynthesis.getVoices()
    const normalized = language.toLowerCase()
    const prefix = normalized.split("-")[0]
    return voices.find((voice) => voice.localService && voice.lang.toLowerCase() === normalized) ||
      voices.find((voice) => voice.localService && voice.lang.toLowerCase().startsWith(prefix)) ||
      voices.find((voice) => voice.lang.toLowerCase() === normalized) ||
      voices.find((voice) => voice.lang.toLowerCase().startsWith(prefix))
  }

  /** Extracts rendered prose while excluding controls and discussion content outside the target. */
  readableText() {
    return this.contentTarget.innerText.replace(/\s+/g, " ").trim().slice(0, 12000)
  }

  /** Splits long prose at sentence and word boundaries for reliable Chromium playback. */
  chunkText(text) {
    const language = document.documentElement.lang || navigator.language || "en"
    const segments = "Segmenter" in Intl
      ? Array.from(new Intl.Segmenter(language, { granularity: "sentence" }).segment(text), (entry) => entry.segment.trim())
      : text.match(/[^.!?]+[.!?]+|[^.!?]+$/g) || [text]
    const chunks = []
    let current = ""
    for (const segment of segments.filter(Boolean)) {
      const words = segment.split(/\s+/)
      for (const word of words) {
        if (current && `${current} ${word}`.length > 240) {
          chunks.push(current)
          current = word
        } else {
          current = current ? `${current} ${word}` : word
        }
      }
    }
    if (current) chunks.push(current)
    return chunks
  }

  /** Synchronizes icons, accessible labels, live feedback, and the optional stop action. */
  updateState(state, message) {
    const playing = state === "playing"
    const active = playing || state === "paused"
    this.playIconTarget.hidden = playing
    this.pauseIconTarget.hidden = !playing
    this.stopButtonTarget.hidden = !active
    this.toggleTarget.classList.toggle("is-active", active)
    this.toggleTarget.setAttribute("aria-pressed", String(active))
    this.toggleTarget.setAttribute("aria-label", message)
    this.toggleTarget.title = message
    this.statusTarget.textContent = message
  }

  /** Ends this queue without canceling unrelated speech and announces the result. */
  finish(message) {
    this.active = false
    this.paused = false
    this.chunks = []
    this.updateState("idle", message)
  }
}
