import React from 'react';

/**
 * PictureCue — child-friendly emoji + colored tile + label.
 * Decorative emoji is aria-hidden; the visible label is the accessible name.
 */
const CUES = {
  home:        { emoji: '🏠', surface: 'bg-yellow-200 text-yellow-900' },
  classes:     { emoji: '📚', surface: 'bg-sky-200 text-sky-900' },
  messages:    { emoji: '💬', surface: 'bg-green-200 text-green-900' },
  assignments: { emoji: '✏️', surface: 'bg-orange-200 text-orange-900' },
  today:       { emoji: '☀️', surface: 'bg-yellow-100 text-yellow-900' },
  quiz:        { emoji: '🎯', surface: 'bg-purple-200 text-purple-900' },
  discussion:  { emoji: '💭', surface: 'bg-pink-200 text-pink-900' },
  pages:       { emoji: '📄', surface: 'bg-teal-200 text-teal-900' },
  grades:      { emoji: '⭐', surface: 'bg-amber-200 text-amber-900' },
  logout:      { emoji: '👋', surface: 'bg-gray-200 text-gray-900' },
};

const PictureCue = ({ type, label, className = '' }) => {
  const cue = CUES[type] || CUES.home;
  return (
    <span className={`flex flex-col items-center gap-1 ${className}`}>
      <span
        aria-hidden="true"
        className={`h-20 w-20 rounded-card flex items-center justify-center text-5xl shadow-sm hover:rotate-3 transition-transform duration-emphatic ${cue.surface}`}
      >
        {cue.emoji}
      </span>
      <span className="font-display text-base">{label}</span>
    </span>
  );
};

export default PictureCue;
