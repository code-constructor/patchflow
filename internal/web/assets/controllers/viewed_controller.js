import { Controller } from "@hotwired/stimulus"

export default class extends Controller {
  static targets = ["checkbox", "value"]

  /** Persists a changed checkbox and collapses only a classic file diff. */
  change() {
    this.valueTarget.value = this.checkboxTarget.checked ? "true" : "false"
    const disclosure = this.element.closest(".file-review-card")?.querySelector("[data-file-disclosure]")
    if (disclosure) disclosure.open = !this.checkboxTarget.checked
    this.element.requestSubmit()
  }
}
