import React from 'react';
import { Plus, Trash2, Check } from 'lucide-react';
import { makeId, normalizeQuestionType } from './types';
import MultipleAnswerEditor from './MultipleAnswerEditor';
import MultipleDropdownEditor from './MultipleDropdownEditor';
import FillInBlankEditor from './FillInBlankEditor';
import FormulaEditor from './FormulaEditor';
import FileUploadEditor from './FileUploadEditor';
import OrderingEditor from './OrderingEditor';
import CategorizationEditor from './CategorizationEditor';
import HotSpotEditor from './HotSpotEditor';
import TextOnlyEditor from './TextOnlyEditor';
import MatchingEditor from './MatchingEditor';
import FillInMultipleBlanksEditor from './FillInMultipleBlanksEditor';
import NumericalEditor from './NumericalEditor';

/**
 * ItemAnswersEditor — the answer-key editor for any item type, shared by
 * the question-bank page (and anything else that authors a question
 * outside the quiz editor). `answers` is the parsed answers value.
 */

// Single-correct choices (multiple choice, true/false).
export const ChoiceEditor = ({ answers, onChange, fixed = false }) => {
  const list = Array.isArray(answers) ? answers : [];
  const markCorrect = (i) => onChange(list.map((a, idx) => ({ ...a, weight: idx === i ? 100 : 0 })));
  const setText = (i, text) => onChange(list.map((a, idx) => (idx === i ? { ...a, text } : a)));
  return (
    <div>
      <span className="block text-xs font-medium text-text-secondary mb-2">Answers <span className="text-text-tertiary">(mark the correct one)</span></span>
      <div className="space-y-2">
        {list.map((a, i) => (
          <div key={a.id || i} className="flex items-center gap-2">
            <button type="button" onClick={() => markCorrect(i)} aria-pressed={a.weight > 0}
              aria-label={`Mark answer ${i + 1} correct`}
              className={`w-6 h-6 rounded-full border-2 flex items-center justify-center flex-shrink-0 ${a.weight > 0 ? 'border-accent-success bg-accent-success text-on-accent' : 'border-border-strong'}`}>
              {a.weight > 0 && <Check className="w-3 h-3" />}
            </button>
            <input type="text" value={a.text ?? ''} readOnly={fixed} onChange={(e) => setText(i, e.target.value)}
              className="flex-1 border border-border-strong rounded px-3 py-1.5 text-sm bg-surface-0 text-text-primary"
              placeholder={`Answer ${i + 1}`} aria-label={`Answer ${i + 1}`} />
            {!fixed && list.length > 2 && (
              <button type="button" onClick={() => onChange(list.filter((_, idx) => idx !== i))}
                className="p-1 text-text-tertiary hover:text-accent-danger" aria-label={`Remove answer ${i + 1}`}>
                <Trash2 className="w-3.5 h-3.5" />
              </button>
            )}
          </div>
        ))}
      </div>
      {!fixed && (
        <button type="button" onClick={() => onChange([...list, { id: makeId('a'), text: '', weight: 0 }])}
          className="mt-2 text-xs text-brand-600 hover:text-brand-800 flex items-center gap-1">
          <Plus className="w-3 h-3" /> Add Answer
        </button>
      )}
    </div>
  );
};

// Accepted typed answers (short answer): every option is full credit.
export const AcceptedAnswersEditor = ({ answers, onChange }) => {
  const list = Array.isArray(answers) ? answers : [];
  return (
    <div>
      <span className="block text-xs font-medium text-text-secondary mb-2">Acceptable answers <span className="text-text-tertiary">(case-insensitive)</span></span>
      <div className="space-y-2">
        {list.map((a, i) => (
          <div key={a.id || i} className="flex items-center gap-2">
            <input type="text" value={a.text ?? ''} onChange={(e) => onChange(list.map((x, idx) => (idx === i ? { ...x, text: e.target.value } : x)))}
              className="flex-1 border border-border-strong rounded px-3 py-1.5 text-sm bg-surface-0 text-text-primary"
              placeholder="Acceptable answer" aria-label={`Acceptable answer ${i + 1}`} />
            {list.length > 1 && (
              <button type="button" onClick={() => onChange(list.filter((_, idx) => idx !== i))}
                className="p-1 text-text-tertiary hover:text-accent-danger" aria-label={`Remove acceptable answer ${i + 1}`}>
                <Trash2 className="w-3.5 h-3.5" />
              </button>
            )}
          </div>
        ))}
      </div>
      <button type="button" onClick={() => onChange([...list, { id: makeId('a'), text: '', weight: 100 }])}
        className="mt-2 text-xs text-brand-600 hover:text-brand-800 flex items-center gap-1">
        <Plus className="w-3 h-3" /> Add Acceptable Answer
      </button>
    </div>
  );
};

const ItemAnswersEditor = ({ type, answers, onChange, questionText, courseId, quizId }) => {
  switch (normalizeQuestionType(type)) {
    case 'multiple_choice':
      return <ChoiceEditor answers={answers} onChange={onChange} />;
    case 'true_false':
      return <ChoiceEditor answers={answers} onChange={onChange} fixed />;
    case 'short_answer':
      return <AcceptedAnswersEditor answers={answers} onChange={onChange} />;
    case 'essay':
      return <p className="text-xs text-text-tertiary italic">Essay questions are graded by hand.</p>;
    case 'numerical_question':
      return <NumericalEditor answers={answers} onChange={onChange} />;
    case 'matching':
      return <MatchingEditor answers={answers} onChange={onChange} />;
    case 'fill_in_multiple_blanks':
      return <FillInMultipleBlanksEditor answers={answers} onChange={onChange} questionText={questionText} />;
    case 'multiple_answer':
      return <MultipleAnswerEditor answers={answers} onChange={onChange} />;
    case 'multiple_dropdown':
      return <MultipleDropdownEditor answers={answers} onChange={onChange} questionText={questionText} />;
    case 'fill_in_the_blank':
      return <FillInBlankEditor answers={answers} onChange={onChange} />;
    case 'formula':
      return <FormulaEditor answers={answers} onChange={onChange} courseId={courseId} quizId={quizId} />;
    case 'file_upload':
      return <FileUploadEditor />;
    case 'ordering':
      return <OrderingEditor answers={answers} onChange={onChange} />;
    case 'categorization':
      return <CategorizationEditor answers={answers} onChange={onChange} />;
    case 'hot_spot':
      return <HotSpotEditor answers={answers} onChange={onChange} courseId={courseId} />;
    case 'text_only':
      return <TextOnlyEditor />;
    default:
      return <p className="text-xs text-accent-warning">This question type has no answer editor.</p>;
  }
};

export default ItemAnswersEditor;
