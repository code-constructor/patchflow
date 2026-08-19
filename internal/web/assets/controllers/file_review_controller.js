import { Controller } from "@hotwired/stimulus"

export default class extends Controller {
  static targets = ["item", "treeLink"]

  /** Restores an addressable file position and starts tracking the visible diff. */
  connect() {
    this.handleIntersection = this.handleIntersection.bind(this)
    this.handlePopState = this.handlePopState.bind(this)
    this.handleFrameLoad = this.handleFrameLoad.bind(this)
    this.receiveMode = this.receiveMode.bind(this)
    window.addEventListener("popstate", this.handlePopState)
    window.addEventListener("patchflow:diff-mode", this.receiveMode)
    document.addEventListener("turbo:frame-load", this.handleFrameLoad)
    this.requestedPath = window.location.pathname
    this.tracking = false
    this.observer = new IntersectionObserver(this.handleIntersection, {
      rootMargin: "-18% 0px -68% 0px",
      threshold: 0
    })
    for (const item of this.itemTargets) this.observer.observe(item)
    this.layoutObserver = new ResizeObserver(() => this.handleInitialResize())
    const content = this.element.querySelector(".file-browser__content")
    if (content) this.layoutObserver.observe(content)
    this.restoreInitialPosition()
  }

  /** Stops viewport and history observation before Turbo removes the file stream. */
  disconnect() {
    this.observer?.disconnect()
    this.layoutObserver?.disconnect()
    clearTimeout(this.initialSettleTimer)
    window.removeEventListener("popstate", this.handlePopState)
    window.removeEventListener("patchflow:diff-mode", this.receiveMode)
    document.removeEventListener("turbo:frame-load", this.handleFrameLoad)
  }

  /** Reanchors the first deep-link jump while eager diff frames settle. */
  restoreInitialPosition() {
    this.anchoringInitialPosition = true
    requestAnimationFrame(() => requestAnimationFrame(() => {
      this.scrollToPath(this.requestedPath, false)
      this.scheduleInitialSettle()
    }))
  }

  /** Keeps a direct file resource anchored as lazy frames change preceding heights. */
  handleFrameLoad(event) {
    if (!this.anchoringInitialPosition || !this.element.contains(event.target)) return
    requestAnimationFrame(() => this.scrollToPath(this.requestedPath, false))
    this.scheduleInitialSettle()
  }

  /** Reanchors while the continuous stream changes height during initial loading. */
  handleInitialResize() {
    if (!this.anchoringInitialPosition) return
    requestAnimationFrame(() => this.scrollToPath(this.requestedPath, false))
    this.scheduleInitialSettle()
  }

  /** Restarts the short quiet period used to detect a stable initial layout. */
  scheduleInitialSettle() {
    clearTimeout(this.initialSettleTimer)
    this.initialSettleTimer = setTimeout(() => this.finishInitialPosition(), 650)
  }

  /** Ends initial anchoring and hands subsequent URL changes to viewport tracking. */
  finishInitialPosition() {
    if (!this.anchoringInitialPosition) return
    clearTimeout(this.initialSettleTimer)
    this.scrollToPath(this.requestedPath, false)
    this.anchoringInitialPosition = false
    this.tracking = true
    this.layoutObserver?.disconnect()
  }

  /** Scrolls to a tree selection without replacing the continuous file page. */
  navigate(event) {
    event.preventDefault()
    const item = document.getElementById(event.params.domId)
    if (!item) return

    history.pushState(history.state, "", this.withCurrentQuery(event.currentTarget.href))
    this.activate(item.dataset.filePath)
    item.scrollIntoView({ behavior: "smooth", block: "start" })
  }

  /** Keeps the tree and shareable URL aligned with the file nearest the reading line. */
  handleIntersection(entries) {
    if (!this.tracking) return
    const visible = entries.filter((entry) => entry.isIntersecting)
    if (visible.length === 0) return

    visible.sort((left, right) => Math.abs(left.boundingClientRect.top) - Math.abs(right.boundingClientRect.top))
    const item = visible[0].target
    this.activate(item.dataset.filePath)
    history.replaceState(history.state, "", this.withCurrentQuery(item.dataset.fileUrl))
  }

  /** Restores in-page navigation when browser history moves between changed files. */
  handlePopState() {
    this.scrollToPath(window.location.pathname, true)
  }

  /** Carries a newly selected diff layout into frames that have not loaded yet. */
  receiveMode(event) {
    const mode = event.detail.mode === "line-by-line" ? "unified" : "split"
    for (const frame of this.element.querySelectorAll("turbo-frame[src]")) {
      const target = new URL(frame.getAttribute("src"), window.location.href)
      target.searchParams.set("diff", mode)
      frame.setAttribute("src", target.pathname + target.search)
    }
  }

  /** Finds one file resource path and optionally scrolls it into view. */
  scrollToPath(path, smooth) {
    const item = this.itemTargets.find((candidate) => new URL(candidate.dataset.fileUrl, window.location.href).pathname === path)
    if (!item) return

    this.activate(item.dataset.filePath)
    item.scrollIntoView({ behavior: smooth ? "smooth" : "auto", block: "start" })
  }

  /** Marks exactly one stream item and tree link as the active file. */
  activate(path) {
    for (const item of this.itemTargets) item.classList.toggle("is-active", item.dataset.filePath === path)
    for (const link of this.treeLinkTargets) {
      const active = link.dataset.filePath === path
      link.classList.toggle("is-active", active)
      if (active) link.setAttribute("aria-current", "location")
      else link.removeAttribute("aria-current")
    }
  }

  /** Adds only the current diff presentation query to a stable file resource. */
  withCurrentQuery(value) {
    const target = new URL(value, window.location.href)
    const mode = new URL(window.location.href).searchParams.get("diff")
    if (mode === "split" || mode === "unified") target.searchParams.set("diff", mode)
    return target.pathname + target.search
  }
}
