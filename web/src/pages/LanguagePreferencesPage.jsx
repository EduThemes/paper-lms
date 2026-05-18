import React, { useState } from 'react';
import { Globe, Check } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import Layout from '../components/Layout';

// LanguagePreferencesPage is the user-personal home for the locale
// choice. It replaces the globe icon that used to live in the main
// left-rail of Layout.jsx — language is a preference, not navigation.
//
// Persistence mirrors the legacy LanguageSwitcher: localStorage
// 'paperlms_locale' is the source of truth on this device, and i18next
// is updated synchronously so the rest of the app re-renders. The
// per-tenant `account.default_locale` (applied in AuthContext) is the
// fallback when the user has NOT made an explicit choice — setting a
// value here pins the device to that choice and overrides the tenant
// default.

const LANGUAGES = [
  { code: 'en', labelKey: 'languagePreferencesPage.english', nativeLabel: 'English' },
  { code: 'es', labelKey: 'languagePreferencesPage.spanish', nativeLabel: 'Español' },
];

const isActive = (current, code) =>
  current === code || (typeof current === 'string' && current.startsWith(code + '-'));

export default function LanguagePreferencesPage() {
  const { t, i18n } = useTranslation();
  const [savedAt, setSavedAt] = useState(null);

  const handleSelect = (code) => {
    try { localStorage.setItem('paperlms_locale', code); } catch (_) { /* ignore */ }
    i18n.changeLanguage(code);
    setSavedAt(Date.now());
  };

  return (
    <Layout>
      <div className="max-w-2xl mx-auto py-6 space-y-6">
        <header>
          <h1 className="text-xl font-semibold text-text-primary flex items-center gap-2">
            <Globe className="w-5 h-5" /> {t('languagePreferencesPage.title')}
          </h1>
          <p className="text-sm text-text-secondary mt-1">
            {t('languagePreferencesPage.subtitle')}
          </p>
        </header>

        <section
          className="border border-surface-raised rounded-lg bg-surface-0"
          role="radiogroup"
          aria-label={t('languagePreferencesPage.currentLabel')}
        >
          <div className="px-5 py-4 border-b border-surface-raised">
            <h2 className="text-sm font-semibold text-text-primary uppercase tracking-wider">
              {t('languagePreferencesPage.currentLabel')}
            </h2>
          </div>
          <ul className="divide-y divide-surface-raised">
            {LANGUAGES.map((lang) => {
              const active = isActive(i18n.language, lang.code);
              return (
                <li key={lang.code}>
                  <button
                    type="button"
                    role="radio"
                    aria-checked={active}
                    onClick={() => handleSelect(lang.code)}
                    className={`w-full flex items-center justify-between px-5 py-3 text-left transition-colors ${
                      active
                        ? 'bg-brand-50 text-brand-600'
                        : 'text-text-primary hover:bg-surface-1'
                    }`}
                  >
                    <span className="flex flex-col">
                      <span className="text-sm font-medium">{lang.nativeLabel}</span>
                      <span className="text-xs text-text-secondary">
                        {t(lang.labelKey)}
                      </span>
                    </span>
                    {active && <Check className="w-4 h-4" aria-hidden="true" />}
                  </button>
                </li>
              );
            })}
          </ul>
        </section>

        {savedAt && (
          <div
            role="status"
            aria-live="polite"
            className="flex items-center gap-1.5 text-xs text-accent-success"
          >
            <Check className="w-3.5 h-3.5" />
            <span>{t('languagePreferencesPage.savedToast')}</span>
          </div>
        )}
      </div>
    </Layout>
  );
}
