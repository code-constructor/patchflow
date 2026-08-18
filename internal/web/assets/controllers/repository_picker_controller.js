import { Controller } from "@hotwired/stimulus"

export default class extends Controller {
  static targets = ["dialog", "frame", "input"]
  static values = { root: String }

  open() {
    if (!this.frameTarget.src) {
      this.frameTarget.src = `/repository-picker?path=${encodeURIComponent(this.rootValue)}`
    }
    this.dialogTarget.showModal()
  }

  close() {
    this.dialogTarget.close()
  }

  dismiss(event) {
    if (event.target === this.dialogTarget) this.close()
  }

  select(event) {
    this.inputTarget.value = event.params.path
    this.close()
    this.inputTarget.focus()
  }
}
