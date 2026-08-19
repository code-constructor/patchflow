import { Controller } from "@hotwired/stimulus"

export default class extends Controller {
  static targets = ["checkbox", "value"]

  /** Persists a changed checkbox and announces its immediate disclosure effect. */
  change() {
    const viewed = this.checkboxTarget.checked
    this.valueTarget.value = viewed ? "true" : "false"
    const disclosure = this.element.closest(".file-review-card")?.querySelector("[data-file-disclosure]")
    if (disclosure) disclosure.open = !viewed
    window.dispatchEvent(new CustomEvent("patchflow:viewed", {
      detail: { key: this.element.dataset.viewedKey, viewed }
    }))
    this.element.requestSubmit()
  }
}
