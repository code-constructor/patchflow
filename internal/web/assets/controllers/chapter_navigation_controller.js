import { Controller } from "@hotwired/stimulus"

export default class extends Controller {
  static targets = ["item"]

  /** connect tracks the evidence block currently crossing the reading position. */
  connect() {
    this.observer = new IntersectionObserver(
      (entries) => {
        const visible = entries
          .filter((entry) => entry.isIntersecting)
          .sort((left, right) => left.boundingClientRect.top - right.boundingClientRect.top)
        if (visible[0]) this.activate(visible[0].target.id)
      },
      { rootMargin: "-18% 0px -68% 0px", threshold: 0 }
    )

    for (const item of this.itemTargets) {
      const block = document.getElementById(item.dataset.chapterNavigationBlockIdParam)
      if (block) this.observer.observe(block)
    }
  }

  /** scroll moves to a block without changing the chapter URL or browser history. */
  scroll(event) {
    document.getElementById(event.params.blockId)?.scrollIntoView({ behavior: "smooth", block: "start" })
  }

  /** activate marks the navigation item for the block at the reading position. */
  activate(blockID) {
    for (const item of this.itemTargets) {
      item.classList.toggle("is-active", item.dataset.chapterNavigationBlockIdParam === blockID)
    }
  }

  /** disconnect releases block observers before Turbo replaces the chapter. */
  disconnect() {
    this.observer?.disconnect()
  }
}
