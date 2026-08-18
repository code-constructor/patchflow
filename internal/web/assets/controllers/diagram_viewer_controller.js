import { Controller } from "@hotwired/stimulus"

export default class extends Controller {
  static targets = ["canvas", "dialog"]

  /** Moves a clicked rendered SVG into the fullscreen dialog without duplicating IDs. */
  expand(event) {
    if (event.target.closest(".diagram-dialog")) return
    if (!event.target.closest(".diagram-expand, .mermaid-diagram")) return

    // Ignore the SVG already moved into the dialog when events bubble.
    const diagram = [...this.element.querySelectorAll(".mermaid-diagram svg")]
      .find((candidate) => !candidate.closest("dialog"))
    if (!diagram || this.dialogTarget.open) return

    this.placeholder = document.createComment("diagram origin")
    diagram.before(this.placeholder)
    this.diagram = diagram
    this.canvasTarget.replaceChildren(diagram)
    document.documentElement.classList.add("is-diagram-open")
    this.dialogTarget.showModal()
  }

  /** Requests a normal native-dialog close. */
  close() {
    this.dialogTarget.close()
  }

  /** Closes the overlay only when the user clicks its backdrop. */
  dismiss(event) {
    if (event.target === this.dialogTarget) this.close()
  }

  /** Returns the original SVG to its chapter block and unlocks page scrolling. */
  restore() {
    if (this.placeholder?.isConnected && this.diagram) {
      this.placeholder.replaceWith(this.diagram)
    }
    this.canvasTarget.replaceChildren()
    this.placeholder = null
    this.diagram = null
    document.documentElement.classList.remove("is-diagram-open")
  }

  /** Restores moved content before Turbo removes the controller element. */
  disconnect() {
    this.restore()
  }
}
