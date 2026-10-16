/* eslint-disable react/prop-types -- Paper JSX components don't declare propTypes */
/**
 * Advanced → Custom CSS: the teacher's own stylesheet for this page, stored in the design (`doc.customCss`)
 * and appended after the designer's CSS when the page is saved. Only offered when the course's account
 * allows hand-written CSS (`course.custom_css_enabled`). The server sanitizes and scopes it on save.
 */
import { useId, useRef } from 'react';
import { Trans } from 'react-i18next';
import { useDesigner } from '../state/context';
import { useDesignerT } from '../i18n';

const code = <code className="rounded bg-surface-2 px-1 font-mono text-[0.85em]" />;

export function CustomCssPanel() {
  const { t } = useDesignerT();
  const { doc, dispatch } = useDesigner();
  const id = useId();
  const gutter = useRef(null);
  const value = doc?.customCss ?? '';
  const lines = Math.max(1, value.split('\n').length);

  return (
    <section aria-labelledby={`${id}-h`} className="space-y-3">
      <div className="space-y-1.5">
        <h2 id={`${id}-h`} className="text-base font-semibold text-text-primary">
          {t('advanced.title')}
        </h2>
        <p className="text-sm text-text-secondary">{t('advanced.intro')}</p>
        <ul className="list-disc space-y-1 pl-5 text-xs text-text-secondary">
          <li>
            <Trans t={t} i18nKey="advanced.root" components={{ code }} />
          </li>
          <li>
            <Trans t={t} i18nKey="advanced.dark" components={{ code }} />
          </li>
          <li>{t('advanced.limits')}</li>
        </ul>
      </div>
      <div className="space-y-1.5">
        <label htmlFor={id} className="text-sm font-medium text-text-primary">
          {t('advanced.label')}
        </label>
        <div className="flex overflow-hidden rounded-md border border-input bg-surface-0 focus-within:ring-2 focus-within:ring-ring">
          <pre
            ref={gutter}
            aria-hidden="true"
            className="m-0 select-none overflow-hidden border-r border-border bg-surface-1 px-2 py-2 text-right font-mono text-xs leading-5 text-text-tertiary"
          >
            {Array.from({ length: lines }, (_, i) => i + 1).join('\n')}
          </pre>
          <textarea
            id={id}
            value={value}
            rows={16}
            spellCheck={false}
            autoCapitalize="off"
            autoComplete="off"
            wrap="off"
            aria-describedby={`${id}-count`}
            onScroll={(e) => {
              if (gutter.current) gutter.current.scrollTop = e.currentTarget.scrollTop;
            }}
            onChange={(e) => dispatch({ type: 'setDoc', patch: { customCss: e.target.value }, coalesce: 'customCss', at: Date.now() })}
            onKeyDown={(e) => {
              // Tab inserts two spaces; Escape then Tab leaves the field (keyboard users aren't trapped).
              if (e.key === 'Tab' && !e.shiftKey && !e.currentTarget.dataset.escaped) {
                e.preventDefault();
                const el = e.currentTarget;
                const { selectionStart: a, selectionEnd: b } = el;
                const next = `${value.slice(0, a)}  ${value.slice(b)}`;
                dispatch({ type: 'setDoc', patch: { customCss: next }, coalesce: 'customCss', at: Date.now() });
                requestAnimationFrame(() => el.setSelectionRange(a + 2, a + 2));
              }
              if (e.key === 'Escape') e.currentTarget.dataset.escaped = '1';
              else if (e.key !== 'Tab') delete e.currentTarget.dataset.escaped;
            }}
            onBlur={(e) => delete e.currentTarget.dataset.escaped}
            className="min-h-[16rem] flex-1 resize-y bg-transparent px-3 py-2 font-mono text-xs leading-5 text-text-primary outline-none"
          />
        </div>
        <p id={`${id}-count`} className="text-xs text-text-secondary">
          {t('advanced.lines', { count: lines })}
        </p>
      </div>
    </section>
  );
}
