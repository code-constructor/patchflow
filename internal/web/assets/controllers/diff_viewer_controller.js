import { Controller } from "@hotwired/stimulus"
import DOMPurify from "dompurify"

export default class extends Controller {
  static targets = ["source", "output", "splitButton", "unifiedButton"]
  static values = { highlights: Object, initial: String }

  /** Restores the preferred layout and renders the initial diff. */
  connect() {
    const configuredMode = this.initialValue === "unified" ? "line-by-line" : "side-by-side"

    this.receiveMode = this.receiveMode.bind(this)
    window.addEventListener("patchflow:diff-mode", this.receiveMode)
    this.mode = this.modeFromURL() || configuredMode
    this.writeModeToURL()
    this.render()
  }

  /** Stops synchronizing this viewer after Turbo removes it. */
  disconnect() {
    window.removeEventListener("patchflow:diff-mode", this.receiveMode)
  }

  /** Switches the current diff to side-by-side presentation. */
  showSplit() {
    this.selectMode("side-by-side")
  }

  /** Switches the current diff to a single inline presentation. */
  showUnified() {
    this.selectMode("line-by-line")
  }

  /** Makes a selected layout shareable and synchronizes sibling diff blocks. */
  selectMode(mode) {
    this.mode = mode
    this.writeModeToURL()
    this.render()
    window.dispatchEvent(new CustomEvent("patchflow:diff-mode", { detail: { mode, source: this } }))
  }

  /** Applies a layout selected in another diff block on the same chapter. */
  receiveMode(event) {
    if (event.detail.source === this || event.detail.mode === this.mode) return

    this.mode = event.detail.mode
    this.render()
  }

  /** Reads the optional shareable diff layout from the current URL. */
  modeFromURL() {
    const value = new URL(window.location.href).searchParams.get("diff")
    if (value === "split") return "side-by-side"
    if (value === "unified") return "line-by-line"
    return null
  }

  /** Stores presentation state in the URL and carries it into review links. */
  writeModeToURL() {
    const value = this.mode === "line-by-line" ? "unified" : "split"
    const current = new URL(window.location.href)
    current.searchParams.set("diff", value)
    history.replaceState(history.state, "", current)

    for (const link of document.querySelectorAll("[data-preserve-diff-mode]")) {
      const target = new URL(link.href, window.location.href)
      target.searchParams.set("diff", value)
      link.href = target
    }
  }

  /** Rebuilds Diff2Html output and reapplies Patchflow's syntax spans. */
  render() {
    this.outputTarget.innerHTML = globalThis.Diff2Html.html(this.sourceTarget.textContent, {
      drawFileList: false,
      matching: "lines",
      outputFormat: this.mode
    })
    this.updateButtons()
    this.applySyntaxHighlighting()
  }

  /** Keeps the layout controls and their accessibility state in sync. */
  updateButtons() {
    const splitActive = this.mode === "side-by-side"
    this.splitButtonTarget.classList.toggle("is-active", splitActive)
    this.splitButtonTarget.setAttribute("aria-pressed", splitActive)
    this.unifiedButtonTarget.classList.toggle("is-active", !splitActive)
    this.unifiedButtonTarget.setAttribute("aria-pressed", !splitActive)
  }

  /** Applies server-generated token colors to the active diff layout. */
  applySyntaxHighlighting() {
    if (this.mode === "side-by-side") {
      const panes = this.outputTarget.querySelectorAll(".d2h-file-side-diff")
      this.highlightRows(panes[0], "old", ".d2h-code-side-linenumber")
      this.highlightRows(panes[1], "new", ".d2h-code-side-linenumber")
      return
    }

    for (const row of this.outputTarget.querySelectorAll("tr")) {
      const code = row.querySelector(".d2h-code-line-ctn")
      if (!code) continue

      const oldNumber = row.querySelector(".line-num1")?.textContent.trim()
      const newNumber = row.querySelector(".line-num2")?.textContent.trim()
      const side = row.querySelector(".d2h-del") ? "old" : "new"
      const number = side === "old" ? oldNumber : newNumber
      this.highlightCode(code, this.highlightsValue[side]?.[number])
    }
  }

  /** Highlights every code row in one side of a split diff. */
  highlightRows(pane, side, numberSelector) {
    if (!pane) return

    for (const row of pane.querySelectorAll("tr")) {
      const number = row.querySelector(numberSelector)?.textContent.trim()
      const code = row.querySelector(".d2h-code-line-ctn")
      this.highlightCode(code, this.highlightsValue[side]?.[number])
    }
  }

  /** Safely installs a pre-sanitized line's token spans into Diff2Html. */
  highlightCode(element, highlighted) {
    if (!element || !highlighted) return

    element.innerHTML = DOMPurify.sanitize(highlighted, {
      ALLOWED_TAGS: ["span"],
      ALLOWED_ATTR: ["style"]
    })
  }
}
