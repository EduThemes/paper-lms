// Gradebook CSV import mapping. Accepts Paper's own export
// ("Student, Student ID, <assignment name>, ...") and Canvas's export
// ("Student, ID, SIS User ID, SIS Login ID, Section,
// <assignment name> (<id>), ...", with a "Points Possible" row under
// the header).

const NON_ASSIGNMENT = /^(student|student\s*id|id|sis\s*user\s*id|sis\s*login\s*id|root\s*account|section|integration\s*id|total.*|percentage|letter\s*grade|(current|final|unposted)\s*(score|grade|points).*|.*\b(current|final|unposted)\s*(score|grade|points)|override\s*(score|grade))$/i;

const ID_SUFFIX = /\((\d+)\)\s*$/;

const norm = (s) => String(s ?? '').trim().toLowerCase();

// columnAssignment returns the assignment a header names, or null.
export const columnAssignment = (header, assignments) => {
  const h = String(header ?? '').trim();
  if (!h || NON_ASSIGNMENT.test(h)) return null;
  const m = ID_SUFFIX.exec(h);
  if (m) {
    const byId = assignments.find((a) => String(a.id) === m[1]);
    if (byId) return byId;
  }
  const name = norm(h.replace(ID_SUFFIX, ''));
  return assignments.find((a) => norm(a.name) === name) || assignments.find((a) => norm(a.name) === norm(h)) || null;
};

// isMetaRow spots Canvas's "Points Possible" / "Manual Posting" rows.
const isMetaRow = (cells) => /^(points\s*possible|manual\s*posting)/i.test(String(cells?.[0] ?? '').trim());

/**
 * mapGradebookCSV turns parsed CSV rows into grade entries.
 * @param {string[][]} rows parsed CSV (header first)
 * @param {Array<{id, name}>} assignments
 * @returns {{ entries: Array<{user_id, assignment_id, score}>, skipped: number, unmatched: string[] }}
 */
export const mapGradebookCSV = (rows, assignments) => {
  if (!Array.isArray(rows) || rows.length < 2) throw new Error('CSV must have a header row and at least one data row');
  const headers = rows[0].map((h) => String(h ?? '').trim());
  let idIdx = headers.findIndex((h) => /^student\s*id$/i.test(h));
  if (idIdx === -1) idIdx = headers.findIndex((h) => /^id$/i.test(h));
  if (idIdx === -1) throw new Error('CSV must include a "Student ID" (or Canvas "ID") column');

  const columns = [];
  const unmatched = [];
  headers.forEach((h, col) => {
    if (col === idIdx || col === 0) return;
    const a = columnAssignment(h, assignments || []);
    if (a) columns.push({ col, assignment: a });
    else if (h && !NON_ASSIGNMENT.test(h)) unmatched.push(h);
  });

  const entries = [];
  let skipped = 0;
  for (let i = 1; i < rows.length; i++) {
    const cells = rows[i] || [];
    if (cells.every((c) => String(c ?? '').trim() === '')) continue;
    if (isMetaRow(cells)) continue;
    const userId = parseInt(String(cells[idIdx] ?? '').trim(), 10);
    if (!userId) {
      skipped++;
      continue;
    }
    for (const { col, assignment } of columns) {
      const raw = String(cells[col] ?? '').trim();
      if (raw === '') continue;
      const score = parseFloat(raw);
      if (Number.isNaN(score)) continue;
      entries.push({ user_id: userId, assignment_id: assignment.id, score });
    }
  }
  return { entries, skipped, unmatched };
};
