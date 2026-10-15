/**
 * The designer's UI strings live in their own i18next namespace ("designer"), registered when the designer
 * loads, so they ship in the designer's chunk and not in the app's main bundle.
 */
import { useTranslation } from 'react-i18next';
import i18n from '@/i18n';
import en from './en.json';
import es from './es.json';

export const DESIGNER_NS = 'designer';

let registered = false;
export function registerDesignerStrings(instance = i18n) {
  if (registered && instance === i18n) return;
  instance.addResourceBundle('en', DESIGNER_NS, en, true, true);
  instance.addResourceBundle('es', DESIGNER_NS, es, true, true);
  if (instance === i18n) registered = true;
}

registerDesignerStrings();

/** `const { t, i18n } = useDesignerT()` — t() reads from the designer namespace. */
export function useDesignerT() {
  return useTranslation(DESIGNER_NS);
}
