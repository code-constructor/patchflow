import { Controller } from "@hotwired/stimulus"

export default class extends Controller {
  static targets = ["composerTemplate"]
  static values = { anchors: Array }

  /** connect prepares stable pointer handlers and restores persisted thread markers. */
  connect() {
    this.trackLineSelection = this.trackLineSelection.bind(this)
    this.finishLineSelection = this.finishLineSelection.bind(this)
    this.handleDiscussionMutations = this.handleDiscussionMutations.bind(this)
    this.discussionObserver = new MutationObserver(this.handleDiscussionMutations)
    this.discussionObserver.observe(this.element, { childList: true, subtree: true })
    this.refreshDiscussionState()
  }

  /** disconnect releases pointer handlers and closes top-layer windows owned by this block. */
  disconnect() {
    this.stopTrackingLines()
    this.discussionObserver?.disconnect()
    for (const popover of this.element.querySelectorAll(".comment-popover:popover-open")) popover.hidePopover()
  }

  /** openComposer creates an independent whole-block draft beside its action. */
  openComposer(event) {
    event.preventDefault()
    event.stopPropagation()
    this.clearActiveSelection()
    this.revealComposer(event.currentTarget, null)
  }

  /** closePopover removes only the draft or discussion window containing the action. */
  closePopover(event) {
    const popover = event.currentTarget.closest(".comment-popover")
    if (!popover) return
    if (popover.matches(":popover-open")) popover.hidePopover()
    popover.remove()
    this.markSelectedLines()
  }

  /** commentOnBlock retargets one independent source draft without changing other drafts. */
  commentOnBlock(event) {
    const composer = event.currentTarget.closest(".comment-composer")
    if (!composer) return
    this.configureComposer(composer, null)
    this.markSelectedLines()
  }

  /** revealComposer clones, configures, and positions a new independent draft. */
  revealComposer(anchor, selection) {
    const fragment = this.composerTemplateTarget.content.cloneNode(true)
    const composer = fragment.querySelector(".comment-composer")
    composer.id = `comment-draft-${crypto.randomUUID()}`
    composer.querySelector("form").elements.draft_id.value = composer.id
    this.configureComposer(composer, selection)
    this.element.append(composer)
    composer.showPopover()
    this.positionPopover(composer, anchor)
    composer.querySelector("textarea[name='body']")?.focus({ preventScroll: true })
    this.clearActiveSelection()
  }

  /** configureComposer binds one cloned form to either its block or immutable source range. */
  configureComposer(composer, selection) {
    const form = composer.querySelector("form")
    const selectionView = composer.querySelector("[data-comment-role='selection']")
    const label = composer.querySelector("[data-comment-role='selection-label']")
    form.elements.target_type.value = selection ? "code" : "block"
    form.elements.start_line.value = selection ? `${selection.start}` : ""
    form.elements.end_line.value = selection ? `${selection.end}` : ""
    if (selection) {
      form.elements.side.value = selection.side
      const lines = selection.start === selection.end ? `line ${selection.start}` : `lines ${selection.start}–${selection.end}`
      label.textContent = `${selection.side} ${lines}`
      selectionView.querySelector("span").textContent = `Selected ${selection.side} ${lines}`
      selectionView.hidden = false
      composer.dataset.commentDraftSide = selection.side
      composer.dataset.commentDraftStart = `${selection.start}`
      composer.dataset.commentDraftEnd = `${selection.end}`
      return
    }
    label.textContent = "Entire block"
    selectionView.hidden = true
    delete composer.dataset.commentDraftSide
    delete composer.dataset.commentDraftStart
    delete composer.dataset.commentDraftEnd
  }

  /** openBlockThreads opens every persisted discussion for this block beside its badge. */
  openBlockThreads(event) {
    event.preventDefault()
    event.stopPropagation()
    const threadIDs = [...this.element.querySelectorAll(".discussion-panel .comment-thread[data-comment-thread-id]")]
      .map((thread) => thread.dataset.commentThreadId)
    this.revealThreads(event.currentTarget, threadIDs)
  }

  /** openLineThreads reopens discussions anchored to the selected source line. */
  openLineThreads(event) {
    event.preventDefault()
    event.stopPropagation()
    this.revealThreads(event.currentTarget, JSON.parse(event.currentTarget.dataset.threadIds || "[]"))
  }

  /** revealThreads clones persisted thread cards into an independent contextual window. */
  revealThreads(anchor, threadIDs) {
    const requestedIDs = [...new Set(threadIDs)]
    const unopenedIDs = []
    for (const threadID of requestedIDs) {
      const existing = this.openThreadPopover(threadID)
      if (existing) {
        existing.hidePopover()
        existing.showPopover()
        existing.focus({ preventScroll: true })
      } else {
        unopenedIDs.push(threadID)
      }
    }
    const threads = unopenedIDs.map((threadID) => this.threadElement(threadID)).filter(Boolean)
    if (threads.length === 0) return

    const popover = document.createElement("section")
    popover.className = "comment-popover thread-popover"
    popover.setAttribute("popover", "manual")
    popover.tabIndex = -1
    popover.dataset.threadIds = JSON.stringify(unopenedIDs)
    const header = document.createElement("header")
    header.className = "comment-composer__header"
    const title = document.createElement("strong")
    title.textContent = threads.length === 1 ? "Discussion" : `${threads.length} discussions`
    const close = document.createElement("button")
    close.type = "button"
    close.className = "comment-composer__close"
    close.dataset.action = "comment-thread#closePopover"
    close.setAttribute("aria-label", "Close discussions")
    close.textContent = "×"
    header.append(title, close)
    const list = document.createElement("div")
    list.className = "thread-popover__list"
    for (const thread of threads) {
      const clone = thread.cloneNode(true)
      clone.removeAttribute("id")
      for (const identified of clone.querySelectorAll("[id]")) identified.removeAttribute("id")
      list.append(clone)
    }
    popover.append(header, list)
    this.element.append(popover)
    popover.showPopover()
    this.positionPopover(popover, anchor)
  }

  /** openThreadPopover finds the one open discussion window already containing a thread. */
  openThreadPopover(threadID) {
    return [...this.element.querySelectorAll(".thread-popover:popover-open")]
      .find((popover) => JSON.parse(popover.dataset.threadIds || "[]").includes(threadID))
  }

  /** threadElement returns the original rendered card for one stable thread ID. */
  threadElement(threadID) {
    return [...this.element.querySelectorAll(".discussion-panel .comment-thread[data-comment-thread-id]")]
      .find((thread) => thread.dataset.commentThreadId === threadID)
  }

  /** handleDiscussionMutations refreshes only when a Turbo Stream adds discussion data. */
  handleDiscussionMutations(mutations) {
    const addedDiscussion = mutations.some((mutation) => [...mutation.addedNodes].some((node) => {
      if (node.nodeType !== Node.ELEMENT_NODE) return false
      return node.matches(".comment-thread, [data-comment-saved-thread-id]") || node.querySelector(".comment-thread, [data-comment-saved-thread-id]")
    }))
    if (addedDiscussion) this.refreshDiscussionState()
  }

  /** refreshDiscussionState promotes saved drafts and synchronizes controls after Turbo Streams. */
  refreshDiscussionState() {
    for (const saved of this.element.querySelectorAll("[data-comment-saved-thread-id]")) {
      const popover = saved.closest(".comment-popover")
      if (!popover || popover.classList.contains("thread-popover")) continue
      popover.classList.remove("comment-composer")
      popover.classList.add("thread-popover")
      popover.dataset.threadIds = JSON.stringify([saved.dataset.commentSavedThreadId])
      delete popover.dataset.commentDraftSide
      delete popover.dataset.commentDraftStart
      delete popover.dataset.commentDraftEnd
    }
    const persistedThreads = this.element.querySelectorAll(".discussion-panel .comment-thread[data-comment-thread-id]")
    const blockAction = this.element.querySelector(".block-thread-action")
    if (blockAction) blockAction.hidden = persistedThreads.length === 0
    this.markAnchoredLines()
    this.markSelectedLines()
  }

  /** threadAnchors merges initial evidence with threads appended by Turbo Streams. */
  threadAnchors() {
    const anchors = new Map(this.anchorsValue.map((anchor) => [anchor.id, anchor]))
    for (const thread of this.element.querySelectorAll(".discussion-panel .comment-thread[data-comment-target-type='code']")) {
      anchors.set(thread.dataset.commentThreadId, {
        id: thread.dataset.commentThreadId,
        side: thread.dataset.commentSide,
        start: Number.parseInt(thread.dataset.commentStart, 10),
        end: Number.parseInt(thread.dataset.commentEnd, 10),
      })
    }
    return [...anchors.values()]
  }

  /** positionPopover anchors a window to document coordinates so scrolling leaves it behind. */
  positionPopover(popover, anchor) {
    const anchorRect = anchor.getBoundingClientRect()
    const popoverRect = popover.getBoundingClientRect()
    const margin = 16
    const viewportLeft = Math.min(Math.max(margin, anchorRect.right + 8), window.innerWidth - popoverRect.width - margin)
    const preferredTop = anchorRect.bottom + 8
    const viewportTop = preferredTop + popoverRect.height <= window.innerHeight - margin
      ? preferredTop
      : Math.max(margin, anchorRect.top - popoverRect.height - 8)
    const bounds = {
      left: window.scrollX + margin,
      right: window.scrollX + window.innerWidth - margin,
      top: window.scrollY + margin,
      bottom: window.scrollY + window.innerHeight - margin,
    }
    let left = window.scrollX + viewportLeft
    let top = window.scrollY + viewportTop
    const others = [...this.element.querySelectorAll(".comment-popover:popover-open")].filter((candidate) => candidate !== popover)
    for (const other of others) {
      const rect = other.getBoundingClientRect()
      const documentRect = {
        left: window.scrollX + rect.left,
        right: window.scrollX + rect.right,
        top: window.scrollY + rect.top,
        bottom: window.scrollY + rect.bottom,
      }
      if (!this.popoversOverlap(left, top, popoverRect.width, popoverRect.height, documentRect)) continue
      if (documentRect.right + 12 + popoverRect.width <= bounds.right) {
        left = documentRect.right + 12
      } else if (documentRect.left - 12 - popoverRect.width >= bounds.left) {
        left = documentRect.left - 12 - popoverRect.width
      } else {
        top = Math.min(bounds.bottom - popoverRect.height, documentRect.top + 36)
      }
    }
    popover.style.left = `${left}px`
    popover.style.top = `${Math.max(bounds.top, top)}px`
  }

  /** popoversOverlap reports whether a proposed window would obscure an open one. */
  popoversOverlap(left, top, width, height, other) {
    return left < other.right && left + width > other.left && top < other.bottom && top + height > other.top
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

  /** decorateCell turns one rendered line number into a draggable comment handle. */
  decorateCell(cell, side) {
    if (!cell) return
    const match = cell.textContent.trim().match(/^\d+$/)
    if (!match) return

    const line = Number.parseInt(match[0], 10)
    cell.dataset.commentLine = `${line}`
    cell.dataset.commentSide = side
    if (!this.hasComposerTemplateTarget || cell.querySelector(".line-comment-button")) return

    const button = document.createElement("button")
    button.type = "button"
    button.className = "line-comment-button"
    button.dataset.line = `${line}`
    button.dataset.side = side
    button.dataset.action = "pointerdown->comment-thread#beginLineSelection"
    button.setAttribute("aria-label", `Comment on ${side} line ${line}`)
    button.textContent = "+"
    cell.prepend(button)
  }

  /** decorateThreadMarker adds a reusable discussion badge to one anchored source line. */
  decorateThreadMarker(element, anchors) {
    const host = element.querySelector(".line-actions") || element
    let button = host.querySelector(".line-thread-button")
    if (anchors.length === 0) {
      button?.remove()
      return
    }
    if (!button) {
      button = document.createElement("button")
      button.type = "button"
      button.className = "line-thread-button"
      button.dataset.action = "comment-thread#openLineThreads"
      host.append(button)
    }
    button.dataset.threadIds = JSON.stringify(anchors.map((anchor) => anchor.id))
    button.setAttribute("aria-label", `Open ${anchors.length} ${anchors.length === 1 ? "discussion" : "discussions"} on this line`)
    button.title = anchors.length === 1 && anchors[0].start !== anchors[0].end
      ? `Open discussion for lines ${anchors[0].start}–${anchors[0].end}`
      : "Open discussions"
    button.textContent = `${anchors.length}`
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

  /** finishLineSelection opens a new independent draft for the selected range. */
  finishLineSelection(event) {
    this.trackLineSelection(event)
    this.stopTrackingLines()
    const selection = { side: this.selectedSide, start: this.selectedStart, end: this.selectedEnd }
    this.revealComposer(this.dragAnchor, selection)
  }

  /** stopTrackingLines releases global handlers and native-selection suppression. */
  stopTrackingLines() {
    window.removeEventListener("pointermove", this.trackLineSelection)
    window.removeEventListener("pointerup", this.finishLineSelection)
    document.documentElement.classList.remove("is-line-selecting")
  }

  /** clearActiveSelection removes only the transient pointer-drag range. */
  clearActiveSelection() {
    this.selectedSide = null
    this.selectedStart = null
    this.selectedEnd = null
    this.markSelectedLines()
  }

  /** markSelectedLines highlights every open draft range plus the active drag range. */
  markSelectedLines() {
    const drafts = [...this.element.querySelectorAll(".comment-composer[data-comment-draft-side]")]
      .map((composer) => ({
        side: composer.dataset.commentDraftSide,
        start: Number.parseInt(composer.dataset.commentDraftStart, 10),
        end: Number.parseInt(composer.dataset.commentDraftEnd, 10),
      }))
    if (this.selectedSide) drafts.push({ side: this.selectedSide, start: this.selectedStart, end: this.selectedEnd })
    for (const element of this.element.querySelectorAll("[data-comment-line][data-comment-side]")) {
      const line = Number.parseInt(element.dataset.commentLine, 10)
      const selected = drafts.some((draft) => draft.side === element.dataset.commentSide && line >= draft.start && line <= draft.end)
      element.classList.toggle("is-comment-selected", selected)
    }
  }

  /** markAnchoredLines highlights source lines and adds controls for persisted discussions. */
  markAnchoredLines() {
    const persistedAnchors = this.threadAnchors()
    for (const element of this.element.querySelectorAll("[data-comment-line][data-comment-side]")) {
      const line = Number.parseInt(element.dataset.commentLine, 10)
      const anchors = persistedAnchors.filter((anchor) => anchor.side === element.dataset.commentSide && line >= anchor.start && line <= anchor.end)
      element.classList.toggle("has-comment-thread", anchors.length > 0)
      element.classList.toggle("is-comment-thread-start", anchors.some((anchor) => anchor.start === line))
      element.classList.toggle("is-comment-thread-end", anchors.some((anchor) => anchor.end === line))
      this.decorateThreadMarker(element, anchors.filter((anchor) => anchor.start === line))
    }
  }
}
