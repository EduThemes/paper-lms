import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import LanguageDetector from 'i18next-browser-languagedetector';

import en from './en.json';
import es from './es.json';

const RTL_LANGUAGES = ['ar', 'he', 'fa', 'ur'];

const applyDocumentLangDir = (lng) => {
  if (typeof document === 'undefined' || !lng) return;
  const base = String(lng).split('-')[0].toLowerCase();
  document.documentElement.lang = lng;
  document.documentElement.dir = RTL_LANGUAGES.includes(base) ? 'rtl' : 'ltr';
};

i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources: {
      en: { translation: en },
      es: { translation: es },
    },
    fallbackLng: 'en',
    // Empty string in a non-English bundle means "not yet translated" — fall back to English.
    // (Wave D.2's translator is filling es.json placeholders in parallel.)
    returnEmptyString: false,
    interpolation: {
      escapeValue: false, // React already escapes
    },
    detection: {
      order: ['localStorage', 'navigator'],
      caches: ['localStorage'],
      lookupLocalStorage: 'i18nextLng',
    },
  })
  .then(() => {
    applyDocumentLangDir(i18n.resolvedLanguage || i18n.language);
  });

i18n.on('languageChanged', (lng) => {
  applyDocumentLangDir(lng);
});

export default i18n;
