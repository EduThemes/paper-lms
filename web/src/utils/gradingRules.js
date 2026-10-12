// Assignment group drop rules (G020) helpers.

// parseRules reads an assignment group's rules (object, JSON string or
// Canvas "drop_lowest:1\n" text) into { drop_lowest, drop_highest,
// never_drop }.
export function parseRules(rules) {
  const out = { drop_lowest: 0, drop_highest: 0, never_drop: [] };
  if (!rules) return out;
  let r = rules;
  if (typeof r === 'string') {
    try {
      r = JSON.parse(r);
    } catch {
      const lines = r.split(/\n/);
      r = {};
      for (const line of lines) {
        const [k, v] = line.split(':');
        if (!k) continue;
        if (k === 'never_drop') r.never_drop = [...(r.never_drop || []), Number(v)];
        else r[k] = Number(v);
      }
    }
  }
  out.drop_lowest = Number(r.drop_lowest) || 0;
  out.drop_highest = Number(r.drop_highest) || 0;
  out.never_drop = Array.isArray(r.never_drop) ? r.never_drop.map(Number).filter(Boolean) : [];
  return out;
}

// describeRules is a one-line summary ("Drops lowest 1, highest 1").
export function describeRules(rules) {
  const r = parseRules(rules);
  const parts = [];
  if (r.drop_lowest > 0) parts.push(`lowest ${r.drop_lowest}`);
  if (r.drop_highest > 0) parts.push(`highest ${r.drop_highest}`);
  if (parts.length === 0) return '';
  return `Drops ${parts.join(', ')}`;
}
