/* eslint-disable react/prop-types -- Paper JSX components don't declare propTypes */
/**
 * The page outline: every section in order, with drag-and-drop (pointer and keyboard, via dnd-kit), move
 * up/down buttons, duplicate and delete. Moves are announced to screen readers.
 */
import { DndContext, KeyboardSensor, PointerSensor, closestCenter, useSensor, useSensors } from '@dnd-kit/core';
import { SortableContext, sortableKeyboardCoordinates, useSortable, verticalListSortingStrategy } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { ArrowDown, ArrowUp, Copy, GripVertical, Plus, Trash2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { cn } from '@/lib/utils';
import { SECTION_TYPES } from '../lib/engine';
import { useDesigner } from '../state/context';
import { useDesignerT } from '../i18n';

function IconAction({ label, onClick, disabled, children }) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      title={label}
      className="inline-flex h-8 w-8 items-center justify-center rounded-md text-text-secondary hover:bg-surface-2 hover:text-text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-40"
    >
      {children}
      <span className="sr-only">{label}</span>
    </button>
  );
}

function OutlineItem({ section, index, total }) {
  const { t } = useDesignerT();
  const { state, select, moveSection, duplicateSection, deleteSection, nameOf } = useDesigner();
  const { attributes, listeners, setNodeRef, setActivatorNodeRef, transform, transition, isDragging } = useSortable({ id: section.id });
  const name = nameOf(section);
  const selected = state.selectedId === section.id;

  return (
    <li
      ref={setNodeRef}
      style={{ transform: CSS.Transform.toString(transform), transition }}
      className={cn(
        'flex items-center gap-1 rounded-md border bg-surface-0 py-1 pl-1 pr-1',
        selected ? 'border-brand-600 ring-1 ring-brand-600' : 'border-border',
        isDragging && 'relative z-10 shadow-md',
      )}
    >
      <button
        type="button"
        ref={setActivatorNodeRef}
        {...attributes}
        {...listeners}
        aria-label={t('customize.dragHandle', { name })}
        className="inline-flex h-8 w-6 shrink-0 cursor-grab items-center justify-center rounded text-text-tertiary hover:text-text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring active:cursor-grabbing"
      >
        <GripVertical className="h-4 w-4" aria-hidden="true" />
      </button>
      <button
        type="button"
        onClick={() => select(section.id)}
        title={name}
        aria-label={t('customize.edit', { name })}
        className="min-w-0 flex-1 truncate rounded px-1 py-1 text-left text-sm text-text-primary hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        {name}
      </button>
      <span className="flex shrink-0">
        <IconAction label={t('customize.moveUp', { name })} disabled={index === 0} onClick={() => moveSection(section.id, index - 1)}>
          <ArrowUp className="h-4 w-4" aria-hidden="true" />
        </IconAction>
        <IconAction label={t('customize.moveDown', { name })} disabled={index === total - 1} onClick={() => moveSection(section.id, index + 1)}>
          <ArrowDown className="h-4 w-4" aria-hidden="true" />
        </IconAction>
        <IconAction label={t('customize.duplicate', { name })} onClick={() => duplicateSection(section.id)}>
          <Copy className="h-4 w-4" aria-hidden="true" />
        </IconAction>
        <IconAction label={t('customize.delete', { name })} onClick={() => deleteSection(section.id)}>
          <Trash2 className="h-4 w-4" aria-hidden="true" />
        </IconAction>
      </span>
    </li>
  );
}

export function AddSectionMenu({ afterId = null }) {
  const { t } = useDesignerT();
  const { addSection } = useDesigner();
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" className="w-full justify-start">
          <Plus aria-hidden="true" />
          {t('customize.addSection')}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="max-h-[60vh] w-80 overflow-y-auto">
        <DropdownMenuLabel>{t('customize.addSectionMenu')}</DropdownMenuLabel>
        {SECTION_TYPES.map((st) => (
          <DropdownMenuItem key={st.type} onSelect={() => addSection(st.type, afterId)} className="flex-col items-start gap-0.5">
            <span className="font-semibold">{t(`sectionType.${st.type}`)}</span>
            <span className="whitespace-normal text-xs text-muted-foreground">{t(`sectionTypeDesc.${st.type}`)}</span>
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export function Outline() {
  const { t } = useDesignerT();
  const { doc, moveSection, nameOf } = useDesigner();
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );
  if (!doc) return null;
  const sections = doc.sections;
  const label = (id) => {
    const s = sections.find((x) => x.id === id);
    return s ? nameOf(s) : '';
  };
  const position = (id) => sections.findIndex((x) => x.id === id) + 1;
  const total = sections.length;

  const announcements = {
    onDragStart: ({ active }) => t('customize.dragStart', { name: label(active.id) }),
    onDragOver: ({ active, over }) => (over ? t('customize.dragOver', { name: label(active.id), n: position(over.id), total }) : undefined),
    onDragEnd: ({ active, over }) =>
      over ? t('customize.dragEnd', { name: label(active.id), n: position(over.id), total }) : t('customize.dragEndNowhere', { name: label(active.id) }),
    onDragCancel: ({ active }) => t('customize.dragCancel', { name: label(active.id) }),
  };

  function onDragEnd(e) {
    if (!e.over || e.active.id === e.over.id) return;
    const to = sections.findIndex((s) => s.id === e.over.id);
    if (to >= 0) moveSection(String(e.active.id), to);
  }

  return (
    <section aria-labelledby="pd-outline-h" className="space-y-3">
      <div>
        <h2 id="pd-outline-h" className="text-base font-semibold text-text-primary">
          {t('customize.outline')}
        </h2>
        <p className="text-xs text-text-secondary">{t('customize.outlineHint')}</p>
      </div>
      {sections.length === 0 ? (
        <p className="text-sm text-text-primary">{t('customize.empty')}</p>
      ) : (
        <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={onDragEnd} accessibility={{ announcements }}>
          <SortableContext items={sections.map((s) => s.id)} strategy={verticalListSortingStrategy}>
            <ol className="space-y-1.5">
              {sections.map((s, i) => (
                <OutlineItem key={s.id} section={s} index={i} total={total} />
              ))}
            </ol>
          </SortableContext>
        </DndContext>
      )}
      <AddSectionMenu afterId={sections.at(-1)?.id ?? null} />
    </section>
  );
}
