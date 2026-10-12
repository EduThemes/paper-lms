// Site-wide equation rendering for rich content (G007).
//
// Every display surface (RichContentViewer, SafeHTML and everything built on
// them) runs renderMathInElement over the rendered HTML. It understands every
// way an equation reaches stored content:
//
//   - <span class="math-tex" data-latex="...">\(...\)</span>   Paper's editor
//   - <span class="math_equation_latex">\(...\)</span>          Canvas's RCE
//   - <img class="equation_image" data-equation-content="...">  Canvas legacy
//     (the image points at Canvas's /equation_images service, which does not
//     exist here, so it would otherwise be a broken image)
//   - bare \( ... \), \[ ... \] and $$ ... $$ in text
import katex from 'katex';
// KaTeX layout CSS: without it .katex-mathml is not visually hidden and every
// equation renders twice (MathML + flattened text) (B100).
import 'katex/dist/katex.min.css';

/** Strip \( \) / \[ \] / $$ $$ delimiters from an equation's text. */
export function stripLatexDelimiters(text) {
  const t = String(text || '').trim();
  let m;
  if ((m = t.match(/^\\\(([\s\S]*)\\\)$/))) return { latex: m[1].trim(), display: false };
  if ((m = t.match(/^\\\[([\s\S]*)\\\]$/))) return { latex: m[1].trim(), display: true };
  if ((m = t.match(/^\$\$([\s\S]*)\$\$$/))) return { latex: m[1].trim(), display: true };
  return { latex: t, display: false };
}

/** The LaTeX (and display mode) of an equation element, or null. */
export function latexOfMathElement(el) {
  if (!el || !el.getAttribute) return null;
  if (el.tagName === 'IMG') {
    const latex = (el.getAttribute('data-equation-content') || '').trim();
    return latex ? { latex, display: el.getAttribute('data-display') === 'block' } : null;
  }
  const attr = el.getAttribute('data-latex');
  const parsed = attr != null && attr.trim() !== ''
    ? { latex: attr.trim(), display: false }
    : stripLatexDelimiters(el.textContent);
  if (el.getAttribute('data-display') === 'block') parsed.display = true;
  return parsed.latex ? parsed : null;
}

function renderInto(target, latex, displayMode) {
  try {
    katex.render(latex, target, { throwOnError: false, displayMode });
    return true;
  } catch {
    return false;
  }
}

const MATH_ELEMENT_SELECTOR = '.math-tex, .math_equation_latex, img.equation_image[data-equation-content], img[data-equation-content]';
const TEXT_DELIMITERS = /(\\\(.*?\\\)|\\\[.*?\\\]|\$\$.*?\$\$)/s;

/**
 * Render every equation inside container with KaTeX. Idempotent: rendered
 * equations are skipped on a second pass.
 */
export function renderMathInElement(container) {
  if (!container || typeof document === 'undefined') return;

  container.querySelectorAll(MATH_ELEMENT_SELECTOR).forEach((el) => {
    if (el.querySelector && el.querySelector('.katex')) return; // already rendered
    const math = latexOfMathElement(el);
    if (!math) return;
    if (el.tagName === 'IMG') {
      const span = document.createElement('span');
      span.className = 'math-tex';
      span.setAttribute('data-latex', math.latex);
      if (!renderInto(span, math.latex, math.display)) span.textContent = el.getAttribute('alt') || math.latex;
      el.replaceWith(span);
      return;
    }
    renderInto(el, math.latex, math.display);
  });

  const walker = document.createTreeWalker(container, NodeFilter.SHOW_TEXT, null);
  const textNodes = [];
  let node;
  while ((node = walker.nextNode())) {
    // Leave rendered math and code samples alone.
    if (node.parentElement?.closest('.katex, code, pre, textarea')) continue;
    if (TEXT_DELIMITERS.test(node.textContent)) textNodes.push(node);
  }

  textNodes.forEach((textNode) => {
    const parts = textNode.textContent.split(TEXT_DELIMITERS);
    if (parts.length <= 1) return;
    const fragment = document.createDocumentFragment();
    parts.forEach((part) => {
      if (!part) return;
      if (/^(\\\(.*\\\)|\\\[.*\\\]|\$\$.*\$\$)$/s.test(part)) {
        const { latex, display } = stripLatexDelimiters(part);
        const el = document.createElement(display ? 'div' : 'span');
        if (display) {
          el.style.textAlign = 'center';
          el.style.margin = '0.5em 0';
        }
        if (!renderInto(el, latex, display)) el.textContent = part;
        fragment.appendChild(el);
      } else {
        fragment.appendChild(document.createTextNode(part));
      }
    });
    textNode.parentNode.replaceChild(fragment, textNode);
  });
}
