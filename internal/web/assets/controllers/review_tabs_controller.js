import { Controller } from "@hotwired/stimulus"

export default class extends Controller {
  static values = { memoryKey: String, view: String, reviewPath: String }

  /** Restores the last scroll offset when revisiting this exact review view. */
  connect() {
    this.rememberCurrentView = this.rememberCurrentView.bind(this)
    document.addEventListener("turbo:before-visit", this.rememberCurrentView)
    this.restoreScrollPosition()
  }

  /** Persists the departing view and removes the document-level listener. */
  disconnect() {
    document.removeEventListener("turbo:before-visit", this.rememberCurrentView)
  }

  /** Redirects a tab to its last visited resource before Turbo handles the click. */
  switchView(event) {
    this.rememberCurrentView()
    const remembered = this.readMemory(event.params.view)
    if (!remembered || !this.belongsToReview(remembered.path)) return

    event.currentTarget.href = remembered.path
  }

  /** Stores the current resource and vertical offset for this browser tab only. */
  rememberCurrentView() {
    const memory = { path: window.location.pathname + window.location.search, scrollY: window.scrollY }
    try {
      sessionStorage.setItem(this.storageKey(this.viewValue), JSON.stringify(memory))
    } catch (_) {
      // Navigation remains fully functional when browser storage is unavailable.
    }
  }

  /** Restores scrolling only when the remembered resource matches the current URL. */
  restoreScrollPosition() {
    const remembered = this.readMemory(this.viewValue)
    const currentPath = window.location.pathname + window.location.search
    if (!remembered || remembered.path !== currentPath || !Number.isFinite(remembered.scrollY)) return

    requestAnimationFrame(() => requestAnimationFrame(() => window.scrollTo({ top: remembered.scrollY })))
  }

  /** Reads one guarded view-memory record from session storage. */
  readMemory(view) {
    try {
      return JSON.parse(sessionStorage.getItem(this.storageKey(view)))
    } catch (_) {
      return null
    }
  }

  /** Namespaces transient memory by repository, review, and global view. */
  storageKey(view) {
    return `patchflow:review-view:${this.memoryKeyValue}:${view}`
  }

  /** Rejects remembered paths that do not belong to this review. */
  belongsToReview(path) {
    const parsed = new URL(path, window.location.origin)
    return parsed.origin === window.location.origin && (parsed.pathname === this.reviewPathValue || parsed.pathname.startsWith(`${this.reviewPathValue}/`))
  }
}
