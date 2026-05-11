// Default grading scale matching the backend's defaultGradingScale.
// Each entry is [name, minPercentageAsDecimal].
// Must be sorted descending by percentage.
const DEFAULT_SCALE = [
  ['A', 0.93],
  ['A-', 0.90],
  ['B+', 0.87],
  ['B', 0.83],
  ['B-', 0.80],
  ['C+', 0.77],
  ['C', 0.73],
  ['C-', 0.70],
  ['D+', 0.67],
  ['D', 0.63],
  ['D-', 0.60],
  ['F', 0.0],
];

/**
 * Convert a percentage (0–100) to a letter grade.
 * @param {number|null} percentage - The percentage score (0–100)
 * @param {Array} scale - Optional custom scale as [[name, minDecimal], ...], sorted descending
 * @returns {string} The letter grade, or '-' if percentage is null/undefined
 */
export const getLetterGrade = (percentage, scale = null) => {
  if (percentage === null || percentage === undefined) return '-';
  const pct = parseFloat(percentage) / 100;
  const entries = scale || DEFAULT_SCALE;
  for (const entry of entries) {
    // Support both formats: [name, minDecimal] arrays and {name, value} objects
    const name = Array.isArray(entry) ? entry[0] : entry.name;
    let threshold = Array.isArray(entry) ? entry[1] : entry.value;
    // API returns whole-number percentages (e.g. 93), normalize to decimal (0.93)
    if (threshold > 1) threshold = threshold / 100;
    if (pct >= threshold) return name;
  }
  if (entries.length === 0) return 'F';
  const last = entries[entries.length - 1];
  return Array.isArray(last) ? last[0] : last.name;
};

/**
 * Get a Tailwind text color class for a letter grade.
 * @param {string} letter - The letter grade
 * @returns {string} Tailwind CSS class
 */
export const gradeColor = (letter) => {
  if (letter === '-') return 'text-gray-400';
  if (letter.startsWith('A')) return 'text-green-600';
  if (letter.startsWith('B')) return 'text-blue-600';
  if (letter.startsWith('C')) return 'text-yellow-600';
  if (letter.startsWith('D')) return 'text-orange-600';
  return 'text-red-600';
};

export { DEFAULT_SCALE };
