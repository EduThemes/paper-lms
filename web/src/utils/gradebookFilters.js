// Gradebook filters (Canvas gradebook "View > Filters"): by section,
// grading period and module.

// assignmentInPeriod: Canvas places an assignment in the grading period
// that contains its due date; an assignment without a due date belongs
// to the last period.
export const assignmentInPeriod = (assignment, period, periods = []) => {
  if (!period) return true;
  const start = new Date(period.start_date).getTime();
  const end = new Date(period.end_date).getTime();
  if (!assignment?.due_at) {
    const last = [...periods].sort((a, b) => new Date(b.end_date) - new Date(a.end_date))[0];
    return !!last && String(last.id) === String(period.id);
  }
  const due = new Date(assignment.due_at).getTime();
  return due > start && due <= end;
};

// moduleAssignmentIds maps module id -> Set of assignment ids from
// modules loaded with their items.
export const moduleAssignmentIds = (modules) => {
  const out = {};
  for (const m of Array.isArray(modules) ? modules : []) {
    const ids = new Set();
    for (const item of m.items || m.module_items || []) {
      const type = item.type || item.content_type;
      if (type === 'Assignment' && item.content_id != null) ids.add(String(item.content_id));
    }
    out[String(m.id)] = ids;
  }
  return out;
};

export const filterAssignments = (assignments, { gradingPeriodId, moduleId } = {}, { periods = [], modules = {} } = {}) => {
  let list = Array.isArray(assignments) ? assignments : [];
  if (gradingPeriodId) {
    const period = periods.find((p) => String(p.id) === String(gradingPeriodId));
    if (period) list = list.filter((a) => assignmentInPeriod(a, period, periods));
  }
  if (moduleId) {
    const ids = modules[String(moduleId)];
    if (ids) list = list.filter((a) => ids.has(String(a.id)));
  }
  return list;
};

export const filterStudentsBySection = (students, sectionId) => {
  if (!sectionId) return students;
  return (students || []).filter((s) => (s.section_ids || []).some((id) => String(id) === String(sectionId)));
};
