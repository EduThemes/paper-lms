// Course file-tree helpers (G070 / G071 / G072).

// buildFolderTree indexes the flat folder list from
// GET /courses/:id/folders. The root ("course files") is the folder with
// no parent; the Files page shows its children as the top level.
export function buildFolderTree(folders = []) {
  const byId = new Map();
  const children = new Map();
  let root = null;
  for (const f of folders) {
    byId.set(f.id, f);
    if (f.parent_folder_id == null) {
      if (!root) root = f;
      continue;
    }
    if (!children.has(f.parent_folder_id)) children.set(f.parent_folder_id, []);
    children.get(f.parent_folder_id).push(f);
  }
  for (const list of children.values()) {
    list.sort((a, b) => (a.position ?? 0) - (b.position ?? 0) || String(a.name).localeCompare(String(b.name)));
  }
  return {
    root,
    byId,
    // childrenOf(null) = the top level (the root folder's children).
    childrenOf: (id) => children.get(id == null ? root?.id : id) || [],
  };
}

// folderPath returns the folders from the top level down to id (the root
// folder itself is left out), for breadcrumbs.
export function folderPath(tree, id) {
  const path = [];
  const seen = new Set();
  let cur = id == null ? null : tree.byId.get(id);
  while (cur && cur.parent_folder_id != null && !seen.has(cur.id)) {
    seen.add(cur.id);
    path.unshift(cur);
    cur = tree.byId.get(cur.parent_folder_id);
  }
  return path;
}

// moveTargets lists the folders something can be moved into, depth-first
// with indentation, starting with the top level (value null). Pass
// excludeId to drop a folder and everything below it (a folder can't
// move into itself).
export function moveTargets(tree, excludeId = null) {
  const out = [{ value: null, label: 'Files (top level)', depth: 0 }];
  const walk = (parentId, depth) => {
    for (const f of tree.childrenOf(parentId)) {
      if (f.id === excludeId) continue;
      out.push({ value: f.id, label: f.name, depth });
      walk(f.id, depth + 1);
    }
  };
  walk(null, 1);
  return out;
}

export function formatBytes(bytes) {
  const n = Number(bytes) || 0;
  if (n < 1024) return `${n} B`;
  const units = ['KB', 'MB', 'GB', 'TB'];
  let value = n / 1024;
  let i = 0;
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024;
    i += 1;
  }
  return `${value >= 10 ? Math.round(value) : value.toFixed(1)} ${units[i]}`;
}

// quotaSummary turns {quota, quota_used} into display numbers.
export function quotaSummary(q) {
  if (!q || !q.quota) return null;
  const pct = Math.min(100, Math.round(((q.quota_used || 0) / q.quota) * 100));
  return { pct, used: formatBytes(q.quota_used || 0), total: formatBytes(q.quota), nearlyFull: pct >= 90 };
}

const fmtDate = (iso) => new Date(iso).toLocaleString(undefined, { month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' });

// fileStatus describes a file's visibility for the list (staff view) or
// its availability (learner view).
export function fileStatus(file, now = new Date()) {
  const unlock = file.unlock_at ? new Date(file.unlock_at) : null;
  const lock = file.lock_at ? new Date(file.lock_at) : null;
  if (file.locked) return { kind: 'unpublished', label: 'Unpublished' };
  if (unlock && now < unlock) return { kind: 'scheduled', label: `Available ${fmtDate(file.unlock_at)}` };
  if (lock && now >= lock) return { kind: 'closed', label: `Locked since ${fmtDate(file.lock_at)}` };
  if (file.hidden) return { kind: 'hidden', label: 'Hidden (link only)' };
  if (lock) return { kind: 'published', label: `Until ${fmtDate(file.lock_at)}` };
  return { kind: 'published', label: 'Published' };
}

export const USAGE_RIGHTS_OPTIONS = [
  { value: '', label: 'Not set' },
  { value: 'own_copyright', label: 'I hold the copyright' },
  { value: 'used_by_permission', label: 'I have permission to use it' },
  { value: 'public_domain', label: 'It is in the public domain' },
  { value: 'fair_use', label: 'Fair use or similar exception' },
  { value: 'creative_commons', label: 'Creative Commons license' },
];

export const CC_LICENSES = [
  { value: 'cc_by', label: 'CC BY' },
  { value: 'cc_by_sa', label: 'CC BY-SA' },
  { value: 'cc_by_nd', label: 'CC BY-ND' },
  { value: 'cc_by_nc', label: 'CC BY-NC' },
  { value: 'cc_by_nc_sa', label: 'CC BY-NC-SA' },
  { value: 'cc_by_nc_nd', label: 'CC BY-NC-ND' },
];
