import React from 'react';
import { Clock } from 'lucide-react';

const formatTime = (seconds) => {
  if (seconds == null) return '';
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return `${m}:${s.toString().padStart(2, '0')}`;
};

const QuizTimer = ({ timeLeft }) => {
  if (timeLeft === null || timeLeft === undefined) return null;
  const danger = timeLeft < 60;
  const warning = timeLeft < 300 && !danger;
  return (
    <div
      role="timer"
      aria-live={danger ? 'assertive' : 'off'}
      aria-label={`Time remaining: ${formatTime(timeLeft)}`}
      className={`inline-flex items-center gap-2 px-3 py-1 rounded-full text-sm font-mono font-semibold tabular-nums ${
        danger
          ? 'bg-accent-danger/20 text-accent-danger ring-1 ring-red-300'
          : warning
          ? 'bg-accent-warning/20 text-amber-800'
          : 'bg-surface-2 text-text-secondary'
      }`}
    >
      <Clock className="w-4 h-4" aria-hidden="true" />
      <span>{formatTime(timeLeft)}</span>
    </div>
  );
};

export default QuizTimer;
