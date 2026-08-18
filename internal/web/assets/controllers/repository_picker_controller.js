import { Controller } from "@hotwired/stimulus"

export default class extends Controller {
  static targets = ["dialog", "frame", "input"]
  static values = { root: String }

  /** Lazily loads and opens the server-backed directory browser. */
  open() {
    if (!this.frameTarget.src) {
      this.frameTarget.src = `/repository-picker?path=${encodeURIComponent(this.rootValue)}`
    }
    this.dialogTarget.showModal()
  }

  /** Closes the repository browser without changing the selected path. */
  close() {
    this.dialogTarget.close()
  }

  /** Closes the browser only when the native dialog backdrop was clicked. */
  dismiss(event) {
    if (event.target === this.dialogTarget) this.close()
  }

  /** Copies a discovered Git root into the repository form and returns focus. */
  select(event) {
    this.inputTarget.value = event.params.path
    this.close()
    this.inputTarget.focus()
  }
}
