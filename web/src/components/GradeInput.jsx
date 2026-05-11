import React, { useState, useEffect, useRef } from 'react';

const GradeInput = ({ value, pointsPossible, onSave, studentName }) => {
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
    const numericValue = inputValue === '' ? null : parseFloat(inputValue);

    // Skip save if value hasn't changed
    if (numericValue === value || (numericValue === null && (value === null || value === undefined))) {
      return;
    }

    setSaving(true);
    try {
      await onSave(numericValue);
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
          type="number"
          step="0.01"
          min="0"
          max={pointsPossible}
          value={inputValue}
          onChange={(e) => setInputValue(e.target.value)}
          onBlur={handleSave}
          onKeyDown={handleKeyDown}
          className="w-16 text-sm border border-blue-400 rounded px-1.5 py-0.5 text-center focus:ring-2 focus:ring-brand-500 focus:outline-none"
        />
        <span className="text-xs text-text-disabled">/{pointsPossible}</span>
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
      <span className="text-xs text-text-disabled">/{pointsPossible}</span>
    </button>
  );
};

export default GradeInput;
