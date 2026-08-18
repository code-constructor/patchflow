import { Controller } from "@hotwired/stimulus"
import DOMPurify from "dompurify"
import { marked } from "marked"

export default class extends Controller {
  static targets = ["source", "output"]

  async connect() {
    const renderedMarkdown = marked.parse(this.sourceTarget.textContent, {
      gfm: true,
      breaks: false
    })
    this.outputTarget.innerHTML = DOMPurify.sanitize(renderedMarkdown)
    this.sourceTarget.dataset.rendered = "true"
    await this.renderMermaidDiagrams()
  }

  async renderMermaidDiagrams() {
    if (!globalThis.mermaid) return

    globalThis.mermaid.initialize({
      startOnLoad: false,
      securityLevel: "strict",
      theme: "neutral"
    })

    const diagrams = this.outputTarget.querySelectorAll("pre > code.language-mermaid")
    for (const [index, source] of diagrams.entries()) {
      const container = document.createElement("div")
      container.className = "mermaid-diagram"
      source.parentElement.replaceWith(container)

      try {
        const identifier = `patchflow-mermaid-${Date.now()}-${index}`
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
