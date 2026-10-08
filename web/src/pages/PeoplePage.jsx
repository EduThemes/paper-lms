import React, { useMemo, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { Users, UserPlus, Search, X, Layers } from 'lucide-react';
import { useAuth } from '../contexts/AuthContext';
import useCourseStaffRole from '../hooks/useCourseStaffRole';
import { useRoster, useCourseSections, useEnrollmentAction } from '../hooks/useAdminEnrollments';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import Page from '../components/Page';
import AddPeopleForm from '../components/enrollments/AddPeopleForm';
import EditEnrollmentDialog from '../components/enrollments/EditEnrollmentDialog';
import EnrollmentActionsMenu from '../components/enrollments/EnrollmentActionsMenu';
import {
  ROLE_LABELS,
  ACTION_CONFIRM,
  STATUS_FILTERS,
  enrollmentActions,
  enrollmentStatus,
  sectionLabel,
} from '../utils/enrollmentLifecycle';

const ROLE_COLORS = {
  TeacherEnrollment: 'bg-brand-100 text-brand-800',
  TaEnrollment: 'bg-purple-100 text-purple-800',
  StudentEnrollment: 'bg-accent-success/20 text-accent-success',
  ObserverEnrollment: 'bg-surface-2 text-text-primary',
  DesignerEnrollment: 'bg-orange-100 text-orange-800',
};

const STATUS_COLORS = {
  active: 'bg-accent-success/20 text-accent-success',
  invited: 'bg-accent-warning/20 text-accent-warning',
  pending: 'bg-accent-warning/20 text-accent-warning',
  concluded: 'bg-surface-2 text-text-secondary',
  inactive: 'bg-surface-2 text-text-tertiary',
  rejected: 'bg-accent-danger/10 text-accent-danger',
};

const ROLE_PRIORITY = { TeacherEnrollment: 0, TaEnrollment: 1, DesignerEnrollment: 2, StudentEnrollment: 3, ObserverEnrollment: 4 };
const ADMIN_ROLES = new Set(['admin', 'super_admin']);

/**
 * Course People page: roster with role / status filters, Add People
 * (direct or by invitation) and, for staff, per-person lifecycle actions
 * — edit role, section and dates; conclude; deactivate; re-activate;
 * remove (G105, G107, G110, G122).
 */
const PeoplePage = () => {
  const { courseId } = useParams();
  const { user } = useAuth();
  const staffRole = useCourseStaffRole(courseId);
  const isAdmin = ADMIN_ROLES.has(user?.role);
  const caller = {
    userId: user?.id,
    isAdmin,
    isTeacher: isAdmin || Boolean(staffRole?.teacher),
    isStaff: isAdmin || Boolean(staffRole?.staff),
  };

  const [statusFilter, setStatusFilter] = useState('current');
  const states = caller.isStaff ? (STATUS_FILTERS.find((f) => f.key === statusFilter)?.states || []) : [];
  const roster = useRoster(courseId, states, { enabled: !!courseId && staffRole !== null });
  const sectionsQuery = useCourseSections(courseId);
  const sections = sectionsQuery.data || [];

  // The roster waits for the caller's role (it picks the state filter).
  const pageQuery = staffRole === null ? { ...roster, isLoading: true } : roster;

  return (
    <Page query={pageQuery} title="People">
      {(enrollments) => (
        <PeopleView
          courseId={courseId}
          caller={caller}
          enrollments={enrollments || []}
          sections={sections}
          statusFilter={statusFilter}
          setStatusFilter={setStatusFilter}
          isFetching={roster.isFetching}
        />
      )}
    </Page>
  );
};

function PeopleView({ courseId, caller, enrollments, sections, statusFilter, setStatusFilter, isFetching }) {
  const [search, setSearch] = useState('');
  const [roleFilter, setRoleFilter] = useState('all');
  const [showAddForm, setShowAddForm] = useState(false);
  const [editing, setEditing] = useState(null);
  const [error, setError] = useState(null);
  const [notice, setNotice] = useState(null);

  const sectionsById = useMemo(() => Object.fromEntries(sections.map((s) => [s.id, s])), [sections]);
  const enrolledUserIds = useMemo(() => new Set(enrollments.map((e) => e.user_id)), [enrollments]);

  const action = useEnrollmentAction(courseId, {
    onSuccess: (_data, vars) => {
      setError(null);
      setNotice(`${vars.label} done.`);
    },
    onError: (err) => setError(err.message),
  });

  const runAction = (enrollment, act) => {
    if (act === 'edit') {
      setEditing(enrollment);
      return;
    }
    // eslint-disable-next-line no-alert
    if (ACTION_CONFIRM[act] && !window.confirm(ACTION_CONFIRM[act])) return;
    const name = enrollment.user?.name || `User ${enrollment.user_id}`;
    const verb = { conclude: 'Concluding', deactivate: 'Deactivating', reactivate: 'Re-activating', delete: 'Removing' }[act];
    action.mutate({ enrollmentId: enrollment.id, action: act, label: `${verb} ${name}` });
  };

  const roleCounts = {};
  enrollments.forEach((e) => { roleCounts[e.type] = (roleCounts[e.type] || 0) + 1; });

  const term = search.trim().toLowerCase();
  const filtered = enrollments
    .filter((e) => roleFilter === 'all' || e.type === roleFilter)
    .filter((e) => !term || `${e.user?.name || ''} ${e.user?.login_id || ''}`.toLowerCase().includes(term))
    .sort((a, b) => (ROLE_PRIORITY[a.type] ?? 9) - (ROLE_PRIORITY[b.type] ?? 9));

  return (
    <Layout>
      <CourseNav />
      <div className="mb-6">
        <Link to={`/courses/${courseId}`} className="text-brand-600 hover:underline text-sm">&larr; Back to Course</Link>
        <div className="flex flex-wrap items-center justify-between gap-2 mt-2">
          <h2 className="text-2xl font-bold text-text-primary flex items-center gap-2">
            <Users className="w-6 h-6" />
            People
          </h2>
          {caller.isStaff && (
            <div className="flex items-center gap-2">
              {caller.isTeacher && (
                <Link
                  to={`/courses/${courseId}/sections`}
                  className="inline-flex items-center gap-2 border border-border-strong px-3 py-2 rounded-md text-sm font-medium text-text-primary hover:bg-surface-1"
                >
                  <Layers className="w-4 h-4" />
                  Sections
                </Link>
              )}
              <button
                type="button"
                onClick={() => setShowAddForm((v) => !v)}
                className="inline-flex items-center gap-2 bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm font-medium"
              >
                {showAddForm ? <X className="w-4 h-4" /> : <UserPlus className="w-4 h-4" />}
                {showAddForm ? 'Cancel' : 'Add People'}
              </button>
            </div>
          )}
        </div>
      </div>

      {error && (
        <div role="alert" className="bg-accent-danger/10 border border-accent-danger/30 text-accent-danger rounded-md p-3 mb-4 text-sm">
          {error}
          <button type="button" onClick={() => setError(null)} className="ms-2 font-bold" aria-label="Dismiss">&times;</button>
        </div>
      )}
      {notice && !error && (
        <div role="status" className="bg-accent-success/10 border border-accent-success/30 text-accent-success rounded-md p-3 mb-4 text-sm">
          {notice}
          <button type="button" onClick={() => setNotice(null)} className="ms-2 font-bold" aria-label="Dismiss">&times;</button>
        </div>
      )}

      {showAddForm && caller.isStaff && (
        <AddPeopleForm
          courseId={courseId}
          caller={caller}
          sections={sections}
          enrolledUserIds={enrolledUserIds}
          onDone={() => { setShowAddForm(false); setNotice('Person added.'); }}
        />
      )}

      <div className="flex flex-wrap items-center gap-3 mb-4">
        <div className="relative flex-1 min-w-[12rem] max-w-xs">
          <Search className="w-4 h-4 absolute start-3 top-1/2 -translate-y-1/2 text-text-disabled" />
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search people..."
            aria-label="Search people"
            className="w-full ps-9 pe-3 py-2 border border-border-strong rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
          />
        </div>
        <div className="flex flex-wrap items-center gap-2" role="group" aria-label="Filter by role">
          <button
            type="button"
            onClick={() => setRoleFilter('all')}
            className={`px-3 py-1.5 text-xs font-medium rounded-full ${roleFilter === 'all' ? 'bg-brand-100 text-brand-800' : 'bg-surface-2 text-text-secondary hover:bg-border-default'}`}
          >
            All roles ({enrollments.length})
          </button>
          {Object.entries(roleCounts).map(([role, count]) => (
            <button
              type="button"
              key={role}
              onClick={() => setRoleFilter(role)}
              className={`px-3 py-1.5 text-xs font-medium rounded-full ${roleFilter === role ? ROLE_COLORS[role] || 'bg-brand-100 text-brand-800' : 'bg-surface-2 text-text-secondary hover:bg-border-default'}`}
            >
              {ROLE_LABELS[role] || role} ({count})
            </button>
          ))}
        </div>
        {caller.isStaff && (
          <div className="flex items-center gap-2 ms-auto">
            <label htmlFor="people-status" className="text-xs text-text-tertiary">Status</label>
            <select
              id="people-status"
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value)}
              className="border border-border-strong rounded-md px-2 py-1.5 text-sm"
            >
              {STATUS_FILTERS.map((f) => <option key={f.key} value={f.key}>{f.label}</option>)}
            </select>
          </div>
        )}
      </div>

      <div className="bg-surface-0 rounded-lg shadow overflow-hidden" aria-busy={isFetching || action.isPending}>
        {filtered.length === 0 ? (
          <div className="p-6 text-center text-text-tertiary">
            {search ? 'No people match your search.' : 'No people here.'}
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead className="bg-surface-1 border-b">
                <tr>
                  <th className="text-start px-4 py-3 text-xs font-medium text-text-tertiary uppercase">Name</th>
                  <th className="text-start px-4 py-3 text-xs font-medium text-text-tertiary uppercase">Login ID</th>
                  <th className="text-start px-4 py-3 text-xs font-medium text-text-tertiary uppercase">Role</th>
                  <th className="text-start px-4 py-3 text-xs font-medium text-text-tertiary uppercase">Section</th>
                  <th className="text-start px-4 py-3 text-xs font-medium text-text-tertiary uppercase">Status</th>
                  {caller.isStaff && <th className="px-4 py-3"><span className="sr-only">Actions</span></th>}
                </tr>
              </thead>
              <tbody className="divide-y">
                {filtered.map((enrollment) => {
                  const status = enrollmentStatus(enrollment);
                  const name = enrollment.user?.name || `User ${enrollment.user_id}`;
                  return (
                    <tr key={enrollment.id} className="hover:bg-surface-1">
                      <td className="px-4 py-3">
                        <div className="flex items-center gap-3">
                          <div className="w-8 h-8 bg-border-default rounded-full flex items-center justify-center text-text-tertiary text-sm font-medium flex-shrink-0">
                            {(enrollment.user?.name || '?')[0].toUpperCase()}
                          </div>
                          <div>
                            <div className="text-sm font-medium text-text-primary">{name}</div>
                            {enrollment.user?.email && <div className="text-xs text-text-tertiary">{enrollment.user.email}</div>}
                          </div>
                        </div>
                      </td>
                      <td className="px-4 py-3 text-sm text-text-secondary">{enrollment.user?.login_id || '-'}</td>
                      <td className="px-4 py-3">
                        <span className={`text-xs font-medium px-2 py-0.5 rounded-full ${ROLE_COLORS[enrollment.type] || 'bg-surface-2 text-text-primary'}`}>
                          {ROLE_LABELS[enrollment.type] || enrollment.type}
                        </span>
                      </td>
                      <td className="px-4 py-3 text-sm text-text-secondary">
                        {sectionLabel(sectionsById, enrollment.course_section_id)}
                        {enrollment.limit_privileges_to_course_section && (
                          <span className="block text-xs text-text-tertiary">section only</span>
                        )}
                      </td>
                      <td className="px-4 py-3">
                        <span className={`text-xs px-2 py-0.5 rounded-full ${STATUS_COLORS[status.key] || 'bg-surface-2 text-text-secondary'}`}>
                          {status.label}
                        </span>
                      </td>
                      {caller.isStaff && (
                        <td className="px-4 py-3 text-end">
                          <EnrollmentActionsMenu
                            actions={enrollmentActions(enrollment, caller)}
                            personName={name}
                            disabled={action.isPending}
                            onAction={(act) => runAction(enrollment, act)}
                          />
                        </td>
                      )}
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {editing && (
        <EditEnrollmentDialog
          courseId={courseId}
          enrollment={editing}
          sections={sections}
          caller={caller}
          onClose={() => setEditing(null)}
        />
      )}
    </Layout>
  );
}

export default PeoplePage;
