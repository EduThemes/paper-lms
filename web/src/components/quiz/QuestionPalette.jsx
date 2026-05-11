import React, { useState } from 'react';
import { Grid3x3 } from 'lucide-react';

const QuestionButton = ({ idx, current, answered, onClick }) => (
  <button
    type="button"
    onClick={() => onClick(idx)}
    aria-current={current ? 'true' : undefined}
    aria-label={`Question ${idx + 1}${answered ? ', answered' : ', unanswered'}${current ? ', current' : ''}`}
    className={`w-9 h-9 rounded text-sm font-medium focus:outline-none focus:ring-2 focus:ring-brand-500 focus:ring-offset-1 transition-colors ${
      current
        ? 'bg-brand-600 text-white'
        : answered
        ? 'bg-accent-success/20 text-accent-success border border-green-300 hover:bg-green-200'
        : 'bg-surface-2 text-text-secondary border border-border-strong hover:bg-border-default'
    }`}
  >
    {idx + 1}
  </button>
);

const QuestionPalette = ({ questions, currentIdx, answers, onJump }) => {
  const [mobileOpen, setMobileOpen] = useState(false);

  const grid = (
    <div role="list" aria-label="Question navigation" className="grid grid-cols-5 gap-2">
      {questions.map((q, idx) => (
        <div role="listitem" key={q.id}>
          <QuestionButton
            idx={idx}
            current={idx === currentIdx}
            answered={Boolean(answers[q.id])}
            onClick={(i) => {
              onJump(i);
              setMobileOpen(false);
            }}
          />
        </div>
      ))}
    </div>
  );

  return (
    <>
      {/* Mobile trigger */}
      <div className="md:hidden mb-3">
        <button
          type="button"
          onClick={() => setMobileOpen((v) => !v)}
          aria-expanded={mobileOpen}
          aria-controls="question-palette-mobile"
          className="inline-flex items-center gap-2 px-3 py-1.5 rounded border border-border-strong bg-surface-0 text-sm font-medium hover:bg-surface-1"
        >
          <Grid3x3 className="w-4 h-4" aria-hidden="true" />
          Q{currentIdx + 1} of {questions.length}
        </button>
        {mobileOpen && (
          <div
            id="question-palette-mobile"
            className="mt-2 p-3 bg-surface-0 border border-border-default rounded-lg shadow-sm"
          >
            {grid}
          </div>
        )}
      </div>

      {/* Desktop sticky aside */}
      <aside
        aria-label="Question palette"
        className="hidden md:block sticky top-16 self-start max-h-[calc(100vh-5rem)] overflow-y-auto p-3 bg-surface-0 border border-border-default rounded-lg shadow-sm w-48"
      >
        <div className="text-xs uppercase tracking-wide text-text-tertiary font-semibold mb-2">
          Questions
        </div>
        {grid}
        <div className="mt-3 pt-3 border-t border-border-subtle text-xs text-text-tertiary space-y-1">
          <div className="flex items-center gap-1.5">
            <span className="w-3 h-3 rounded bg-accent-success/20 border border-green-300" /> Answered
          </div>
          <div className="flex items-center gap-1.5">
            <span className="w-3 h-3 rounded bg-surface-2 border border-border-strong" /> Unanswered
          </div>
        </div>
      </aside>
    </>
  );
};

export default QuestionPalette;
