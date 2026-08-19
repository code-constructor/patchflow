import { Controller } from "@hotwired/stimulus"

export default class extends Controller {
  static targets = ["copiedIcon", "copyIcon", "status"]
  static values = { path: String }

  /** copy writes the stable repository-relative review path to the clipboard. */
  async copy() {
    try {
      await navigator.clipboard.writeText(this.pathValue)
      this.showCopied()
    } catch (_error) {
      this.statusTarget.textContent = "Reference path could not be copied"
      this.element.title = "Copy failed"
    }
  }

  /** showCopied confirms the copy without navigating away from the chapter. */
  showCopied() {
    window.clearTimeout(this.resetTimer)
    this.copyIconTarget.hidden = true
    this.copiedIconTarget.hidden = false
    this.statusTarget.textContent = `Copied ${this.pathValue}`
    this.element.title = "Reference path copied"
    this.element.classList.add("is-copied")
    this.resetTimer = window.setTimeout(() => this.reset(), 1800)
  }

  /** reset restores the copy control after its confirmation interval. */
  reset() {
    this.copyIconTarget.hidden = false
    this.copiedIconTarget.hidden = true
    this.statusTarget.textContent = ""
    this.element.title = "Copy reference path"
    this.element.classList.remove("is-copied")
  }

  /** disconnect clears pending feedback when Turbo removes the block. */
  disconnect() {
    window.clearTimeout(this.resetTimer)
  }
}
