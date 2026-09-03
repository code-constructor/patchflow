import { Controller } from "@hotwired/stimulus"
import DOMPurify from "dompurify"
import { marked } from "marked"

let diagramSequence = 0

export default class extends Controller {
  static targets = ["source", "output"]

  /** Converts stored Markdown into sanitized HTML when the block connects. */
  async connect() {
    const renderedMarkdown = marked.parse(this.sourceTarget.textContent, {
      gfm: true,
      breaks: false
    })
    this.outputTarget.innerHTML = DOMPurify.sanitize(renderedMarkdown)
    this.prepareLinks()
    this.sourceTarget.dataset.rendered = "true"
    await this.renderMermaidDiagrams()
  }

  /** prepareLinks keeps prose navigation out of an owning review-block Turbo Frame. */
  prepareLinks() {
    for (const link of this.outputTarget.querySelectorAll("a")) link.dataset.turboFrame = "_top"
  }

  /** Replaces Mermaid code fences with strict, sanitized SVG diagrams. */
  async renderMermaidDiagrams() {
    if (!globalThis.mermaid) return

    globalThis.mermaid.initialize({
      startOnLoad: false,
      securityLevel: "strict",
      theme: "neutral",
      // DOMPurify's strict SVG profile intentionally removes foreignObject.
      // Render labels as native SVG text so sanitizing cannot erase them.
      htmlLabels: false
    })

    const diagrams = this.outputTarget.querySelectorAll("pre > code.language-mermaid")
    for (const [index, source] of diagrams.entries()) {
      const container = document.createElement("div")
      container.className = "mermaid-diagram"
      source.parentElement.replaceWith(container)

      try {
        const identifier = `patchflow-mermaid-${Date.now()}-${diagramSequence++}-${index}`
        const { svg, bindFunctions } = await globalThis.mermaid.render(identifier, source.textContent)
        container.innerHTML = DOMPurify.sanitize(svg, { USE_PROFILES: { svg: true, svgFilters: true } })
        bindFunctions?.(container)
      } catch (error) {
        container.className = "mermaid-error"
        container.textContent = `Mermaid could not render this diagram: ${error.message}`
      }
    }
  }
}
