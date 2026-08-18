import { Controller } from "@hotwired/stimulus"

export default class extends Controller {
  static targets = ["body", "details", "endLine", "selection", "selectionLabel", "side", "startLine", "targetType"]
  static values = { anchors: Array }

  /** connect marks source lines that already own persisted discussions. */
  connect() {
    this.markAnchoredLines()
  }

  /** enhanceDiff adds comment affordances after Diff2Html has rendered its rows. */
  enhanceDiff(event) {
    const output = event.target.querySelector("[data-diff-viewer-target='output']")
    if (!output) return

    const panes = output.querySelectorAll(".d2h-file-side-diff")
    if (panes.length >= 2) {
      this.decorateRows(panes[0], "base", ".d2h-code-side-linenumber")
      this.decorateRows(panes[1], "target", ".d2h-code-side-linenumber")
    } else {
      for (const row of output.querySelectorAll("tr")) {
        this.decorateCell(row.querySelector(".line-num1"), "base")
        this.decorateCell(row.querySelector(".line-num2"), "target")
      }
    }
    this.markAnchoredLines()
  }

  /** decorateRows equips every numbered row in one split-diff pane. */
  decorateRows(pane, side, selector) {
    for (const row of pane.querySelectorAll("tr")) this.decorateCell(row.querySelector(selector), side)
  }

  /** decorateCell turns one rendered line number into a code-comment trigger. */
  decorateCell(cell, side) {
    if (!cell || cell.querySelector(".line-comment-button")) return
    const match = cell.textContent.trim().match(/^\d+$/)
    if (!match) return

    const line = Number.parseInt(match[0], 10)
    const button = document.createElement("button")
    button.type = "button"
    button.className = "line-comment-button"
    button.dataset.line = `${line}`
    button.dataset.side = side
    button.dataset.action = "comment-thread#selectLine"
    button.setAttribute("aria-label", `Comment on ${side} line ${line}`)
    button.textContent = "+"
    cell.dataset.commentLine = `${line}`
    cell.dataset.commentSide = side
    cell.prepend(button)
  }

  /** selectLine chooses one line or extends the current selection with Shift-click. */
  selectLine(event) {
    event.preventDefault()
    event.stopPropagation()
    const line = Number.parseInt(event.currentTarget.dataset.line, 10)
    const side = event.currentTarget.dataset.side
    if (event.shiftKey && this.selectedSide === side && this.selectedStart) {
      this.selectedEnd = Math.max(this.selectedStart, line)
      this.selectedStart = Math.min(this.selectedStart, line)
    } else {
      this.selectedSide = side
      this.selectedStart = line
      this.selectedEnd = line
    }
    this.applySelection()
  }

  /** applySelection synchronizes the visible range and submitted immutable anchor. */
  applySelection() {
    this.targetTypeTarget.value = "code"
    this.sideTarget.value = this.selectedSide
    this.startLineTarget.value = `${this.selectedStart}`
    this.endLineTarget.value = `${this.selectedEnd}`
    const lines = this.selectedStart === this.selectedEnd ? `line ${this.selectedStart}` : `lines ${this.selectedStart}–${this.selectedEnd}`
    this.selectionLabelTarget.textContent = `Comment on ${this.selectedSide} ${lines}`
    this.selectionTarget.querySelector("span").textContent = `Selected ${this.selectedSide} ${lines}`
    this.selectionTarget.hidden = false
    this.detailsTarget.open = true
    this.markSelectedLines()
    this.bodyTarget.focus({ preventScroll: true })
  }

  /** resetTarget returns the composer to a whole-block discussion. */
  resetTarget() {
    this.targetTypeTarget.value = "block"
    this.startLineTarget.value = ""
    this.endLineTarget.value = ""
    this.selectionLabelTarget.textContent = "Comment on entire block"
    this.selectionTarget.hidden = true
    this.selectedSide = null
    this.selectedStart = null
    this.selectedEnd = null
    this.markSelectedLines()
  }

  /** markSelectedLines highlights the range currently targeted by the composer. */
  markSelectedLines() {
    for (const element of this.element.querySelectorAll("[data-comment-line][data-comment-side]")) {
      const line = Number.parseInt(element.dataset.commentLine, 10)
      const selected = element.dataset.commentSide === this.selectedSide && line >= this.selectedStart && line <= this.selectedEnd
      element.classList.toggle("is-comment-selected", Boolean(selected))
    }
  }

  /** markAnchoredLines highlights every source line covered by a persisted thread. */
  markAnchoredLines() {
    for (const element of this.element.querySelectorAll("[data-comment-line][data-comment-side]")) {
      const line = Number.parseInt(element.dataset.commentLine, 10)
      const anchored = this.anchorsValue.some((anchor) => anchor.side === element.dataset.commentSide && line >= anchor.start && line <= anchor.end)
      element.classList.toggle("has-comment-thread", anchored)
    }
  }
}
