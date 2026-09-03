import { Controller } from "@hotwired/stimulus"

export default class extends Controller {
  static targets = ["dialog"]

  /** Opens the declared review image in a native fullscreen overlay. */
  expand() {
    if (this.dialogTarget.open) return
    document.documentElement.classList.add("is-image-open")
    this.dialogTarget.showModal()
  }

  /** Closes the fullscreen image overlay. */
  close() {
    this.dialogTarget.close()
  }

  /** Unlocks page scrolling after any native dialog close, including Escape. */
  restore() {
    document.documentElement.classList.remove("is-image-open")
  }

  /** Closes the overlay only when its native backdrop is clicked. */
  dismiss(event) {
    if (event.target === this.dialogTarget) this.close()
  }

  /** Restores page scrolling if Turbo removes an open image block. */
  disconnect() {
    this.restore()
  }
}
