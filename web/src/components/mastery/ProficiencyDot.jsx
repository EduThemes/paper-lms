import React, { useState } from 'react';

/**
 * ProficiencyDot renders a small colored circle representing a student's
 * proficiency level for a single learning outcome. Hovering reveals a tooltip
 * showing the rating description and the score.
 *
 * Props:
 *   rating  - { description, points, color, mastery } | null
 *   score   - number | null
 *   possible - number | null
 *   size    - number (px diameter, default 20)
 */
const ProficiencyDot = ({ rating, score, possible, size = 20 }) => {
  const [hovered, setHovered] = useState(false);

  // Empty cell — student has no result for this outcome.
  if (!rating) {
    return (
      <div
        className="inline-flex items-center justify-center"
        title="No data"
        style={{ width: size, height: size }}
      >
        <div
          className="rounded-full border border-dashed border-border-strong"
          style={{ width: size * 0.7, height: size * 0.7 }}
          aria-label="No proficiency data"
        />
      </div>
    );
  }

  const ringClass = rating.mastery ? 'ring-2 ring-offset-1 ring-blue-400' : '';

  const tooltipLines = [rating.description];
  if (score != null) {
    tooltipLines.push(possible != null ? `Score: ${score} / ${possible}` : `Score: ${score}`);
  }
  tooltipLines.push(`Threshold: ${rating.points}`);

  return (
    <div
      className="relative inline-flex items-center justify-center"
      style={{ width: size, height: size }}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      onFocus={() => setHovered(true)}
      onBlur={() => setHovered(false)}
      tabIndex={0}
      role="img"
      aria-label={`${rating.description}${score != null ? `, score ${score}` : ''}`}
    >
      <div
        className={`rounded-full ${ringClass}`}
        style={{
          width: size,
          height: size,
          backgroundColor: rating.color || 'rgb(var(--color-text-tertiary))',
        }}
      />
      {hovered && (
        <div
          className="absolute z-50 left-1/2 -translate-x-1/2 mt-1 top-full w-44 rounded-md bg-gray-900 px-2 py-1.5 text-xs text-white shadow-lg pointer-events-none"
          role="tooltip"
        >
          <div className="font-semibold">{rating.description}</div>
          {score != null && (
            <div className="text-gray-200">
              {possible != null ? `Score: ${score} / ${possible}` : `Score: ${score}`}
            </div>
          )}
          <div className="text-gray-300">Threshold: {rating.points}</div>
          {rating.mastery && <div className="text-blue-300">Mastery line</div>}
        </div>
      )}
    </div>
  );
};

export default ProficiencyDot;
