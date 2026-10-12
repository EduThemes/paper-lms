import React, { useState, useEffect, useRef } from 'react';

// Grading types whose grades are typed as text (G023): a letter or GPA
// grade, "85%", or complete / incomplete. Points take a number (or EX).
const TEXT_GRADING_TYPES = new Set(['letter_grade', 'gpa_scale', 'pass_fail', 'percent']);

const GradeInput = ({ value, pointsPossible, onSave, studentName, gradingType = 'points' }) => {
  const textMode = TEXT_GRADING_TYPES.has(gradingType);
  const [editing, setEditing] = useState(false);
  const [inputValue, setInputValue] = useState(value ?? '');
  const [saving, setSaving] = useState(false);
  const [feedback, setFeedback] = useState(null); // 'success' | 'error'
  const inputRef = useRef(null);
  const triggerRef = useRef(null);
  const wasEditingRef = useRef(false);

  useEffect(() => {
    setInputValue(value ?? '');
  }, [value]);

  useEffect(() => {
    if (editing && inputRef.current) {
      inputRef.current.focus();
      inputRef.current.select();
      wasEditingRef.current = true;
    } else if (wasEditingRef.current && triggerRef.current) {
      triggerRef.current.focus();
      wasEditingRef.current = false;
    }
  }, [editing]);

  useEffect(() => {
    if (feedback) {
      const timer = setTimeout(() => setFeedback(null), 1500);
      return () => clearTimeout(timer);
    }
  }, [feedback]);

  const handleSave = async () => {
    setEditing(false);
    const raw = String(inputValue ?? '').trim();
    // Text grades ("B+", "85%", "complete") and "EX" (excuse) go to the
    // server as typed; plain numbers are points.
    const isNumber = raw !== '' && !Number.isNaN(Number(raw));
    const nextValue = raw === '' ? null : textMode || !isNumber ? raw : parseFloat(raw);

    // Skip save if value hasn't changed
    if (nextValue === value || String(nextValue) === String(value ?? '') || (nextValue === null && (value === null || value === undefined))) {
      return;
    }

    setSaving(true);
    try {
      await onSave(nextValue);
      setFeedback('success');
    } catch (err) {
      setFeedback('error');
      setInputValue(value ?? '');
    } finally {
      setSaving(false);
    }
  };

  const handleKeyDown = (e) => {
    if (e.key === 'Enter') {
      handleSave();
    } else if (e.key === 'Escape') {
      setEditing(false);
      setInputValue(value ?? '');
    }
  };

  const feedbackClasses = {
    success: 'ring-2 ring-green-400 bg-accent-success/10',
    error: 'ring-2 ring-red-400 bg-accent-danger/10',
  };

  if (editing) {
    return (
      <div className="flex items-center space-x-1">
        <input
          ref={inputRef}
          type="text"
          inputMode={textMode ? 'text' : 'decimal'}
          aria-label={studentName ? `Grade for ${studentName}` : 'Grade'}
          value={inputValue}
          onChange={(e) => setInputValue(e.target.value)}
          onBlur={handleSave}
          onKeyDown={handleKeyDown}
          className="w-16 text-sm border border-blue-400 rounded px-1.5 py-0.5 text-center focus:ring-2 focus:ring-brand-500 focus:outline-none"
        />
        {!textMode && <span className="text-xs text-text-disabled">/{pointsPossible}</span>}
      </div>
    );
  }

  const triggerLabel = studentName ? `Edit grade for ${studentName}` : 'Edit grade';

  return (
    <button
      ref={triggerRef}
      type="button"
      disabled={saving}
      aria-label={triggerLabel}
      title={triggerLabel}
      className={`appearance-none border-0 bg-transparent text-left w-full flex items-center space-x-1 cursor-pointer rounded px-1.5 py-0.5 transition-all duration-300 focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 ${
        feedback ? feedbackClasses[feedback] : 'hover:bg-surface-2'
      }`}
      onClick={() => !saving && setEditing(true)}
    >
      <span className={`text-sm ${value !== null && value !== undefined ? 'font-medium' : 'text-text-disabled'}`}>
        {saving ? '...' : (value !== null && value !== undefined ? value : '-')}
      </span>
      {!textMode && <span className="text-xs text-text-disabled">/{pointsPossible}</span>}
    </button>
  );
};

export default GradeInput;
