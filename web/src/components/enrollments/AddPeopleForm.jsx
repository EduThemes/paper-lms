import React, { useEffect, useState } from 'react';
import { Search, X } from 'lucide-react';
import { api } from '../../services/api';
import { useCreateEnrollment } from '../../hooks/useAdminEnrollments';
import { ROLE_LABELS, assignableRoles } from '../../utils/enrollmentLifecycle';

const fieldClass = 'border border-border-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500';

/**
 * Course "Add People" form. Looks people up by exact email, login ID or
 * SIS ID (GET /courses/:id/enrollable_users), then enrolls them directly
 * or sends an invitation they must accept (G122), optionally limited to
 * the chosen section (G107). A person may be added again in another role
 * or section (G110).
 */
export default function AddPeopleForm({ courseId, caller, sections = [], enrolledUserIds, onDone }) {
  const roles = assignableRoles(caller);
  const [userSearch, setUserSearch] = useState('');
  const [userResults, setUserResults] = useState([]);
  const [searching, setSearching] = useState(false);
  const [showDropdown, setShowDropdown] = useState(false);
  const [selectedUser, setSelectedUser] = useState(null);
  const [type, setType] = useState(roles[0] || 'StudentEnrollment');
  const [sectionId, setSectionId] = useState('');
  const [invite, setInvite] = useState(false);
  const [limit, setLimit] = useState(false);
  const [error, setError] = useState(null);

  const create = useCreateEnrollment(courseId, {
    onSuccess: () => {
      setSelectedUser(null);
      setUserSearch('');
      setInvite(false);
      setLimit(false);
      onDone?.();
    },
    onError: (err) => setError(err.message),
  });

  useEffect(() => {
    const term = userSearch.trim();
    if (term.length < 3) {
      setUserResults([]);
      setShowDropdown(false);
      return undefined;
    }
    const timer = setTimeout(async () => {
      setSearching(true);
      try {
        const users = await api.searchEnrollableUsers(courseId, term);
        setUserResults(users || []);
        setShowDropdown(true);
      } catch {
        setUserResults([]);
      } finally {
        setSearching(false);
      }
    }, 300);
    return () => clearTimeout(timer);
  }, [userSearch, courseId]);

  const submit = (e) => {
    e.preventDefault();
    if (!selectedUser) return;
    setError(null);
    create.mutate({
      user_id: selectedUser.id,
      type,
      enrollment_state: invite ? 'invited' : 'active',
      ...(sectionId ? { course_section_id: Number(sectionId) } : {}),
      ...(limit ? { limit_privileges_to_course_section: true } : {}),
    });
  };

  return (
    <div className="bg-surface-0 rounded-lg shadow p-4 mb-4">
      <form onSubmit={submit} className="flex flex-wrap items-end gap-3" aria-label="Add people">
        <div className="flex-1 min-w-[16rem] relative">
          <label htmlFor="add-person-search" className="block text-sm font-medium text-text-secondary mb-1">Person</label>
          {selectedUser ? (
            <div className="flex items-center gap-2 border border-border-strong rounded-md px-3 py-2 text-sm bg-surface-1">
              <span className="text-text-primary font-medium truncate">{selectedUser.name}</span>
              {selectedUser.email && <span className="text-text-disabled text-xs truncate">{selectedUser.email}</span>}
              {enrolledUserIds?.has(selectedUser.id) && (
                <span className="text-xs text-accent-warning">already in this course — adds another role</span>
              )}
              <button type="button" aria-label="Clear person" onClick={() => setSelectedUser(null)} className="ml-auto text-text-disabled hover:text-text-secondary">
                <X className="w-4 h-4" />
              </button>
            </div>
          ) : (
            <>
              <div className="relative">
                <Search className="w-4 h-4 absolute start-3 top-1/2 -translate-y-1/2 text-text-disabled" />
                <input
                  id="add-person-search"
                  type="text"
                  value={userSearch}
                  onChange={(e) => setUserSearch(e.target.value)}
                  onBlur={() => setTimeout(() => setShowDropdown(false), 200)}
                  placeholder="Email, login ID or SIS ID"
                  className={`w-full ps-9 pe-3 ${fieldClass}`}
                />
              </div>
              {showDropdown && (
                <div className="absolute z-20 top-full start-0 end-0 mt-1 bg-surface-0 border border-border-default rounded-md shadow-lg max-h-48 overflow-y-auto">
                  {searching ? (
                    <div className="px-3 py-2 text-sm text-text-tertiary">Searching...</div>
                  ) : userResults.length === 0 ? (
                    <div className="px-3 py-2 text-sm text-text-tertiary">No users found</div>
                  ) : (
                    userResults.map((u) => (
                      <button
                        key={u.id}
                        type="button"
                        onMouseDown={(ev) => { ev.preventDefault(); setSelectedUser(u); setShowDropdown(false); }}
                        className="w-full flex flex-col px-3 py-2 hover:bg-brand-50 text-start"
                      >
                        <span className="text-sm font-medium text-text-primary truncate">{u.name || `User ${u.id}`}</span>
                        {u.email && <span className="text-xs text-text-tertiary truncate">{u.email}</span>}
                      </button>
                    ))
                  )}
                </div>
              )}
            </>
          )}
        </div>
        <div>
          <label htmlFor="add-person-role" className="block text-sm font-medium text-text-secondary mb-1">Role</label>
          <select id="add-person-role" value={type} onChange={(e) => setType(e.target.value)} className={fieldClass}>
            {roles.map((r) => <option key={r} value={r}>{ROLE_LABELS[r]}</option>)}
          </select>
        </div>
        {sections.length > 0 && (
          <div>
            <label htmlFor="add-person-section" className="block text-sm font-medium text-text-secondary mb-1">Section</label>
            <select id="add-person-section" value={sectionId} onChange={(e) => setSectionId(e.target.value)} className={fieldClass}>
              <option value="">Default section</option>
              {sections.map((s) => <option key={s.id} value={String(s.id)}>{s.name || `Section ${s.id}`}</option>)}
            </select>
          </div>
        )}
        <div className="flex flex-col gap-1 text-sm text-text-secondary">
          <label className="inline-flex items-center gap-2">
            <input type="checkbox" checked={invite} onChange={(e) => setInvite(e.target.checked)} />
            Send an invitation (they accept before joining)
          </label>
          <label className="inline-flex items-center gap-2">
            <input type="checkbox" checked={limit} onChange={(e) => setLimit(e.target.checked)} />
            Can only interact with people in their section
          </label>
        </div>
        <button
          type="submit"
          disabled={create.isPending || !selectedUser}
          className="bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm font-medium disabled:opacity-50"
        >
          {create.isPending ? 'Adding...' : invite ? 'Invite' : 'Add'}
        </button>
      </form>
      {error && <p role="alert" className="text-sm text-accent-danger mt-2">{error}</p>}
    </div>
  );
}
