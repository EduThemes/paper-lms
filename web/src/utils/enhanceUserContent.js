// Post-render enhancement of stored rich content, shared by every display
// surface (RichContentViewer and SafeHTML): equations are typeset with KaTeX
// (G007) and media players get their caption tracks (G002).
import { renderMathInElement } from './mathRender';
import { enhanceMediaInElement } from './mediaEnhance';

export function enhanceUserContent(container) {
  if (!container) return;
  renderMathInElement(container);
  enhanceMediaInElement(container).catch(() => {});
}

export default enhanceUserContent;
