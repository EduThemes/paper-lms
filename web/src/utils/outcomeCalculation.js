// Outcome calculation methods (G150/G151), matching the server's
// service.OutcomeCalculationMethods and Canvas semantics.
//
// "decaying_average" is Paper's long-standing default and keeps Canvas's
// legacy meaning (Canvas now calls it "Weighted Average"): the most recent
// score counts calculation_int %, the average of the earlier ones the rest.
// "standard_decaying_average" is the true decaying average Canvas labels
// "Decaying Average".

export const CALCULATION_METHODS = [
  {
    value: 'decaying_average',
    label: 'Weighted Average',
    description: 'The most recent score counts for the chosen %, the average of earlier scores for the rest.',
    intLabel: 'Most recent weight (%)',
    min: 1,
    max: 99,
    defaultInt: 65,
  },
  {
    value: 'standard_decaying_average',
    label: 'Decaying Average',
    description: 'Each newer score counts for the chosen %, so older scores fade out over time.',
    intLabel: 'Decay rate (%)',
    min: 1,
    max: 99,
    defaultInt: 65,
  },
  {
    value: 'n_mastery',
    label: 'n Number of Times',
    description: 'Mastery must be shown this many times; the score is the average of the mastering scores.',
    intLabel: 'Times mastery is required',
    min: 1,
    max: 10,
    defaultInt: 5,
  },
  { value: 'latest', label: 'Most Recent Score', description: 'Uses the most recent score.' },
  { value: 'highest', label: 'Highest Score', description: 'Uses the highest score achieved.' },
  { value: 'average', label: 'Average', description: 'Simple average of all scores.' },
];

const ALIASES = { weighted_average: 'decaying_average' };

export function methodInfo(method) {
  const key = ALIASES[method] || method;
  return CALCULATION_METHODS.find((m) => m.value === key) || null;
}

export function methodLabel(method) {
  return methodInfo(method)?.label || method || 'Weighted Average';
}

export function methodTakesInt(method) {
  return methodInfo(method)?.defaultInt != null;
}

/** The calculation_int to keep when switching to `method`. */
export function intForMethod(method, current) {
  const info = methodInfo(method);
  if (!info || info.defaultInt == null) return null;
  const n = Number(current);
  if (Number.isInteger(n) && n >= info.min && n <= info.max) return n;
  return info.defaultInt;
}

/** A one-line, human description of an outcome's calculation. */
export function describeCalculation(method, calcInt) {
  const info = methodInfo(method);
  if (!info) return '';
  const key = ALIASES[method] || method;
  switch (key) {
    case 'decaying_average':
      return `Most recent score ${calcInt ?? 65}%, earlier scores ${100 - (calcInt ?? 65)}%`;
    case 'standard_decaying_average':
      return `Each newer score weighted ${calcInt ?? 65}%`;
    case 'n_mastery':
      return `Mastery required ${calcInt ?? 5} time${(calcInt ?? 5) === 1 ? '' : 's'}`;
    default:
      return info.description;
  }
}

/** Validates an outcome form; returns an error message or null. */
export function validateOutcomeForm({ title, masteryPoints, method, calcInt, ratings }) {
  if (!title || !String(title).trim()) return 'Title is required.';
  const info = methodInfo(method);
  if (!info) return 'Choose a calculation method.';
  if (info.defaultInt != null) {
    const n = Number(calcInt);
    if (!Number.isInteger(n) || n < info.min || n > info.max) {
      return `${info.intLabel} must be between ${info.min} and ${info.max}.`;
    }
  }
  const mp = Number(masteryPoints);
  if (!Number.isFinite(mp) || mp < 0) return 'Mastery points must be zero or more.';
  if (Array.isArray(ratings)) {
    if (ratings.length < 2) return 'Add at least two ratings.';
    if (ratings.some((r) => !String(r.description || '').trim())) return 'Every rating needs a description.';
    const top = Math.max(...ratings.map((r) => Number(r.points) || 0));
    if (mp > top) return 'Mastery points cannot be above the highest rating.';
  }
  return null;
}
