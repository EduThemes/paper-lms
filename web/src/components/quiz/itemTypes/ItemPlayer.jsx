import React, { useMemo, useRef, useState } from 'react';
import { DndContext, closestCenter, PointerSensor, KeyboardSensor, useSensor, useSensors } from '@dnd-kit/core';
import { arrayMove, SortableContext, sortableKeyboardCoordinates, useSortable, verticalListSortingStrategy } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { GripVertical, Upload, AlertCircle } from 'lucide-react';
import { sanitizeHTML } from '../../RichContentViewer';
import { parseAnswers } from './types';
import { extractBlankIds } from './MultipleDropdownEditor';

/**
 * ItemPlayer — renders the student-facing UI for a quiz question based on
 * `question.question_type`. Receives the current `value` for this question
 * (whatever shape was last passed to `onChange`) and reports updates via
 * `onChange(value)`. Disabled when `readOnly`.
 *
 * Shapes per type:
 *   multiple_choice / true_false        -> string (answer id)
 *   essay / short_answer / fill_in_blank/ formula -> string
 *   numerical_question                  -> string (numeric)
 *   multiple_answer                     -> array of ids
 *   multiple_dropdown                   -> { blank_id: answerId }
 *   ordering                            -> array of item ids (current order)
 *   categorization                      -> { item_id: bucket_id }
 *   hot_spot                            -> { x: 0..1, y: 0..1 }
 *   file_upload                         -> { file_id, filename }
 *   text_only                           -> n/a
 */

const SortableItem = ({ id, text, index }) => {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id });
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
  };
  return (
    <div ref={setNodeRef} style={style}
      className="flex items-center gap-2 bg-surface-0 border border-border-default rounded px-3 py-2">
      <button type="button" {...attributes} {...listeners}
        className="cursor-grab active:cursor-grabbing text-text-disabled hover:text-text-secondary touch-none"
        aria-label="Reorder">
        <GripVertical className="w-4 h-4" />
      </button>
      <span className="text-xs font-mono text-text-tertiary w-6 text-center">{index + 1}.</span>
      <span className="flex-1 text-sm text-text-primary">{text}</span>
    </div>
  );
};

const renderInlineTextWithBlanks = (htmlOrText, blankIds, value, onChange, options, readOnly) => {
  // Split rendered text by blank tokens to interleave <select>s.
  const plain = String(htmlOrText || '').replace(/<[^>]*>/g, ' ');
  const segments = plain.split(/(\[[a-zA-Z][\w-]*\])/);
  const v = value || {};
  return (
    <span className="leading-loose">
      {segments.map((seg, i) => {
        const m = seg.match(/^\[([a-zA-Z][\w-]*)\]$/);
        if (m && blankIds.includes(m[1])) {
          const bid = m[1];
          const opts = options.filter(o => o.blank_id === bid);
          return (
            <select
              key={`b-${bid}-${i}`}
              disabled={readOnly}
              value={v[bid] || ''}
              onChange={(e) => onChange({ ...v, [bid]: e.target.value })}
              className="mx-1 inline-block border border-border-strong rounded px-2 py-1 text-sm bg-surface-0 text-text-primary"
              aria-label={`Blank ${bid}`}
            >
              <option value="">Choose…</option>
              {opts.map(o => <option key={o.id} value={o.id}>{o.text}</option>)}
            </select>
          );
        }
        return <span key={`t-${i}`}>{seg}</span>;
      })}
    </span>
  );
};

const ItemPlayer = ({ question, value, onChange, readOnly = false, onFileUpload }) => {
  const qid = question.id;
  const labelId = `q-${qid}-label`;
  const answers = useMemo(() => parseAnswers(question.answers, []), [question.answers]);

  switch (question.question_type) {
    case 'multiple_choice':
    case 'true_false':
      return (
        <div role="radiogroup" aria-labelledby={labelId} className="space-y-2 max-w-prose">
          {answers.map(opt => {
            const checked = value === opt.id;
            return (
              <label key={opt.id}
                className={`flex items-center gap-3 p-3 rounded border cursor-pointer transition-colors ${
                  checked ? 'border-brand-500 bg-brand-50 dark:bg-brand-500/10' : 'border-border-default hover:bg-surface-1'
                }`}>
                <input type="radio" name={`q-${qid}`} checked={checked} disabled={readOnly}
                  onChange={() => onChange(opt.id)} className="text-brand-600" />
                <span className="text-text-primary">{opt.text}</span>
              </label>
            );
          })}
        </div>
      );

    case 'short_answer':
    case 'essay':
      return (
        <textarea
          className="w-full max-w-prose border border-border-strong rounded p-3 min-h-[100px] bg-surface-0 text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
          placeholder="Type your answer..."
          disabled={readOnly}
          value={value || ''}
          onChange={(e) => onChange(e.target.value)}
          rows={question.question_type === 'essay' ? 8 : 3}
          aria-labelledby={labelId}
        />
      );

    case 'numerical_question':
      return (
        <input type="number"
          className="w-full max-w-xs border border-border-strong rounded p-3 bg-surface-0 text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
          placeholder="Enter a number..."
          disabled={readOnly}
          value={value || ''}
          onChange={(e) => onChange(e.target.value)}
          aria-labelledby={labelId} />
      );

    case 'multiple_answer': {
      const arr = Array.isArray(value) ? value : [];
      const toggle = (id) => {
        if (readOnly) return;
        onChange(arr.includes(id) ? arr.filter(x => x !== id) : [...arr, id]);
      };
      return (
        <div role="group" aria-labelledby={labelId} className="space-y-2 max-w-prose">
          {answers.map(opt => {
            const checked = arr.includes(opt.id);
            return (
              <label key={opt.id}
                className={`flex items-center gap-3 p-3 rounded border cursor-pointer transition-colors ${
                  checked ? 'border-brand-500 bg-brand-50 dark:bg-brand-500/10' : 'border-border-default hover:bg-surface-1'
                }`}>
                <input type="checkbox" checked={checked} disabled={readOnly}
                  onChange={() => toggle(opt.id)} className="text-brand-600" />
                <span className="text-text-primary">{opt.text}</span>
              </label>
            );
          })}
        </div>
      );
    }

    case 'multiple_dropdown': {
      const blankIds = extractBlankIds(question.question_text);
      return (
        <div className="max-w-prose">
          <div className="prose prose-sm max-w-prose mb-2 text-text-primary"
               // Strip blank tokens from the rendered HTML so they don't appear twice
               dangerouslySetInnerHTML={{ __html: sanitizeHTML(String(question.question_text || '').replace(/\[[a-zA-Z][\w-]*\]/g, '___')) }} />
          <div className="text-sm text-text-primary">
            {renderInlineTextWithBlanks(question.question_text, blankIds, value, onChange, answers, readOnly)}
          </div>
        </div>
      );
    }

    case 'fill_in_the_blank':
      return (
        <input type="text"
          className="w-full max-w-md border border-border-strong rounded p-3 bg-surface-0 text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
          placeholder="Type your answer..."
          disabled={readOnly}
          value={value || ''}
          onChange={(e) => onChange(e.target.value)}
          aria-labelledby={labelId} />
      );

    case 'formula':
      return (
        <input type="number"
          step="any"
          className="w-full max-w-xs border border-border-strong rounded p-3 bg-surface-0 text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
          placeholder="Enter a numeric answer..."
          disabled={readOnly}
          value={value || ''}
          onChange={(e) => onChange(e.target.value)}
          aria-labelledby={labelId} />
      );

    case 'file_upload': {
      const filename = value?.filename || '';
      return (
        <FileUploadPlayer
          readOnly={readOnly}
          value={value}
          onChange={onChange}
          onUpload={onFileUpload}
          labelId={labelId}
        />
      );
    }

    case 'ordering':
      return <OrderingPlayer answers={answers} value={value} onChange={onChange} readOnly={readOnly} labelId={labelId} />;

    case 'categorization':
      return <CategorizationPlayer cfg={answers[0] || {}} value={value} onChange={onChange} readOnly={readOnly} labelId={labelId} />;

    case 'hot_spot':
      return <HotSpotPlayer cfg={answers[0] || {}} value={value} onChange={onChange} readOnly={readOnly} labelId={labelId} />;

    case 'text_only':
      return (
        <div className="text-xs text-text-tertiary italic">— Informational passage; no response required. —</div>
      );

    default:
      return (
        <div className="text-xs text-accent-warning bg-accent-warning/10 border border-accent-warning/30 rounded p-2 inline-flex items-center gap-1">
          <AlertCircle className="w-3.5 h-3.5" /> Unknown question type: {question.question_type}
        </div>
      );
  }
};

const OrderingPlayer = ({ answers, value, onChange, readOnly, labelId }) => {
  // value is an array of item ids representing the student's current order.
  // Initialize from answers in source order on first render.
  const initial = useMemo(() => {
    if (Array.isArray(value) && value.length === answers.length) return value;
    return answers.map(a => a.id);
  }, [answers, value]);
  const order = Array.isArray(value) && value.length === answers.length ? value : initial;

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

  const handleDragEnd = (event) => {
    if (readOnly) return;
    const { active, over } = event;
    if (!over || active.id === over.id) return;
    const oldIdx = order.indexOf(active.id);
    const newIdx = order.indexOf(over.id);
    onChange(arrayMove(order, oldIdx, newIdx));
  };

  const byId = Object.fromEntries(answers.map(a => [a.id, a]));

  return (
    <div className="max-w-prose" aria-labelledby={labelId}>
      <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
        <SortableContext items={order} strategy={verticalListSortingStrategy}>
          <div className="space-y-2">
            {order.map((id, i) => (
              <SortableItem key={id} id={id} text={byId[id]?.text || '(missing)'} index={i} />
            ))}
          </div>
        </SortableContext>
      </DndContext>
    </div>
  );
};

const CategorizationPlayer = ({ cfg, value, onChange, readOnly, labelId }) => {
  const items = cfg.items || [];
  const buckets = cfg.buckets || [];
  const v = value || {};

  const setAssignment = (itemId, bucketId) => {
    if (readOnly) return;
    if (!bucketId) {
      const { [itemId]: _drop, ...rest } = v;
      onChange(rest);
    } else {
      onChange({ ...v, [itemId]: bucketId });
    }
  };

  return (
    <div className="max-w-prose" aria-labelledby={labelId}>
      <p className="text-xs text-text-tertiary mb-3">Assign each item to a bucket:</p>
      <ul className="space-y-2">
        {items.map(item => (
          <li key={item.id} className="flex items-center gap-3 border border-border-default rounded p-2 bg-surface-0">
            <span className="flex-1 text-sm text-text-primary">{item.text}</span>
            <select
              disabled={readOnly}
              value={v[item.id] || ''}
              onChange={(e) => setAssignment(item.id, e.target.value)}
              className="border border-border-strong rounded px-2 py-1 text-sm bg-surface-0 text-text-primary"
              aria-label={`Bucket for ${item.text}`}
            >
              <option value="">— Choose bucket —</option>
              {buckets.map(b => (
                <option key={b.id} value={b.id}>{b.label}</option>
              ))}
            </select>
          </li>
        ))}
      </ul>
    </div>
  );
};

const HotSpotPlayer = ({ cfg, value, onChange, readOnly, labelId }) => {
  const imgRef = useRef(null);
  const v = value && typeof value.x === 'number' ? value : null;

  const handleClick = (e) => {
    if (readOnly) return;
    const rect = imgRef.current?.getBoundingClientRect();
    if (!rect) return;
    onChange({
      x: (e.clientX - rect.left) / rect.width,
      y: (e.clientY - rect.top) / rect.height,
    });
  };

  if (!cfg.image_url) {
    return <p className="text-xs text-accent-warning">Hot-spot image not configured.</p>;
  }
  return (
    <div className="relative inline-block border border-border-strong rounded overflow-hidden bg-surface-1 select-none"
         aria-labelledby={labelId}>
      <img ref={imgRef} src={cfg.image_url} alt="Hot-spot question"
           onClick={handleClick}
           className="block max-w-full max-h-[400px] cursor-crosshair"
           draggable={false} />
      {v && (
        <div
          className="absolute w-4 h-4 rounded-full -translate-x-1/2 -translate-y-1/2 bg-brand-600 border-2 border-white pointer-events-none"
          style={{ left: `${v.x * 100}%`, top: `${v.y * 100}%` }}
          aria-label="Selected point"
        />
      )}
    </div>
  );
};

const FileUploadPlayer = ({ value, onChange, readOnly, onUpload, labelId }) => {
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState(null);

  const handleFile = async (file) => {
    if (!file) return;
    setBusy(true); setErr(null);
    try {
      if (onUpload) {
        const result = await onUpload(file);
        onChange(result);
      } else {
        // Fallback: store the filename for display only.
        onChange({ filename: file.name });
      }
    } catch (e) {
      setErr(e.message || 'Upload failed');
    } finally {
      setBusy(false);
    }
  };

  return (
    <div aria-labelledby={labelId}>
      <label className={`inline-flex items-center gap-2 px-3 py-2 text-sm rounded border border-border-strong bg-surface-0 hover:bg-surface-1 cursor-pointer ${readOnly ? 'opacity-50 cursor-not-allowed' : ''}`}>
        <Upload className="w-4 h-4" />
        {value?.filename ? 'Replace file' : 'Choose file'}
        <input type="file" className="hidden" disabled={readOnly || busy}
               onChange={(e) => handleFile(e.target.files?.[0])} />
      </label>
      {busy && <span className="ml-2 text-xs text-text-tertiary">Uploading…</span>}
      {err && <span className="ml-2 text-xs text-accent-danger">{err}</span>}
      {value?.filename && !busy && (
        <span className="ml-3 text-xs text-text-secondary">Selected: <strong>{value.filename}</strong></span>
      )}
    </div>
  );
};

export default ItemPlayer;
