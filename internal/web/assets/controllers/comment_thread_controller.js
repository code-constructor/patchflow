import { Controller } from "@hotwired/stimulus"

export default class extends Controller {
  static targets = ["body", "composer", "endLine", "selection", "selectionLabel", "side", "startLine", "targetType"]
  static values = { anchors: Array }

  /** connect prepares stable pointer handlers and marks existing discussions. */
  connect() {
    this.trackLineSelection = this.trackLineSelection.bind(this)
    this.finishLineSelection = this.finishLineSelection.bind(this)
    this.markAnchoredLines()
  }

  /** disconnect releases a drag selection if Turbo removes its block mid-gesture. */
  disconnect() {
    this.stopTrackingLines()
  }

  /** openComposer reveals a contextual whole-block popover beside its action. */
  openComposer(event) {
    event.preventDefault()
    event.stopPropagation()
    this.resetTarget()
    this.revealComposer(event.currentTarget)
  }

  /** closeComposer dismisses the authoring popover without changing discussion. */
  closeComposer() {
    if (this.composerTarget.matches(":popover-open")) this.composerTarget.hidePopover()
  }

  /** revealComposer opens and positions the popover beside its source action. */
  revealComposer(anchor) {
    if (!this.composerTarget.matches(":popover-open")) this.composerTarget.showPopover()
    this.positionComposer(anchor)
    this.bodyTarget.focus({ preventScroll: true })
  }

  /** positionComposer keeps the overlay inside the current browser viewport. */
  positionComposer(anchor) {
    const anchorRect = anchor.getBoundingClientRect()
    const composerRect = this.composerTarget.getBoundingClientRect()
    const margin = 16
    const left = Math.min(Math.max(margin, anchorRect.left), window.innerWidth - composerRect.width - margin)
    const preferredTop = anchorRect.bottom + 8
    const top = preferredTop + composerRect.height <= window.innerHeight - margin
      ? preferredTop
      : Math.max(margin, anchorRect.top - composerRect.height - 8)
    this.composerTarget.style.left = `${left}px`
    this.composerTarget.style.top = `${top}px`
  }

  /** enhanceDiff adds drag handles after Diff2Html has rendered its rows. */
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

  /** decorateCell turns one rendered line number into a draggable comment handle. */
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
    button.dataset.action = "pointerdown->comment-thread#beginLineSelection"
    button.setAttribute("aria-label", `Comment on ${side} line ${line}`)
    button.textContent = "+"
    cell.dataset.commentLine = `${line}`
    cell.dataset.commentSide = side
    cell.prepend(button)
  }

  /** beginLineSelection starts a one-line selection that may be extended by dragging. */
  beginLineSelection(event) {
    if (event.button !== 0) return
    event.preventDefault()
    event.stopPropagation()
    this.dragSide = event.currentTarget.dataset.side
    this.dragStart = Number.parseInt(event.currentTarget.dataset.line, 10)
    this.dragAnchor = event.currentTarget.closest("[data-comment-line]")
    this.selectedSide = this.dragSide
    this.selectedStart = this.dragStart
    this.selectedEnd = this.dragStart
    this.markSelectedLines()
    document.documentElement.classList.add("is-line-selecting")
    window.addEventListener("pointermove", this.trackLineSelection)
    window.addEventListener("pointerup", this.finishLineSelection, { once: true })
  }

  /** trackLineSelection extends the range to the source line beneath the pointer. */
  trackLineSelection(event) {
    const candidate = document.elementFromPoint(event.clientX, event.clientY)?.closest("[data-comment-line][data-comment-side]")
    if (!candidate || candidate.dataset.commentSide !== this.dragSide) return
    const line = Number.parseInt(candidate.dataset.commentLine, 10)
    this.selectedStart = Math.min(this.dragStart, line)
    this.selectedEnd = Math.max(this.dragStart, line)
    this.dragAnchor = candidate
    this.markSelectedLines()
  }

  /** finishLineSelection persists the chosen range in the form and opens its overlay. */
  finishLineSelection(event) {
    this.trackLineSelection(event)
    this.stopTrackingLines()
    this.applySelection(this.dragAnchor)
  }

  /** stopTrackingLines releases global handlers and native-selection suppression. */
  stopTrackingLines() {
    window.removeEventListener("pointermove", this.trackLineSelection)
    window.removeEventListener("pointerup", this.finishLineSelection)
    document.documentElement.classList.remove("is-line-selecting")
  }

  /** applySelection synchronizes the visible range and submitted immutable anchor. */
  applySelection(anchor) {
    this.targetTypeTarget.value = "code"
    this.sideTarget.value = this.selectedSide
    this.startLineTarget.value = `${this.selectedStart}`
    this.endLineTarget.value = `${this.selectedEnd}`
    const lines = this.selectedStart === this.selectedEnd ? `line ${this.selectedStart}` : `lines ${this.selectedStart}–${this.selectedEnd}`
    this.selectionLabelTarget.textContent = `${this.selectedSide} ${lines}`
    this.selectionTarget.querySelector("span").textContent = `Selected ${this.selectedSide} ${lines}`
    this.selectionTarget.hidden = false
    this.markSelectedLines()
    this.revealComposer(anchor)
  }

  /** resetTarget returns the composer to a whole-block discussion. */
  resetTarget() {
    this.targetTypeTarget.value = "block"
    this.startLineTarget.value = ""
    this.endLineTarget.value = ""
    this.selectionLabelTarget.textContent = "Entire block"
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
