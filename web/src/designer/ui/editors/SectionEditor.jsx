/* eslint-disable react/prop-types -- Paper JSX components don't declare propTypes */
/**
 * The editor for one section, one form per section type. Typing in a field coalesces into one undo step.
 */
import { lazy, Suspense, useEffect, useRef } from 'react';
import { ChevronLeft } from 'lucide-react';
import { newId } from '../lib/ids';
import { PAPER_LINKS } from '../lib/sections';
import { Notice, Segmented, SelectField, TextAreaField, TextField } from '../components/controls';
import { ListEditor } from './ListEditor';
import { BannerFields } from './BannerFields';
import { isOwnImageSrc } from '../lib/starter';
import { IconPicker } from './IconPicker';
import { ImageUpload } from './ImageUpload';
import { useDesigner } from '../state/context';
import { useDesignerT } from '../i18n';

// The rich-text editor loads on demand, the first time a section with formatted text is opened.
const LazyRichText = lazy(() => import('./RichTextEditor').then((m) => ({ default: m.RichTextEditor })));
function RichText(props) {
  return (
    <Suspense fallback={<div className="min-h-[9rem] rounded-md border border-input" aria-busy="true" />}>
      <LazyRichText {...props} />
    </Suspense>
  );
}

function LinkTarget({ link, update }) {
  const { t } = useDesignerT();
  const preset = PAPER_LINKS.find((c) => c.href === link.href);
  const value = link.moduleId ? 'module' : preset ? preset.slot : 'custom';
  return (
    <>
      <SelectField
        label={t('field.goesTo')}
        value={value}
        onChange={(v) => {
          if (v === 'custom') update({ href: 'https://', slot: undefined, moduleId: undefined }, 'href');
          else if (v !== 'module') {
            const c = PAPER_LINKS.find((x) => x.slot === v);
            update({ href: c.href, slot: c.slot, moduleId: undefined }, 'href');
          }
        }}
        options={[
          ...(link.moduleId ? [{ value: 'module', label: t('field.fromModule') }] : []),
          ...PAPER_LINKS.map((c) => ({ value: c.slot, label: t(`slot.${c.slot}`) })),
          { value: 'custom', label: t('field.custom') },
        ]}
      />
      {value === 'custom' && <TextField label={t('field.url')} type="url" value={link.href} hint={t('field.urlHint')} onChange={(v) => update({ href: v }, 'href')} />}
    </>
  );
}

function UnpublishedNote({ id }) {
  const { t } = useDesignerT();
  const { state } = useDesigner();
  if (!state.unpublished[id]) return null;
  return <Notice variant="warning">{t('field.unpublished')}</Notice>;
}

function Fields({ s, up }) {
  const { t } = useDesignerT();
  const opt = (k) => `${t(k)} ${t('optional')}`;
  switch (s.type) {
    case 'title':
      return (
        <>
          <TextField label={t('field.titleText')} value={s.text} onChange={(v) => up({ text: v }, 'text')} required />
          <TextField label={opt('field.subtitle')} value={s.subtitle} onChange={(v) => up({ subtitle: v }, 'subtitle')} />
        </>
      );
    case 'banner':
      return <BannerFields section={s} />;
    case 'welcome':
    case 'text':
      return (
        <>
          <TextField label={s.type === 'text' ? opt('field.heading') : t('field.heading')} value={s.heading} onChange={(v) => up({ heading: v }, 'heading')} />
          <RichText label={t('field.body')} value={s.body} onChange={(v) => up({ body: v }, 'body')} />
        </>
      );
    case 'callout':
      return (
        <>
          <Segmented
            legend={t('field.variantLabel')}
            name={`variant-${s.id}`}
            value={s.variant}
            onChange={(v) => up({ variant: v }, null)}
            size="sm"
            options={['note', 'tip', 'important'].map((v) => ({ value: v, label: t(`variant.${v}`) }))}
          />
          <TextField label={t('field.heading')} value={s.heading} onChange={(v) => up({ heading: v }, 'heading')} />
          <RichText label={t('field.body')} value={s.body} onChange={(v) => up({ body: v }, 'body')} />
        </>
      );
    case 'quick-links':
      return (
        <>
          <TextField label={t('field.heading')} value={s.heading} onChange={(v) => up({ heading: v }, 'heading')} />
          <h3 className="text-sm font-medium text-text-primary">{t('field.links')}</h3>
          <ListEditor
            items={s.links}
            itemKey={(l) => l.id}
            title={(l, i) => l.label || t('field.linkN', { n: i + 1 })}
            addLabel={t('field.addLink')}
            newItem={() => ({ id: newId('l'), label: '', href: 'https://', icon: 'link' })}
            onChange={(links, key) => up({ links }, key)}
            renderItem={(l, update) => (
              <>
                <TextField label={t('field.label')} value={l.label} onChange={(v) => update({ label: v }, 'label')} required />
                <LinkTarget link={l} update={update} />
                <IconPicker label={t('field.icon')} value={l.icon} onChange={(v) => update({ icon: v }, 'icon')} />
                <TextField label={opt('field.description')} value={l.description} onChange={(v) => update({ description: v }, 'description')} />
              </>
            )}
          />
        </>
      );
    case 'instructor':
      return (
        <>
          <TextField label={t('field.heading')} value={s.heading} onChange={(v) => up({ heading: v }, 'heading')} />
          <TextField label={t('field.name')} value={s.name} onChange={(v) => up({ name: v }, 'name')} required />
          <TextField label={opt('field.role')} value={s.role} onChange={(v) => up({ role: v }, 'role')} />
          <TextField label={opt('field.email')} type="email" value={s.email} onChange={(v) => up({ email: v }, 'email')} />
          <TextAreaField label={opt('field.officeHours')} value={s.officeHours} rows={2} onChange={(v) => up({ officeHours: v }, 'officeHours')} />
          <ImageUpload label={t('field.photoUpload')} onUploaded={(src) => up({ photoSrc: src })} />
          {isOwnImageSrc(s.photoSrc) && <img src={s.photoSrc} alt="" className="h-20 w-20 rounded-full border border-border object-cover" />}
          <TextField label={opt('field.photo')} type="url" value={s.photoSrc} hint={t('field.photoHint')} onChange={(v) => up({ photoSrc: v }, 'photoSrc')} />
          {s.photoSrc && (
            <TextField
              label={t('field.photoAlt')}
              value={s.photoAlt}
              onChange={(v) => up({ photoAlt: v }, 'photoAlt')}
              required
              hint={!s.photoAlt?.trim() ? t('field.photoAltHint') : undefined}
            />
          )}
          <RichText label={t('field.bio')} value={s.bio} onChange={(v) => up({ bio: v }, 'bio')} />
        </>
      );
    case 'objectives':
      return (
        <>
          <TextField label={t('field.heading')} value={s.heading} onChange={(v) => up({ heading: v }, 'heading')} />
          <h3 className="text-sm font-medium text-text-primary">{t('field.objectives')}</h3>
          <ListEditor
            items={s.items.map((text, i) => ({ text, k: `o${i}` }))}
            itemKey={(_x, i) => `o${i}`}
            title={(_x, i) => t('field.objectiveN', { n: i + 1 })}
            addLabel={t('field.addObjective')}
            newItem={() => ({ text: '', k: '' })}
            onChange={(items, key) => up({ items: items.map((x) => x.text) }, key)}
            renderItem={(x, update, i) => (
              <TextAreaField label={t('field.objectiveN', { n: i + 1 })} labelHidden value={x.text} rows={2} onChange={(v) => update({ text: v }, 'text')} />
            )}
          />
        </>
      );
    case 'schedule':
      return (
        <>
          <TextField label={t('field.heading')} value={s.heading} onChange={(v) => up({ heading: v }, 'heading')} />
          <ListEditor
            items={s.items}
            itemKey={(x) => x.id}
            title={(x, i) => x.title || t('field.weekN', { n: i + 1 })}
            addLabel={t('field.addWeek')}
            newItem={() => ({ id: newId('i'), title: t('newSection.weekN', { n: s.items.length + 1 }), dates: '', summary: '' })}
            onChange={(items, key) => up({ items }, key)}
            renderItem={(x, update) => (
              <>
                <TextField label={t('field.itemTitle')} value={x.title} onChange={(v) => update({ title: v }, 'title')} />
                <TextField label={opt('field.dates')} value={x.dates} onChange={(v) => update({ dates: v }, 'dates')} />
                <TextAreaField label={opt('field.summary')} value={x.summary} rows={2} onChange={(v) => update({ summary: v }, 'summary')} />
                <TextField label={opt('field.link')} type="url" value={x.href} onChange={(v) => update({ href: v }, 'href')} />
              </>
            )}
          />
        </>
      );
    case 'this-week':
      return (
        <>
          <TextField label={t('field.heading')} value={s.heading} onChange={(v) => up({ heading: v }, 'heading')} />
          <TextField label={t('field.weekTitle')} value={s.title} onChange={(v) => up({ title: v }, 'title')} />
          <TextField label={opt('field.dates')} value={s.dates} onChange={(v) => up({ dates: v }, 'dates')} />
          {s.moduleId && <p className="text-xs text-text-secondary">{t('field.weekFromModule')}</p>}
          <h3 className="text-sm font-medium text-text-primary">{t('field.tasks')}</h3>
          <ListEditor
            items={s.items}
            itemKey={(x) => x.id}
            title={(x, i) => x.label || t('field.taskN', { n: i + 1 })}
            addLabel={t('field.addTask')}
            newItem={() => ({ id: newId('i'), label: '' })}
            onChange={(items, key) => up({ items }, key)}
            renderItem={(x, update) => (
              <>
                <UnpublishedNote id={x.id} />
                <TextField label={t('field.taskLabel')} value={x.label} onChange={(v) => update({ label: v }, 'label')} />
                <TextField label={opt('field.due')} value={x.due} onChange={(v) => update({ due: v }, 'due')} />
                <TextField label={opt('field.link')} type="url" value={x.href} onChange={(v) => update({ href: v }, 'href')} />
              </>
            )}
          />
        </>
      );
    case 'divider':
    default:
      return <p className="text-sm text-text-primary">{t('field.divider')}</p>;
  }
}

export function SectionEditor({ section, onDone }) {
  const { t } = useDesignerT();
  const { dispatch, nameOf } = useDesigner();
  const up = (patch, field) => dispatch({ type: 'update', id: section.id, patch, coalesce: field ?? undefined });
  const heading = useRef(null);
  // Opening a section moves focus to its heading, so keyboard and screen-reader users land in the form.
  useEffect(() => {
    heading.current?.focus();
  }, [section.id]);
  return (
    <div className="space-y-3">
      <button
        type="button"
        onClick={onDone}
        className="inline-flex items-center gap-1 rounded text-sm font-medium text-brand-600 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <ChevronLeft className="h-4 w-4" aria-hidden="true" />
        {t('customize.doneEditing')}
      </button>
      <div className="space-y-1">
        <h2 ref={heading} tabIndex={-1} className="text-base font-semibold text-text-primary focus:outline-none">
          {nameOf(section)}
        </h2>
        <p className="text-xs text-text-secondary">{t(`sectionTypeDesc.${section.type}`)}</p>
      </div>
      <form className="space-y-3" onSubmit={(e) => e.preventDefault()}>
        <Fields s={section} up={up} />
      </form>
    </div>
  );
}
