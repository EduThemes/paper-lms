# i18n — Paper LMS frontend translations

## Layout

- `en.json` — canonical English source. Every user-visible string the SPA
  renders is keyed here. Owned by Wave D.1 (string extraction).
- `es.json` — Spanish translations. May lag `en.json` while D.2's
  translation-fill pass is in progress.
- `glossary.md` — translator-facing reference for LMS-specific Spanish
  terms. Consult before adding new `es.json` keys.
- `index.js` — `i18next` bootstrap: resource registration, language
  detection, fallback chain.

## Fallback chain

`i18next` is configured with `fallbackLng: 'en'`. A missing key in the
active language falls back to English; a key missing from both falls
back to the literal key string (which becomes the visible-in-prod
canary for "translator forgot this one").

UI render order: **active locale → en → key**.

The active locale is selected (in priority order):
1. Per-user preference (`users.preferred_locale` once that column exists).
2. Per-tenant default (`account.default_locale`, migration 000055).
3. Browser locale (`navigator.language`) intersected with supported set.
4. Hard default `en`.

## "Spanish translation in progress" expectation

While D.1 lands new English keys ahead of D.2 filling Spanish, the
language switcher will surface English strings even with `es` selected.
This is **intentional and correct** under the fallback chain — it
beats showing the raw key (`dashboard.welcomeBack`) to a Spanish-only
user.

The user-profile-menu language switcher should NOT advertise Spanish
as "100% complete." Instead, render a small `(beta)` tag next to the
`Español` option until the D.2 translation-fill agent finishes. Once
`es.json` key count matches `en.json` key count, drop the tag.

## Adding a new string (workflow for a feature PR)

1. Add the key + English value to `en.json` under the appropriate
   namespace (`auth.*`, `course.*`, `assignments.*`, ...).
2. Wrap the call site in `t('namespace.key')`.
3. If the string is LMS-specific (assignment, submission, grade, etc.),
   consult `glossary.md` and add the Spanish value to `es.json`.
4. If the string is generic UI chrome (button labels, generic errors),
   it's acceptable to defer the Spanish translation to the next
   translation-fill pass — the fallback chain will hold the line.

## Tenant-mode-aware terms

K-12 vs higher-ed wording differs for a handful of terms (`student` →
`alumno` vs `estudiante`, `teacher` → `maestro` vs `profesor`,
`assignment` → `tarea` vs `asignación`). Where a key has both
variants, define them under a sub-namespace:

```json
{
  "people": {
    "student": {
      "k12": "Alumno/a",
      "higher_ed": "Estudiante"
    }
  }
}
```

…and pick at render time:

```js
t(`people.student.${account.tenant_mode}`, t('people.student.higher_ed'))
```

For new strings, default to the higher_ed wording and add K-12 variants
only when a tenant admin requests them. Reduces translation churn.

## Wave D.2 status (2026-05-15)

- Glossary published.
- This README in place.
- Full Spanish translation fill is the **follow-up pass** that runs
  AFTER Wave D.1 lands its new `en.json` keys. The D.2 follow-up agent
  walks every new English key and produces the matching `es.json`
  entry per the glossary.
- Existing `es.json` translations (pre-D.2) are grandfathered — see
  `glossary.md` "When the glossary disagrees with existing es.json."
