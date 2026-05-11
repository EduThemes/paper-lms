import React from 'react';
import { DndContext, closestCenter, PointerSensor, KeyboardSensor, useSensor, useSensors } from '@dnd-kit/core';
import { arrayMove, SortableContext, sortableKeyboardCoordinates, useSortable, verticalListSortingStrategy } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { GripVertical, Plus, Trash2 } from 'lucide-react';
import { makeId } from './types';

const SortableRow = ({ item, index, onChange, onRemove, canRemove }) => {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: item.id });
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
  };
  return (
    <div ref={setNodeRef} style={style} className="flex items-center gap-2 bg-surface-0 border border-border-default rounded px-2 py-1.5">
      <button
        type="button"
        {...attributes}
        {...listeners}
        className="cursor-grab active:cursor-grabbing text-text-disabled hover:text-text-secondary touch-none"
        aria-label={`Drag handle for item ${index + 1}`}
      >
        <GripVertical className="w-4 h-4" />
      </button>
      <span className="text-xs font-mono text-text-tertiary w-6 text-center">{index + 1}.</span>
      <input
        type="text"
        value={item.text}
        onChange={(e) => onChange({ ...item, text: e.target.value })}
        className="flex-1 border-0 bg-transparent px-2 py-1 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-500 rounded"
        placeholder={`Item ${index + 1}`}
      />
      {canRemove && (
        <button
          onClick={onRemove}
          className="p-1 text-text-disabled hover:text-accent-danger"
          aria-label={`Remove item ${index + 1}`}
          type="button"
        >
          <Trash2 className="w-3.5 h-3.5" />
        </button>
      )}
    </div>
  );
};

/**
 * Ordering editor. Authors drag items into the correct order. The list
 * order itself is the correct answer; on save we re-set `position`.
 */
const OrderingEditor = ({ answers, onChange }) => {
  const list = Array.isArray(answers) ? answers : [];
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

  const handleDragEnd = (event) => {
    const { active, over } = event;
    if (!over || active.id === over.id) return;
    const oldIdx = list.findIndex(i => i.id === active.id);
    const newIdx = list.findIndex(i => i.id === over.id);
    const reordered = arrayMove(list, oldIdx, newIdx).map((it, i) => ({ ...it, position: i }));
    onChange(reordered);
  };

  const updateItem = (item) => {
    onChange(list.map(i => i.id === item.id ? item : i));
  };
  const removeItem = (id) => {
    if (list.length <= 2) return;
    onChange(list.filter(i => i.id !== id).map((it, i) => ({ ...it, position: i })));
  };
  const addItem = () => {
    onChange([...list, { id: makeId('o'), text: '', position: list.length }]);
  };

  return (
    <div>
      <label className="block text-xs font-medium text-text-secondary mb-2">
        Items in correct order <span className="text-text-disabled">(drag to reorder)</span>
      </label>
      <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
        <SortableContext items={list.map(i => i.id)} strategy={verticalListSortingStrategy}>
          <div className="space-y-2">
            {list.map((item, i) => (
              <SortableRow
                key={item.id}
                item={item}
                index={i}
                onChange={updateItem}
                onRemove={() => removeItem(item.id)}
                canRemove={list.length > 2}
              />
            ))}
          </div>
        </SortableContext>
      </DndContext>
      <button
        onClick={addItem}
        className="mt-2 text-xs text-brand-600 hover:text-brand-800 flex items-center gap-1"
        type="button"
      >
        <Plus className="w-3 h-3" /> Add Item
      </button>
    </div>
  );
};

export default OrderingEditor;
