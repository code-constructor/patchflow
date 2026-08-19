import { Controller } from "@hotwired/stimulus"

export default class extends Controller {
  static targets = ["content", "toggle"]
  static values = { storageKey: String, viewedKey: String, viewed: Boolean }

  /** Restores the explicit local preference or derives the initial state from Viewed. */
  connect() {
    const preference = this.readPreference()
    this.setCollapsed(preference ? preference === "collapsed" : this.viewedValue)
  }

  /** Reverses this evidence card and remembers the explicit browser-local choice. */
  toggle() {
    const collapsed = !this.contentTarget.hidden
    this.setCollapsed(collapsed)
    this.writePreference(collapsed ? "collapsed" : "expanded")
  }

  /** Applies a shared Viewed change to every plan block for the affected file. */
  viewedChanged(event) {
    if (event.detail.key !== this.viewedKeyValue) return

    this.viewedValue = event.detail.viewed
    if (this.viewedValue) {
      this.setCollapsed(true)
      this.writePreference("collapsed")
      return
    }

    this.setCollapsed(false)
    this.clearPreference()
  }

  /** Synchronizes visibility, icon direction, and accessible state. */
  setCollapsed(collapsed) {
    this.contentTarget.hidden = collapsed
    this.toggleTarget.classList.toggle("is-collapsed", collapsed)
    this.toggleTarget.setAttribute("aria-expanded", String(!collapsed))
    this.toggleTarget.setAttribute("aria-label", collapsed ? "Expand file evidence" : "Collapse file evidence")
    this.toggleTarget.title = collapsed ? "Expand file evidence" : "Collapse file evidence"
  }

  /** Reads one valid disclosure preference without making storage mandatory. */
  readPreference() {
    try {
      const preference = localStorage.getItem(this.preferenceKey())
      return preference === "expanded" || preference === "collapsed" ? preference : null
    } catch (_) {
      return null
    }
  }

  /** Persists one explicit disclosure preference for this review block. */
  writePreference(preference) {
    try {
      localStorage.setItem(this.preferenceKey(), preference)
    } catch (_) {
      // The disclosure remains usable when browser storage is unavailable.
    }
  }

  /** Removes a preference when an unviewed file returns to its default open state. */
  clearPreference() {
    try {
      localStorage.removeItem(this.preferenceKey())
    } catch (_) {
      // The disclosure remains usable when browser storage is unavailable.
    }
  }

  /** Namespaces local presentation state by repository, review, and stable block ID. */
  preferenceKey() {
    const match = window.location.pathname.match(/^(.*\/reviews\/[^/]+)/)
    const reviewPath = match ? match[1] : window.location.pathname
    return `patchflow:plan-file:${reviewPath}:${this.storageKeyValue}`
  }
}
