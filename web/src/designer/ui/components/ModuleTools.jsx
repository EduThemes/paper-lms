/* eslint-disable react/prop-types -- Paper JSX components don't declare propTypes */
/**
 * Course-aware shortcuts above the outline: build quick links from the course's real modules, or fill
 * "This week" from one module's items (with due dates and unpublished items flagged).
 */
import { useState } from 'react';
import { CalendarRange, Layers } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { itemsToWeek, moduleLinks } from '../lib/modules';
import { useDesignerModules } from '../state/useDesignerModules';
import { newSection } from '../lib/sections';
import { Checkbox, Notice, Spinner } from './controls';
import { useDesigner } from '../state/context';
import { useDesignerT } from '../i18n';

function ModulePicker({ mode, onClose }) {
  const { t } = useDesignerT();
  const { doc, courseId, locale, dispatch, select, showToast, undo, markUnpublished } = useDesigner();
  const { modules, isLoading, isError, refetch } = useDesignerModules(courseId);
  const [picked, setPicked] = useState(null);
  const chosen = picked ?? (mode === 'links' ? modules.map((m) => m.id) : modules[0] ? [modules[0].id] : []);
  const title = mode === 'links' ? t('modules.buildLinks') : t('modules.fillWeek');

  function apply() {
    const list = modules.filter((m) => chosen.includes(m.id));
    if (!list.length) return;
    if (mode === 'links') {
      const links = moduleLinks(list);
      const existing = doc.sections.find((s) => s.type === 'quick-links');
      if (existing) dispatch({ type: 'update', id: existing.id, patch: { links } });
      else dispatch({ type: 'add', section: { ...newSection('quick-links', doc.kind, t), links } });
      showToast({ message: t('modules.linksAdded', { count: links.length }), variant: 'success', actionLabel: t('undo'), action: undo });
    } else {
      const m = list[0];
      const week = itemsToWeek(m, m.items, locale, t('modules.openModule', { module: m.name }));
      markUnpublished(week.unpublished);
      const existing = doc.sections.find((s) => s.type === 'this-week');
      if (existing) {
        dispatch({ type: 'update', id: existing.id, patch: week.patch });
        select(existing.id);
      } else {
        const s = { ...newSection('this-week', doc.kind, t), ...week.patch };
        const after = doc.sections.find((x) => x.type === 'welcome' || x.type === 'title');
        dispatch({ type: 'add', section: s, afterId: after?.id ?? null });
        select(s.id);
      }
      showToast({ message: t('modules.weekFilled', { module: m.name }), variant: 'success', actionLabel: t('undo'), action: undo });
    }
    onClose();
  }

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-h-[85vh] overflow-y-auto bg-surface-0 text-text-primary">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{mode === 'links' ? t('modules.pickModules') : t('modules.pickModule')}</DialogDescription>
        </DialogHeader>
        {isLoading ? (
          <Spinner label={t('modules.loading')} />
        ) : isError ? (
          <Notice
            variant="error"
            action={
              <Button size="sm" variant="outline" onClick={() => refetch()}>
                {t('app.tryAgain')}
              </Button>
            }
          >
            {t('modules.error')}
          </Notice>
        ) : modules.length === 0 ? (
          <p className="text-sm">{t('modules.none')}</p>
        ) : mode === 'links' ? (
          <fieldset className="space-y-2">
            <legend className="sr-only">{t('modules.pickModules')}</legend>
            {modules.map((m) => (
              <Checkbox
                key={m.id}
                label={m.published === false ? t('modules.unpublishedModule', { module: m.name }) : m.name}
                checked={chosen.includes(m.id)}
                onChange={(c) => setPicked(c ? [...chosen, m.id] : chosen.filter((x) => x !== m.id))}
              />
            ))}
            {!doc.sections.some((s) => s.type === 'quick-links') && <p className="text-xs text-text-secondary">{t('modules.noLinksSection')}</p>}
          </fieldset>
        ) : (
          <fieldset className="space-y-2">
            <legend className="sr-only">{t('modules.pickModule')}</legend>
            {modules.map((m) => (
              <label key={m.id} className="flex items-center gap-2 text-sm">
                <input
                  type="radio"
                  name="pd-week-module"
                  value={m.id}
                  checked={chosen[0] === m.id}
                  onChange={() => setPicked([m.id])}
                  className="h-4 w-4 border-border-strong text-brand-600"
                />
                {m.published === false ? t('modules.unpublishedModule', { module: m.name }) : m.name}
              </label>
            ))}
          </fieldset>
        )}
        <DialogFooter className="gap-2">
          <Button variant="outline" onClick={onClose}>
            {t('cancel')}
          </Button>
          <Button onClick={apply} disabled={!chosen.length || isLoading || isError}>
            {t('modules.apply')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export function ModuleTools() {
  const { t } = useDesignerT();
  const [open, setOpen] = useState(null);
  return (
    <section aria-labelledby="pd-module-tools" className="space-y-2 rounded-md border border-border bg-surface-1 p-3">
      <h2 id="pd-module-tools" className="text-sm font-semibold text-text-primary">
        {t('modules.tools')}
      </h2>
      <Button variant="outline" size="sm" className="w-full justify-start" onClick={() => setOpen('links')}>
        <Layers aria-hidden="true" />
        {t('modules.buildLinks')}
      </Button>
      <Button variant="outline" size="sm" className="w-full justify-start" onClick={() => setOpen('week')}>
        <CalendarRange aria-hidden="true" />
        {t('modules.fillWeek')}
      </Button>
      {open && <ModulePicker mode={open} onClose={() => setOpen(null)} />}
    </section>
  );
}
