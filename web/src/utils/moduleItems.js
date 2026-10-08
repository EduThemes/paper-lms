// Module item helpers shared by ModulesPage: item links (B146) and the
// "add item" content picker (B042).

const API_BASE = import.meta.env.VITE_API_URL || '/api/v1';

// Item types that link to existing course content and therefore need a
// content_id (Canvas requires one; an item without it is a dead row).
export const CONTENT_ITEM_TYPES = ['Assignment', 'Quiz', 'Discussion', 'Page', 'File', 'ExternalTool'];

// Item types that only link existing content: there is no "[ New … ]"
// option (G064 — files are uploaded on the Files page, tools installed
// on the External Tools page).
export const LINK_ONLY_TYPES = ['File', 'ExternalTool'];

export function canCreateContent(type) {
  return itemNeedsContent(type) && !LINK_ONLY_TYPES.includes(type);
}

export const NEW_CONTENT = 'new';

export function itemNeedsContent(type) {
  return CONTENT_ITEM_TYPES.includes(type);
}

// Item types whose link is a plain <a> (leaves the SPA) rather than a
// router <Link>.
export function isExternalItemLink(item) {
  return item?.type === 'ExternalUrl' || item?.type === 'File';
}

// The URL a module item opens, or null when it has nothing to open.
export function getModuleItemLink(item, courseId) {
  if (!item) return null;
  const id = item.content_id;
  switch (item.type) {
    case 'Assignment':
      return id ? `/courses/${courseId}/assignments/${id}` : null;
    case 'Quiz':
      return id ? `/courses/${courseId}/quizzes/${id}/take` : null;
    case 'Page':
      if (item.page_url) return `/courses/${courseId}/pages/${item.page_url}`;
      return id ? `/courses/${courseId}/pages/${id}` : null;
    case 'Discussion':
      return id ? `/courses/${courseId}/discussions/${id}` : null;
    case 'ExternalUrl':
      return item.url || null;
    case 'File':
    case 'Attachment':
      // Imported files: the download endpoint enforces enrollment.
      return id ? `${API_BASE}/files/${id}/download` : null;
    case 'Announcement':
      return `/courses/${courseId}/announcements`;
    case 'ExternalTool':
      // Launched through an LTI form post (launchModuleTool), not a link.
      return null;
    default:
      return null;
  }
}

// Loads the existing content of a type as picker options [{id, label}].
export async function fetchContentOptions(api, courseId, type) {
  switch (type) {
    case 'Assignment': {
      const res = await api.getAssignments(courseId, 1, 100);
      return (res?.data || []).map((a) => ({ id: a.id, label: a.name }));
    }
    case 'Quiz': {
      const res = await api.getQuizzes(courseId, 1, 100);
      return (res?.data || []).map((q) => ({ id: q.id, label: q.title }));
    }
    case 'Discussion': {
      const res = await api.getDiscussionTopics(courseId, 1, 100);
      return (res?.data || []).map((d) => ({ id: d.id, label: d.title }));
    }
    case 'Page': {
      const res = await api.getPages(courseId, 1, 100);
      return (res?.data || []).map((p) => ({ id: p.page_id ?? p.id, label: p.title }));
    }
    case 'File': {
      const res = await api.getCourseFiles(courseId, 1, 100);
      return (res?.data || []).map((f) => ({ id: f.id, label: f.display_name || f.filename }));
    }
    case 'ExternalTool': {
      const res = await api.getExternalTools(courseId, 1, 100);
      return (res?.data || [])
        .filter((t) => !t.workflow_state || t.workflow_state === 'active')
        .map((t) => ({ id: t.id, label: t.name }));
    }
    default:
      return [];
  }
}

// Creates new (unpublished) content for an item and returns its id.
// Pages are left to the backend, which creates one when an item has no
// content_id.
export async function createContentForItem(api, courseId, type, title) {
  switch (type) {
    case 'Assignment': {
      const a = await api.createAssignment(courseId, { name: title, published: false });
      return a?.id;
    }
    case 'Quiz': {
      const q = await api.createQuiz(courseId, { title, published: false });
      return q?.id;
    }
    case 'Discussion': {
      const d = await api.createDiscussionTopic(courseId, { title, message: '' });
      return d?.id;
    }
    default:
      return undefined;
  }
}

// Builds the module_item payload for POST .../items. `selection` is the
// picker value: an existing content id, NEW_CONTENT, or '' (none).
export async function buildModuleItemPayload(api, courseId, { title, type, selection, externalUrl, newTab, position }) {
  const payload = { title: (title || '').trim(), type, position };
  if (type === 'ExternalUrl' || type === 'ExternalTool') {
    if (externalUrl) payload.external_url = externalUrl;
    payload.new_tab = newTab;
  }
  if (LINK_ONLY_TYPES.includes(type)) {
    if (!selection || selection === NEW_CONTENT) {
      throw new Error(type === 'File' ? 'Choose a file' : 'Choose an external tool');
    }
    payload.content_id = Number(selection);
    return payload;
  }
  if (itemNeedsContent(type)) {
    if (selection && selection !== NEW_CONTENT) {
      payload.content_id = Number(selection);
    } else if (type !== 'Page') {
      if (!payload.title) throw new Error(`A title is required for a new ${type.toLowerCase()}`);
      const id = await createContentForItem(api, courseId, type, payload.title);
      if (!id) throw new Error(`Could not create the ${type.toLowerCase()}`);
      payload.content_id = id;
    }
  }
  return payload;
}
