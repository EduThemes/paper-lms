import React, { useState, useEffect, useMemo } from 'react';
import { Shield, Plus, Edit2, Trash2, Copy, Search, ChevronDown, ChevronRight, ToggleLeft, ToggleRight, Info, X } from 'lucide-react';
import Layout from '../components/Layout';
import { useAuth } from '../contexts/AuthContext';
import { getCSRFToken } from '../services/api';

const PERMISSION_CATEGORIES = {
  'Course Management': 'bg-brand-100 text-brand-800',
  'User Management': 'bg-accent-success/20 text-accent-success',
  'Administration': 'bg-purple-100 text-purple-800',
  'Grading & Submissions': 'bg-orange-100 text-orange-800',
};

const BASE_ROLE_TYPES = [
  { value: 'teacher', label: 'Teacher', color: 'bg-brand-100 text-brand-800' },
  { value: 'ta', label: 'Teaching Assistant', color: 'bg-accent-success/20 text-accent-success' },
  { value: 'student', label: 'Student', color: 'bg-accent-success/20 text-accent-success' },
  { value: 'observer', label: 'Observer', color: 'bg-surface-2 text-text-secondary' },
  { value: 'admin', label: 'Admin', color: 'bg-accent-danger/20 text-accent-danger' },
];

const CustomRolesPage = () => {
  const { user } = useAuth();
  const [roles, setRoles] = useState([]);
  const [presets, setPresets] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [editingRole, setEditingRole] = useState(null);
  const [permSearch, setPermSearch] = useState('');
  const [expandedCategories, setExpandedCategories] = useState({});
  const [selectedPreset, setSelectedPreset] = useState('');
  const [showCloneModal, setShowCloneModal] = useState(null);
  const [cloneName, setCloneName] = useState('');
  const [permissionDefs, setPermissionDefs] = useState([]);
  const [tooltipPerm, setTooltipPerm] = useState(null);

  const [formData, setFormData] = useState({
    name: '',
    base_role_type: 'teacher',
    label: '',
    permissions: {},
  });

  const accountId = 1;

  const authHeaders = () => ({
    'Content-Type': 'application/json',
    'X-CSRF-Token': getCSRFToken(),
  });

  const fetchRoles = async () => {
    try {
      const response = await fetch(`/api/v1/accounts/${accountId}/roles?per_page=100`, {
        credentials: 'include', headers: authHeaders(),
      });
      if (!response.ok) throw new Error('Failed to fetch roles');
      const data = await response.json();
      setRoles(data || []);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const fetchPresets = async () => {
    try {
      const response = await fetch(`/api/v1/accounts/${accountId}/roles/presets`, {
        credentials: 'include', headers: authHeaders(),
      });
      if (response.ok) {
        const data = await response.json();
        setPresets(data || []);
      }
    } catch {
      // Presets are optional
    }
  };

  const fetchPermissionDefs = async () => {
    try {
      // Get permission definitions from a role detail endpoint (any role ID or we parse from presets)
      // We'll use the AllPermissions data embedded in role detail responses
      const response = await fetch(`/api/v1/accounts/${accountId}/roles/presets`, {
        credentials: 'include', headers: authHeaders(),
      });
      if (response.ok) {
        // Permission definitions are also included in the presets endpoint
        // But we also need the full list. Let's build them from known constants.
        const allPerms = buildPermissionDefs();
        setPermissionDefs(allPerms);
      }
    } catch {
      setPermissionDefs(buildPermissionDefs());
    }
  };

  const buildPermissionDefs = () => [
    // Course Management
    { name: 'manage_content', label: 'Manage Content', description: 'Create, edit, and delete course content items', category: 'Course Management' },
    { name: 'manage_assignments', label: 'Manage Assignments', description: 'Create, edit, and delete assignments', category: 'Course Management' },
    { name: 'manage_grades', label: 'Manage Grades', description: 'Edit and manage the gradebook', category: 'Course Management' },
    { name: 'view_all_grades', label: 'View All Grades', description: 'View grades for all students in the course', category: 'Course Management' },
    { name: 'manage_sections', label: 'Manage Sections', description: 'Create, edit, and delete course sections', category: 'Course Management' },
    { name: 'manage_enrollments', label: 'Manage Enrollments', description: 'Add, remove, and modify student enrollments', category: 'Course Management' },
    { name: 'manage_calendar', label: 'Manage Calendar', description: 'Create and edit calendar events for the course', category: 'Course Management' },
    { name: 'manage_announcements', label: 'Manage Announcements', description: 'Create, edit, and delete course announcements', category: 'Course Management' },
    { name: 'manage_discussions', label: 'Manage Discussions', description: 'Create, edit, and moderate discussion topics', category: 'Course Management' },
    { name: 'manage_files', label: 'Manage Files', description: 'Upload, organize, and delete course files', category: 'Course Management' },
    { name: 'manage_pages', label: 'Manage Pages', description: 'Create, edit, and delete wiki pages', category: 'Course Management' },
    { name: 'manage_modules', label: 'Manage Modules', description: 'Create, edit, and organize course modules', category: 'Course Management' },
    { name: 'manage_quizzes', label: 'Manage Quizzes', description: 'Create, edit, and publish quizzes', category: 'Course Management' },
    { name: 'manage_rubrics', label: 'Manage Rubrics', description: 'Create, edit, and associate rubrics', category: 'Course Management' },
    { name: 'manage_outcomes', label: 'Manage Outcomes', description: 'Create and manage learning outcomes', category: 'Course Management' },
    { name: 'manage_groups', label: 'Manage Groups', description: 'Create and manage student groups', category: 'Course Management' },
    { name: 'manage_conferences', label: 'Manage Conferences', description: 'Create and manage web conferences', category: 'Course Management' },
    { name: 'manage_collaborations', label: 'Manage Collaborations', description: 'Create and manage collaborative documents', category: 'Course Management' },
    // User Management
    { name: 'send_messages', label: 'Send Messages', description: 'Send messages to course participants via inbox', category: 'User Management' },
    { name: 'view_analytics', label: 'View Analytics', description: 'Access course and student analytics dashboards', category: 'User Management' },
    { name: 'view_user_email', label: 'View User Email', description: 'View email addresses of enrolled users', category: 'User Management' },
    { name: 'manage_user_notes', label: 'Manage User Notes', description: 'Create and view faculty journal notes about students', category: 'User Management' },
    { name: 'read_roster', label: 'Read Roster', description: 'View the list of enrolled students in the course', category: 'User Management' },
    // Administration
    { name: 'manage_courses', label: 'Manage Courses', description: 'Create, edit, and delete courses at the account level', category: 'Administration' },
    { name: 'manage_account_settings', label: 'Manage Account Settings', description: 'Modify account-level settings and configurations', category: 'Administration' },
    { name: 'manage_developer_keys', label: 'Manage Developer Keys', description: 'Create and manage OAuth2 developer keys and API tokens', category: 'Administration' },
    { name: 'manage_sis', label: 'Manage SIS', description: 'Import and export SIS data for the account', category: 'Administration' },
    { name: 'manage_auth_providers', label: 'Manage Auth Providers', description: 'Configure SSO and authentication providers', category: 'Administration' },
    { name: 'manage_users', label: 'Manage Users', description: 'Create, edit, and deactivate user accounts', category: 'Administration' },
    { name: 'view_audit_log', label: 'View Audit Log', description: 'Access the system audit log for compliance tracking', category: 'Administration' },
    { name: 'manage_enrollment_terms', label: 'Manage Enrollment Terms', description: 'Create and edit academic terms and enrollment periods', category: 'Administration' },
    { name: 'manage_blueprint', label: 'Manage Blueprint Courses', description: 'Create and sync blueprint course templates', category: 'Administration' },
    { name: 'manage_pacing', label: 'Manage Course Pacing', description: 'Configure and manage course pacing plans', category: 'Administration' },
    // Grading & Submissions
    { name: 'grade_submissions', label: 'Grade Submissions', description: 'Assign grades and scores to student submissions', category: 'Grading & Submissions' },
    { name: 'comment_on_submissions', label: 'Comment on Submissions', description: 'Add feedback comments to student submissions', category: 'Grading & Submissions' },
    { name: 'view_submission_details', label: 'View Submission Details', description: 'View full submission content and metadata', category: 'Grading & Submissions' },
    { name: 'moderate_grades', label: 'Moderate Grades', description: 'Review and approve grades from multiple graders', category: 'Grading & Submissions' },
  ];

  useEffect(() => {
    fetchRoles();
    fetchPresets();
    fetchPermissionDefs();
  }, []);

  const parsePermissions = (permJson) => {
    if (!permJson || permJson === '{}') return {};
    try {
      const parsed = JSON.parse(permJson);
      const result = {};
      Object.entries(parsed).forEach(([key, val]) => {
        result[key] = typeof val === 'object' ? val.enabled : !!val;
      });
      return result;
    } catch {
      return {};
    }
  };

  const countPermissions = (permJson) => {
    const perms = parsePermissions(permJson);
    return Object.values(perms).filter(Boolean).length;
  };

  const resetForm = () => {
    setFormData({ name: '', base_role_type: 'teacher', label: '', permissions: {} });
    setEditingRole(null);
    setShowCreateForm(false);
    setSelectedPreset('');
    setPermSearch('');
  };

  const handlePresetSelect = (presetName) => {
    setSelectedPreset(presetName);
    const preset = presets.find((p) => p.name === presetName);
    if (preset) {
      const permObj = {};
      preset.permissions.forEach((p) => {
        permObj[p] = true;
      });
      setFormData((prev) => ({ ...prev, permissions: permObj }));
    }
  };

  const togglePermission = (permName) => {
    setFormData((prev) => ({
      ...prev,
      permissions: {
        ...prev.permissions,
        [permName]: !prev.permissions[permName],
      },
    }));
  };

  const toggleCategory = (category) => {
    setExpandedCategories((prev) => ({
      ...prev,
      [category]: !prev[category],
    }));
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    setError(null);

    const permissionsPayload = {};
    Object.entries(formData.permissions).forEach(([key, val]) => {
      permissionsPayload[key] = { enabled: !!val, locked: false };
    });

    const payload = {
      role: {
        name: formData.name,
        base_role_type: formData.base_role_type,
        label: formData.label || formData.name,
        permissions: JSON.stringify(permissionsPayload),
      },
    };

    try {
      const url = editingRole
        ? `/api/v1/accounts/${accountId}/roles/${editingRole.id}`
        : `/api/v1/accounts/${accountId}/roles`;
      const method = editingRole ? 'PUT' : 'POST';

      const response = await fetch(url, {
        method,
        credentials: 'include', headers: authHeaders(),
        body: JSON.stringify(payload),
      });

      if (!response.ok) {
        const body = await response.json().catch(() => ({}));
        throw new Error(body.errors?.[0]?.message || 'Failed to save role');
      }

      resetForm();
      fetchRoles();
    } catch (err) {
      setError(err.message);
    }
  };

  const handleEdit = (role) => {
    setEditingRole(role);
    setFormData({
      name: role.name || '',
      base_role_type: role.base_role_type || 'teacher',
      label: role.label || '',
      permissions: parsePermissions(role.permissions),
    });
    setShowCreateForm(true);
    // Expand all categories when editing
    const expanded = {};
    Object.keys(PERMISSION_CATEGORIES).forEach((cat) => {
      expanded[cat] = true;
    });
    setExpandedCategories(expanded);
  };

  const handleDelete = async (roleId) => {
    if (!window.confirm('Are you sure you want to delete this role? Users with this role will revert to their base role permissions.')) return;
    try {
      const response = await fetch(`/api/v1/accounts/${accountId}/roles/${roleId}`, {
        method: 'DELETE',
        credentials: 'include', headers: authHeaders(),
      });
      if (!response.ok) throw new Error('Failed to delete role');
      fetchRoles();
    } catch (err) {
      setError(err.message);
    }
  };

  const handleClone = async () => {
    if (!showCloneModal) return;
    setError(null);
    try {
      const response = await fetch(`/api/v1/accounts/${accountId}/roles/${showCloneModal.id}/clone`, {
        method: 'POST',
        credentials: 'include', headers: authHeaders(),
        body: JSON.stringify({ name: cloneName }),
      });
      if (!response.ok) {
        const body = await response.json().catch(() => ({}));
        throw new Error(body.errors?.[0]?.message || 'Failed to clone role');
      }
      setShowCloneModal(null);
      setCloneName('');
      fetchRoles();
    } catch (err) {
      setError(err.message);
    }
  };

  const filteredPermissions = useMemo(() => {
    if (!permSearch) return permissionDefs;
    const query = permSearch.toLowerCase();
    return permissionDefs.filter(
      (p) =>
        p.name.toLowerCase().includes(query) ||
        p.label.toLowerCase().includes(query) ||
        p.description.toLowerCase().includes(query) ||
        p.category.toLowerCase().includes(query)
    );
  }, [permSearch, permissionDefs]);

  const permissionsByCategory = useMemo(() => {
    const grouped = {};
    filteredPermissions.forEach((p) => {
      if (!grouped[p.category]) grouped[p.category] = [];
      grouped[p.category].push(p);
    });
    return grouped;
  }, [filteredPermissions]);

  const getBaseRoleBadge = (baseType) => {
    const roleType = BASE_ROLE_TYPES.find((r) => r.value === baseType);
    if (!roleType) return null;
    return (
      <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${roleType.color}`}>
        {roleType.label}
      </span>
    );
  };

  if (loading) {
    return (
      <Layout>
        <div className="text-center py-12 text-text-tertiary" role="status" aria-live="polite">
          Loading custom roles...
        </div>
      </Layout>
    );
  }

  return (
    <Layout>
      <div className="mb-6">
        <div className="flex items-center justify-between">
          <div className="flex items-center space-x-3">
            <Shield className="w-6 h-6 text-indigo-600" aria-hidden="true" />
            <h1 className="text-2xl font-bold text-text-primary">Custom Roles & Permissions</h1>
          </div>
          <button
            onClick={() => { resetForm(); setShowCreateForm(!showCreateForm); }}
            className="flex items-center space-x-2 bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:ring-offset-2"
            aria-expanded={showCreateForm}
          >
            <Plus className="w-4 h-4" aria-hidden="true" />
            <span>New Role</span>
          </button>
        </div>
        <p className="mt-2 text-sm text-text-tertiary">
          Create and manage custom roles with granular permissions. Start from a preset template or build from scratch.
        </p>
      </div>

      {error && (
        <div className="bg-accent-danger/10 border border-accent-danger/30 text-accent-danger p-3 rounded-md mb-4" role="alert">
          {error}
          <button onClick={() => setError(null)} className="ml-2 text-accent-danger hover:text-accent-danger text-sm" aria-label="Dismiss error">
            Dismiss
          </button>
        </div>
      )}

      {/* Clone Modal */}
      {showCloneModal && (
        <div className="fixed inset-0 bg-black bg-opacity-50 z-50 flex items-center justify-center" role="dialog" aria-modal="true" aria-label="Clone role">
          <div className="bg-surface-0 rounded-lg shadow-xl p-6 w-full max-w-md mx-4">
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-lg font-semibold">Clone Role: {showCloneModal.name}</h3>
              <button onClick={() => setShowCloneModal(null)} className="text-text-disabled hover:text-text-secondary" aria-label="Close dialog">
                <X className="w-5 h-5" aria-hidden="true" />
              </button>
            </div>
            <div className="mb-4">
              <label htmlFor="clone-name" className="block text-sm font-medium text-text-secondary mb-1">
                New Role Name
              </label>
              <input
                id="clone-name"
                type="text"
                value={cloneName}
                onChange={(e) => setCloneName(e.target.value)}
                className="w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                placeholder={`${showCloneModal.name} (Copy)`}
                autoFocus
              />
            </div>
            <div className="flex items-center justify-end space-x-3">
              <button
                onClick={() => setShowCloneModal(null)}
                className="text-text-secondary hover:text-text-primary text-sm px-4 py-2"
              >
                Cancel
              </button>
              <button
                onClick={handleClone}
                className="bg-brand-600 text-white px-4 py-2 rounded-md text-sm hover:bg-brand-700 focus:outline-none focus:ring-2 focus:ring-brand-500 focus:ring-offset-2"
              >
                Clone Role
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Create/Edit Form */}
      {showCreateForm && (
        <div className="bg-surface-0 rounded-lg shadow p-6 mb-6" role="region" aria-label={editingRole ? 'Edit role' : 'Create role'}>
          <h2 className="text-lg font-semibold mb-4">{editingRole ? 'Edit Role' : 'Create New Role'}</h2>
          <form onSubmit={handleSubmit} className="space-y-6">
            {/* Basic Info */}
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              <div>
                <label htmlFor="role-name" className="block text-sm font-medium text-text-secondary mb-1">
                  Role Name <span className="text-accent-danger">*</span>
                </label>
                <input
                  id="role-name"
                  type="text"
                  value={formData.name}
                  onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                  className="w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                  placeholder="e.g. Department Chair"
                  required
                  aria-required="true"
                />
              </div>
              <div>
                <label htmlFor="role-base-type" className="block text-sm font-medium text-text-secondary mb-1">
                  Base Role Type <span className="text-accent-danger">*</span>
                </label>
                <select
                  id="role-base-type"
                  value={formData.base_role_type}
                  onChange={(e) => setFormData({ ...formData, base_role_type: e.target.value })}
                  className="w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                  aria-required="true"
                >
                  {BASE_ROLE_TYPES.map((rt) => (
                    <option key={rt.value} value={rt.value}>{rt.label}</option>
                  ))}
                </select>
              </div>
              <div>
                <label htmlFor="role-label" className="block text-sm font-medium text-text-secondary mb-1">
                  Display Label
                </label>
                <input
                  id="role-label"
                  type="text"
                  value={formData.label}
                  onChange={(e) => setFormData({ ...formData, label: e.target.value })}
                  className="w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                  placeholder="Optional display label"
                />
              </div>
            </div>

            {/* Preset Selector */}
            {!editingRole && presets.length > 0 && (
              <div>
                <label htmlFor="role-preset" className="block text-sm font-medium text-text-secondary mb-1">
                  Start from Preset Template
                </label>
                <select
                  id="role-preset"
                  value={selectedPreset}
                  onChange={(e) => handlePresetSelect(e.target.value)}
                  className="w-full md:w-1/2 border border-border-strong rounded-md px-3 py-2 text-sm focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                >
                  <option value="">-- Select a preset (optional) --</option>
                  {presets.map((p) => (
                    <option key={p.name} value={p.name}>{p.label} - {p.description}</option>
                  ))}
                </select>
              </div>
            )}

            {/* Permission Search */}
            <div>
              <div className="flex items-center justify-between mb-2">
                <h3 className="text-sm font-medium text-text-secondary">Permissions</h3>
                <span className="text-xs text-text-tertiary">
                  {Object.values(formData.permissions).filter(Boolean).length} of {permissionDefs.length} enabled
                </span>
              </div>
              <div className="relative mb-3">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-text-disabled" aria-hidden="true" />
                <input
                  type="text"
                  value={permSearch}
                  onChange={(e) => setPermSearch(e.target.value)}
                  className="w-full pl-10 pr-4 py-2 border border-border-strong rounded-md text-sm focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                  placeholder="Search permissions..."
                  aria-label="Search permissions"
                />
                {permSearch && (
                  <button
                    onClick={() => setPermSearch('')}
                    className="absolute right-3 top-1/2 -translate-y-1/2 text-text-disabled hover:text-text-secondary"
                    aria-label="Clear search"
                  >
                    <X className="w-4 h-4" aria-hidden="true" />
                  </button>
                )}
              </div>

              {/* Permission Categories */}
              <div className="border border-border-default rounded-lg divide-y divide-border-default">
                {Object.entries(permissionsByCategory).map(([category, perms]) => (
                  <div key={category}>
                    <button
                      type="button"
                      onClick={() => toggleCategory(category)}
                      className="w-full flex items-center justify-between px-4 py-3 hover:bg-surface-1 focus:outline-none focus:ring-2 focus:ring-inset focus:ring-brand-500"
                      aria-expanded={expandedCategories[category] || false}
                    >
                      <div className="flex items-center space-x-3">
                        {expandedCategories[category] ? (
                          <ChevronDown className="w-4 h-4 text-text-tertiary" aria-hidden="true" />
                        ) : (
                          <ChevronRight className="w-4 h-4 text-text-tertiary" aria-hidden="true" />
                        )}
                        <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${PERMISSION_CATEGORIES[category] || 'bg-surface-2 text-text-secondary'}`}>
                          {category}
                        </span>
                      </div>
                      <span className="text-xs text-text-tertiary">
                        {perms.filter((p) => formData.permissions[p.name]).length}/{perms.length}
                      </span>
                    </button>
                    {expandedCategories[category] && (
                      <div className="px-4 pb-3 space-y-1">
                        {perms.map((perm) => (
                          <div
                            key={perm.name}
                            className="flex items-center justify-between py-2 px-3 rounded-md hover:bg-surface-1"
                          >
                            <div className="flex items-center space-x-3 flex-1 min-w-0">
                              <button
                                type="button"
                                onClick={() => togglePermission(perm.name)}
                                className="flex-shrink-0 focus:outline-none focus:ring-2 focus:ring-brand-500 rounded"
                                role="switch"
                                aria-checked={!!formData.permissions[perm.name]}
                                aria-label={`Toggle ${perm.label}`}
                              >
                                {formData.permissions[perm.name] ? (
                                  <ToggleRight className="w-8 h-5 text-brand-600" aria-hidden="true" />
                                ) : (
                                  <ToggleLeft className="w-8 h-5 text-text-disabled" aria-hidden="true" />
                                )}
                              </button>
                              <div className="min-w-0">
                                <span className="text-sm font-medium text-text-primary">{perm.label}</span>
                                <span className="ml-2 text-xs text-text-disabled font-mono">{perm.name}</span>
                              </div>
                            </div>
                            <div className="relative flex-shrink-0">
                              <button
                                type="button"
                                onMouseEnter={() => setTooltipPerm(perm.name)}
                                onMouseLeave={() => setTooltipPerm(null)}
                                onFocus={() => setTooltipPerm(perm.name)}
                                onBlur={() => setTooltipPerm(null)}
                                className="text-text-disabled hover:text-text-secondary p-1 focus:outline-none focus:ring-2 focus:ring-brand-500 rounded"
                                aria-label={`Info about ${perm.label}`}
                              >
                                <Info className="w-4 h-4" aria-hidden="true" />
                              </button>
                              {tooltipPerm === perm.name && (
                                <div
                                  className="absolute right-0 bottom-full mb-2 w-64 bg-surface-2 text-white text-xs rounded-md px-3 py-2 shadow-lg z-10"
                                  role="tooltip"
                                >
                                  {perm.description}
                                  <div className="absolute right-4 top-full w-0 h-0 border-l-4 border-r-4 border-t-4 border-transparent border-t-gray-900" />
                                </div>
                              )}
                            </div>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            </div>

            {/* Form Actions */}
            <div className="flex items-center space-x-3">
              <button
                type="submit"
                className="bg-brand-600 text-white px-4 py-2 rounded-md text-sm hover:bg-brand-700 focus:outline-none focus:ring-2 focus:ring-brand-500 focus:ring-offset-2"
              >
                {editingRole ? 'Update Role' : 'Create Role'}
              </button>
              <button
                type="button"
                onClick={resetForm}
                className="text-text-secondary hover:text-text-primary text-sm"
              >
                Cancel
              </button>
            </div>
          </form>
        </div>
      )}

      {/* Roles List */}
      {roles.length === 0 ? (
        <div className="bg-surface-0 rounded-lg shadow p-12 text-center">
          <Shield className="w-12 h-12 text-text-disabled mx-auto mb-4" aria-hidden="true" />
          <h3 className="text-lg font-medium text-text-primary mb-1">No custom roles</h3>
          <p className="text-text-tertiary text-sm">Create your first custom role to grant granular permissions beyond the default Canvas roles.</p>
        </div>
      ) : (
        <div className="bg-surface-0 rounded-lg shadow overflow-hidden">
          <table className="min-w-full divide-y divide-border-default" role="table" aria-label="Custom roles">
            <thead className="bg-surface-1">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">Role</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">Base Type</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">Permissions</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">State</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">Created</th>
                <th className="px-4 py-3 text-right text-xs font-medium text-text-tertiary uppercase tracking-wider">Actions</th>
              </tr>
            </thead>
            <tbody className="bg-surface-0 divide-y divide-border-default">
              {roles.map((role) => (
                <tr key={role.id} className="hover:bg-surface-1">
                  <td className="px-4 py-3 whitespace-nowrap">
                    <div>
                      <span className="text-sm font-medium text-text-primary">{role.name}</span>
                      {role.label && role.label !== role.name && (
                        <span className="ml-2 text-xs text-text-tertiary">({role.label})</span>
                      )}
                    </div>
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap">
                    {getBaseRoleBadge(role.base_role_type)}
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap">
                    <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-indigo-100 text-indigo-800">
                      {countPermissions(role.permissions)} permissions
                    </span>
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap">
                    <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${
                      role.workflow_state === 'active' ? 'bg-accent-success/20 text-accent-success' :
                      role.workflow_state === 'inactive' ? 'bg-accent-warning/20 text-accent-warning' :
                      'bg-accent-danger/20 text-accent-danger'
                    }`}>
                      {role.workflow_state}
                    </span>
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap text-sm text-text-tertiary">
                    {role.created_at ? new Date(role.created_at).toLocaleDateString() : '--'}
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap text-right">
                    <div className="flex items-center justify-end space-x-2">
                      <button
                        onClick={() => handleEdit(role)}
                        className="text-text-disabled hover:text-brand-600 p-1 rounded focus:outline-none focus:ring-2 focus:ring-brand-500"
                        aria-label={`Edit ${role.name}`}
                        title="Edit role"
                      >
                        <Edit2 className="w-4 h-4" aria-hidden="true" />
                      </button>
                      <button
                        onClick={() => { setShowCloneModal(role); setCloneName(role.name + ' (Copy)'); }}
                        className="text-text-disabled hover:text-indigo-600 p-1 rounded focus:outline-none focus:ring-2 focus:ring-indigo-500"
                        aria-label={`Clone ${role.name}`}
                        title="Clone role"
                      >
                        <Copy className="w-4 h-4" aria-hidden="true" />
                      </button>
                      <button
                        onClick={() => handleDelete(role.id)}
                        className="text-text-disabled hover:text-accent-danger p-1 rounded focus:outline-none focus:ring-2 focus:ring-accent-danger"
                        aria-label={`Delete ${role.name}`}
                        title="Delete role"
                      >
                        <Trash2 className="w-4 h-4" aria-hidden="true" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Layout>
  );
};

export default CustomRolesPage;
