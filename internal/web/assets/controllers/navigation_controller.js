import { Controller } from "@hotwired/stimulus"

export default class extends Controller {
  static values = { fallback: String }

  /** Records the current internal resource and enables the app-level back shortcut. */
  connect() {
    const current = this.currentPath()
    let stack = this.readStack()
    const existingIndex = stack.lastIndexOf(current)
    stack = existingIndex >= 0 ? stack.slice(0, existingIndex + 1) : [...stack, current]
    this.writeStack(stack.slice(-50))
    this.handleKeydown = this.handleKeydown.bind(this)
    window.addEventListener("keydown", this.handleKeydown)
  }

  /** Removes the global keyboard listener when Turbo replaces the current header. */
  disconnect() {
    window.removeEventListener("keydown", this.handleKeydown)
  }

  /** Returns to the previous Patchflow resource or the server-provided logical parent. */
  back() {
    const current = this.currentPath()
    const stack = this.readStack()
    while (stack.at(-1) === current) stack.pop()
    const target = this.navigablePath(stack.at(-1)) || this.navigablePath(this.fallbackValue)
    if (!target || target === current) return

    this.writeStack(stack)
    window.Turbo.visit(target, { action: "replace" })
  }

  /** Maps Alt+Left to the same safe in-app back behavior as the visible control. */
  handleKeydown(event) {
    if (!event.altKey || event.key !== "ArrowLeft" || event.repeat) return

    event.preventDefault()
    this.back()
  }

  /** Loads the current tab's bounded navigation history when browser storage is available. */
  readStack() {
    try {
      const value = JSON.parse(sessionStorage.getItem("patchflow:navigation-stack"))
      return Array.isArray(value) ? value.map((path) => this.navigablePath(path)).filter(Boolean) : []
    } catch (_) {
      return []
    }
  }

  /** Persists internal history without making navigation depend on browser storage. */
  writeStack(stack) {
    try {
      sessionStorage.setItem("patchflow:navigation-stack", JSON.stringify(stack))
    } catch (_) {
      // The logical server fallback remains available when storage is blocked.
    }
  }

  /** Returns the complete shareable path for the current Patchflow resource. */
  currentPath() {
    return window.location.pathname + window.location.search + window.location.hash
  }

  /** Accepts only same-origin application paths before handing them to Turbo. */
  navigablePath(value) {
    if (!value) return ""
    try {
      const parsed = new URL(value, window.location.origin)
      return parsed.origin === window.location.origin ? parsed.pathname + parsed.search + parsed.hash : ""
    } catch (_) {
      return ""
    }
  }
}
