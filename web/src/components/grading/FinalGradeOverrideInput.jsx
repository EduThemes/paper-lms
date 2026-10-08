import React, { useEffect, useState } from 'react';
import { gradingPolicyApi } from '../../services/api_grading_policy';

// FinalGradeOverrideInput edits a student's final grade override (G029).
// It accepts a percentage ("91.5" or "91.5%") or a grade from the course
// scheme ("A-"); an empty value clears the override.
const FinalGradeOverrideInput = ({ courseId, student, value, grade, gradingPeriodId = 0, onSaved }) => {
  const shown = grade || (value !== null && value !== undefined ? `${value}%` : '');
  const [text, setText] = useState(shown);
  const [state, setState] = useState(null); // 'saving' | 'error'

  useEffect(() => setText(shown), [shown]);

  const save = async () => {
    const t = text.trim();
    if (t === shown) return;
    setState('saving');
    try {
      let res;
      if (t === '') {
        res = await gradingPolicyApi.setScoreOverride(courseId, student.id, { overrideScore: null, gradingPeriodId });
      } else if (/^-?\d+(\.\d+)?%?$/.test(t)) {
        res = await gradingPolicyApi.setScoreOverride(courseId, student.id, { overrideScore: parseFloat(t), gradingPeriodId });
      } else {
        res = await gradingPolicyApi.setScoreOverride(courseId, student.id, { overrideGrade: t, gradingPeriodId });
      }
      setState(null);
      onSaved?.(student.id, res);
    } catch {
      setState('error');
      setText(shown);
    }
  };

  return (
    <input
      type="text"
      value={text}
      onChange={(e) => setText(e.target.value)}
      onBlur={save}
      onKeyDown={(e) => {
        if (e.key === 'Enter') e.currentTarget.blur();
        if (e.key === 'Escape') {
          setText(shown);
          e.currentTarget.blur();
        }
      }}
      placeholder="Override"
      aria-label={`Final grade override for ${student.name}`}
      title={state === 'error' ? 'Could not save the override' : 'Final grade override (percent or grade)'}
      className={`w-16 rounded border px-1 py-0.5 text-center text-xs ${
        state === 'error' ? 'border-accent-danger' : 'border-border-strong'
      } ${state === 'saving' ? 'opacity-50' : ''}`}
    />
  );
};

export default FinalGradeOverrideInput;
