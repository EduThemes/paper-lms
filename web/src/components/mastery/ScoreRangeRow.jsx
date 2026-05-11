import React from 'react';
import {
  DndContext,
  closestCenter,
  PointerSensor,
  useSensor,
  useSensors,
} from '@dnd-kit/core';
import {
  SortableContext,
  arrayMove,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { GripVertical, Trash2, Plus } from 'lucide-react';
import { Button } from '../ui/button';
import { Input } from '../ui/input';
import { Card } from '../ui/card';

/**
 * One row in the Mastery Paths editor: a single scoring range plus the
 * ordered list of assignments unlocked when a student's score falls in it.
 *
 * Props:
 *   range: { lower_bound, upper_bound, assignment_ids: number[] }
 *   index, allAssignments, onChange, onRemove, canRemove
 */
const SortableAssignmentChip = ({ assignment, onRemove }) => {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } =
    useSortable({ id: assignment.id });
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
  };
  return (
    <div
      ref={setNodeRef}
      style={style}
      className="flex items-center gap-2 px-3 py-2 bg-surface-0 border border-border-default rounded-md shadow-sm"
    >
      <button
        type="button"
        className="text-text-disabled hover:text-text-secondary cursor-grab active:cursor-grabbing"
        aria-label="Drag to reorder"
        {...attributes}
        {...listeners}
      >
        <GripVertical size={14} />
      </button>
      <span className="flex-1 text-sm text-text-primary truncate">{assignment.name}</span>
      <button
        type="button"
        onClick={() => onRemove(assignment.id)}
        className="text-text-disabled hover:text-accent-danger"
        aria-label={`Remove ${assignment.name}`}
      >
        <Trash2 size={14} />
      </button>
    </div>
  );
};

const ScoreRangeRow = ({
  range,
  index,
  allAssignments,
  onChange,
  onRemove,
  canRemove,
}) => {
  const sensors = useSensors(useSensor(PointerSensor));
  const [picker, setPicker] = React.useState('');

  const selected = (range.assignment_ids || [])
    .map((id) => allAssignments.find((a) => a.id === id))
    .filter(Boolean);

  const available = allAssignments.filter(
    (a) => !range.assignment_ids?.includes(a.id),
  );

  const handleDragEnd = (event) => {
    const { active, over } = event;
    if (!over || active.id === over.id) return;
    const oldIndex = range.assignment_ids.indexOf(active.id);
    const newIndex = range.assignment_ids.indexOf(over.id);
    onChange({
      ...range,
      assignment_ids: arrayMove(range.assignment_ids, oldIndex, newIndex),
    });
  };

  const handleAdd = () => {
    const id = parseInt(picker, 10);
    if (!id) return;
    onChange({
      ...range,
      assignment_ids: [...(range.assignment_ids || []), id],
    });
    setPicker('');
  };

  const handleRemoveChip = (id) => {
    onChange({
      ...range,
      assignment_ids: range.assignment_ids.filter((a) => a !== id),
    });
  };

  return (
    <Card className="p-4 bg-surface-1 border border-border-default">
      <div className="flex items-start justify-between gap-4 mb-3">
        <div>
          <h3 className="font-semibold text-text-primary">Range {index + 1}</h3>
          <p className="text-xs text-text-tertiary">
            Students whose score falls in this band will be assigned the work below.
          </p>
        </div>
        {canRemove && (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={onRemove}
            aria-label="Remove range"
          >
            <Trash2 size={16} />
          </Button>
        )}
      </div>

      <div className="flex items-center gap-2 mb-4">
        <div className="flex-1">
          <label className="block text-xs text-text-secondary mb-1">Lower %</label>
          <Input
            type="number"
            min="0"
            max="100"
            step="1"
            value={range.lower_bound}
            onChange={(e) =>
              onChange({ ...range, lower_bound: Number(e.target.value) })
            }
          />
        </div>
        <div className="text-text-disabled mt-5">→</div>
        <div className="flex-1">
          <label className="block text-xs text-text-secondary mb-1">Upper %</label>
          <Input
            type="number"
            min="0"
            max="100"
            step="1"
            value={range.upper_bound}
            onChange={(e) =>
              onChange({ ...range, upper_bound: Number(e.target.value) })
            }
          />
        </div>
      </div>

      <div className="space-y-2 mb-3">
        <DndContext
          sensors={sensors}
          collisionDetection={closestCenter}
          onDragEnd={handleDragEnd}
        >
          <SortableContext
            items={range.assignment_ids || []}
            strategy={verticalListSortingStrategy}
          >
            {selected.length === 0 && (
              <p className="text-sm text-text-disabled italic">
                No assignments yet — add one below.
              </p>
            )}
            {selected.map((a) => (
              <SortableAssignmentChip
                key={a.id}
                assignment={a}
                onRemove={handleRemoveChip}
              />
            ))}
          </SortableContext>
        </DndContext>
      </div>

      <div className="flex gap-2">
        <select
          value={picker}
          onChange={(e) => setPicker(e.target.value)}
          className="flex-1 border border-border-strong rounded-md px-2 py-2 text-sm"
        >
          <option value="">Select an assignment to add…</option>
          {available.map((a) => (
            <option key={a.id} value={a.id}>
              {a.name}
            </option>
          ))}
        </select>
        <Button type="button" variant="secondary" onClick={handleAdd} disabled={!picker}>
          <Plus size={14} className="mr-1" />
          Add
        </Button>
      </div>
    </Card>
  );
};

export default ScoreRangeRow;
