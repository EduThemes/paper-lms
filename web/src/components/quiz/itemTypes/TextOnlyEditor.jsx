import React from 'react';
import { Info } from 'lucide-react';

/**
 * Text-only / passage editor. No answer config. The TipTap question_text
 * is rendered to students as instructional content; the item is not graded.
 */
const TextOnlyEditor = () => (
  <div className="border border-dashed border-border-default rounded-lg p-4 bg-surface-1">
    <div className="flex items-start gap-3">
      <Info className="w-5 h-5 text-text-tertiary flex-shrink-0 mt-0.5" />
      <div>
        <div className="text-sm font-medium text-text-primary mb-1">Passage / Instructions only</div>
        <p className="text-xs text-text-secondary">
          This item displays rich-text content to students but is not graded. Useful for stem passages,
          section headings, or context between scored questions.
        </p>
      </div>
    </div>
  </div>
);

export default TextOnlyEditor;
