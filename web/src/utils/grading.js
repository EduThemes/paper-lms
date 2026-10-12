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

/**
 * Whether students can see an assignment (and it counts toward grades).
 * workflow_state is authoritative; `published` covers older payloads.
 */
export const isAssignmentPublished = (assignment) => {
  if (!assignment) return false;
  if (assignment.workflow_state === 'deleted') return false;
  if (assignment.workflow_state) return assignment.workflow_state === 'published';
  return !!assignment.published;
};

/**
 * Canvas grade-calculator rules for one assignment/submission pair.
 * Returns { counts, possible }: unpublished, deleted and not_graded
 * assignments never count, an excused submission is dropped from both
 * earned and possible, and a 0-point assignment's score is extra credit
 * (earned only). Mirrors service.assignmentCounts on the server.
 */
export const assignmentGradeWeight = (assignment, cell) => {
  if (!isAssignmentPublished(assignment) || assignment.grading_type === 'not_graded') {
    return { counts: false, possible: 0 };
  }
  if (cell?.excused) return { counts: false, possible: 0 };
  const possible = parseFloat(assignment.points_possible) || 0;
  if (possible > 0) return { counts: true, possible };
  return { counts: cell?.score != null, possible: 0 };
};

/**
 * The grade a teacher typed (before any late-policy deduction). Editing
 * controls show this value so re-saving never re-applies the penalty.
 */
export const enteredScore = (cell) => {
  if (!cell) return null;
  if (cell.entered_score !== null && cell.entered_score !== undefined) return cell.entered_score;
  return cell.score ?? null;
};

/**
 * Curve an assignment so the class average of ENTERED scores reaches
 * targetPercent of points possible. Every score shifts by the same
 * offset and is clamped to 0…pointsPossible.
 * @returns {{ offset: number, grades: Array<{ user_id, posted_grade }> }}
 */
export const curveGrades = ({ entries, pointsPossible, targetPercent }) => {
  const scored = entries.filter(
    (e) => e.entered !== null && e.entered !== undefined && !isNaN(parseFloat(e.entered))
  );
  const max = parseFloat(pointsPossible) || 0;
  const target = parseFloat(targetPercent);
  if (scored.length === 0 || isNaN(target)) return { offset: 0, grades: [] };
  const avg = scored.reduce((sum, e) => sum + parseFloat(e.entered), 0) / scored.length;
  const targetPoints = max > 0 ? (max * target) / 100 : target;
  const offset = targetPoints - avg;
  const grades = scored.map((e) => {
    let curved = parseFloat(e.entered) + offset;
    curved = max > 0 ? Math.max(0, Math.min(max, curved)) : Math.max(0, curved);
    curved = Math.round(curved * 100) / 100;
    return { user_id: e.userId, posted_grade: String(curved) };
  });
  return { offset, grades };
};

/**
 * RFC 4180 CSV parser: quoted fields may contain commas, escaped quotes
 * ("") and line breaks. Returns an array of rows (arrays of strings),
 * with blank lines dropped and cell whitespace trimmed.
 */
export const parseCSV = (text) => {
  const rows = [];
  let row = [];
  let field = '';
  let inQuotes = false;
  const src = String(text ?? '').replace(/^﻿/, '');
  for (let i = 0; i < src.length; i++) {
    const ch = src[i];
    if (inQuotes) {
      if (ch === '"') {
        if (src[i + 1] === '"') {
          field += '"';
          i++;
        } else {
          inQuotes = false;
        }
      } else {
        field += ch;
      }
      continue;
    }
    if (ch === '"') {
      inQuotes = true;
    } else if (ch === ',') {
      row.push(field);
      field = '';
    } else if (ch === '\n' || ch === '\r') {
      if (ch === '\r' && src[i + 1] === '\n') i++;
      row.push(field);
      rows.push(row);
      row = [];
      field = '';
    } else {
      field += ch;
    }
  }
  if (field !== '' || row.length > 0) {
    row.push(field);
    rows.push(row);
  }
  return rows.map((r) => r.map((c) => c.trim())).filter((r) => r.some((c) => c !== ''));
};

/**
 * Convert each rubric criterion's points to a number (null when blank)
 * before saving an assessment: a typed criterion box yields a string
 * ("4"), which the server could not total (B120).
 */
export const normalizeRubricScores = (scores) => {
  const out = {};
  for (const [id, c] of Object.entries(scores || {})) {
    const raw = c?.points;
    const num = typeof raw === 'number' ? raw : parseFloat(raw);
    out[id] = { ...c, points: Number.isFinite(num) ? num : null };
  }
  return out;
};
