import React, { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import { Users, FolderOpen, Plus, X, Edit2, Trash2, UserPlus, UserMinus, ToggleLeft, ToggleRight } from 'lucide-react';
import { api } from '../services/api';
import { useAuth } from '../contexts/AuthContext';
import useIsTeacher from '../hooks/useIsTeacher';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import { Skeleton } from '@/components/ui/skeleton';

const GroupsPage = () => {
  const { courseId } = useParams();
  const { user } = useAuth();
  const [categories, setCategories] = useState([]);
  const [groupsByCategory, setGroupsByCategory] = useState({});
  const [membersByGroup, setMembersByGroup] = useState({});
  const [enrollments, setEnrollments] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  // Category modal
  const [showCategoryModal, setShowCategoryModal] = useState(false);
  const [editingCategory, setEditingCategory] = useState(null);
  const [categoryForm, setCategoryForm] = useState({ name: '', self_signup: '', group_limit: '', auto_leader: '' });

  // Group modal
  const [showGroupModal, setShowGroupModal] = useState(false);
  const [editingGroup, setEditingGroup] = useState(null);
  const [groupCategoryTarget, setGroupCategoryTarget] = useState(null);
  const [groupForm, setGroupForm] = useState({ name: '', description: '', max_membership: '', join_level: 'invitation_only' });

  // Member assignment
  const [addingMemberGroup, setAddingMemberGroup] = useState(null);
  const [selectedUserId, setSelectedUserId] = useState('');

  const [saving, setSaving] = useState(false);
  const isTeacher = useIsTeacher(courseId);

  const fetchData = async () => {
    try {
      const catResult = await api.getGroupCategories(courseId);
      const cats = catResult.data || [];
      setCategories(cats);

      const groupsMap = {};
      const membersMap = {};
      for (const cat of cats) {
        const groupResult = await api.getGroupsByCategory(cat.id);
        const groups = groupResult.data || [];
        groupsMap[cat.id] = groups;
        for (const group of groups) {
          const memberResult = await api.getGroupMemberships(group.id);
          membersMap[group.id] = memberResult.data || [];
        }
      }
      setGroupsByCategory(groupsMap);
      setMembersByGroup(membersMap);

      const enrollResult = await api.getEnrollments(courseId);
      const enrollList = enrollResult.data || [];
      setEnrollments(enrollList);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, [courseId]);

  // ---- Category actions ----

  const openCreateCategory = () => {
    setEditingCategory(null);
    setCategoryForm({ name: '', self_signup: '', group_limit: '', auto_leader: '' });
    setShowCategoryModal(true);
  };

  const openEditCategory = (cat) => {
    setEditingCategory(cat);
    setCategoryForm({
      name: cat.name,
      self_signup: cat.self_signup || '',
      group_limit: cat.group_limit != null ? String(cat.group_limit) : '',
      auto_leader: cat.auto_leader || '',
    });
    setShowCategoryModal(true);
  };

  const handleSaveCategory = async (e) => {
    e.preventDefault();
    setSaving(true);
    try {
      const payload = {
        name: categoryForm.name,
        self_signup: categoryForm.self_signup || '',
        auto_leader: categoryForm.auto_leader || '',
      };
      if (categoryForm.group_limit !== '') {
        payload.group_limit = parseInt(categoryForm.group_limit, 10);
      }
      if (editingCategory) {
        await api.updateGroupCategory(editingCategory.id, payload);
      } else {
        await api.createGroupCategory(courseId, payload);
      }
      setShowCategoryModal(false);
      setLoading(true);
      await fetchData();
    } catch (err) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  const handleDeleteCategory = async (catId) => {
    if (!window.confirm('Are you sure you want to delete this group category?')) return;
    try {
      await api.deleteGroupCategory(catId);
      setLoading(true);
      await fetchData();
    } catch (err) {
      setError(err.message);
    }
  };

  // ---- Group actions ----

  const openCreateGroup = (catId) => {
    setEditingGroup(null);
    setGroupCategoryTarget(catId);
    setGroupForm({ name: '', description: '', max_membership: '', join_level: 'invitation_only' });
    setShowGroupModal(true);
  };

  const openEditGroup = (group, catId) => {
    setEditingGroup(group);
    setGroupCategoryTarget(catId);
    setGroupForm({
      name: group.name,
      description: group.description || '',
      max_membership: group.max_membership != null ? String(group.max_membership) : '',
      join_level: group.join_level || 'invitation_only',
    });
    setShowGroupModal(true);
  };

  const handleSaveGroup = async (e) => {
    e.preventDefault();
    setSaving(true);
    try {
      const payload = {
        name: groupForm.name,
        description: groupForm.description,
        join_level: groupForm.join_level,
        context_type: 'Course',
        context_id: parseInt(courseId, 10),
      };
      if (groupForm.max_membership !== '') {
        payload.max_membership = parseInt(groupForm.max_membership, 10);
      }
      if (editingGroup) {
        await api.updateGroup(editingGroup.id, payload);
      } else {
        await api.createGroup(groupCategoryTarget, payload);
      }
      setShowGroupModal(false);
      setLoading(true);
      await fetchData();
    } catch (err) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  const handleDeleteGroup = async (groupId) => {
    if (!window.confirm('Are you sure you want to delete this group?')) return;
    try {
      await api.deleteGroup(groupId);
      setLoading(true);
      await fetchData();
    } catch (err) {
      setError(err.message);
    }
  };

  // ---- Member actions ----

  const handleAddMember = async (groupId) => {
    if (!selectedUserId) return;
    setSaving(true);
    try {
      await api.createGroupMembership(groupId, { user_id: parseInt(selectedUserId, 10) });
      setAddingMemberGroup(null);
      setSelectedUserId('');
      setLoading(true);
      await fetchData();
    } catch (err) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  const handleRemoveMember = async (membershipId) => {
    if (!window.confirm('Remove this member from the group?')) return;
    try {
      await api.deleteGroupMembership(membershipId);
      setLoading(true);
      await fetchData();
    } catch (err) {
      setError(err.message);
    }
  };

  const getMembersInGroup = (groupId) => {
    return membersByGroup[groupId] || [];
  };

  const getAvailableUsers = (groupId) => {
    const currentMembers = getMembersInGroup(groupId);
    const memberUserIds = new Set(currentMembers.map((m) => m.user_id));
    return enrollments
      .filter((e) => !memberUserIds.has(e.user_id))
      .map((e) => e.user || { id: e.user_id, name: `User ${e.user_id}` });
  };

  if (loading) {
    return (
      <Layout>
        <CourseNav />
        <div className="space-y-3 p-6">
          <Skeleton className="h-9 w-48" />
          <Skeleton className="h-12 w-full" />
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-16 w-full" />
          ))}
        </div>
      </Layout>
    );
  }

  if (error) {
    return (
      <Layout>
        <div className="text-center py-12">
          <p className="text-accent-danger mb-3">{error}</p>
          <button onClick={() => window.location.reload()} className="text-brand-600 hover:text-brand-800 text-sm font-medium">Try Again</button>
        </div>
      </Layout>
    );
  }

  return (
    <Layout>
      <CourseNav />
      <div className="mb-6">
        <Link to={`/courses/${courseId}`} className="text-brand-600 hover:underline text-sm">
          &larr; Back to Course
        </Link>
        <div className="flex items-center justify-between mt-2">
          <h2 className="text-2xl font-bold text-text-primary">Groups</h2>
          {isTeacher && (
            <button
              onClick={openCreateCategory}
              className="inline-flex items-center space-x-2 bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm font-medium"
            >
              <Plus className="w-4 h-4" />
              <span>New Category</span>
            </button>
          )}
        </div>
      </div>

      {categories.length === 0 ? (
        <div className="bg-surface-0 rounded-lg shadow p-6 text-center text-text-tertiary">
          No group categories yet. Create one to get started.
        </div>
      ) : (
        <div className="space-y-6">
          {categories.map((cat) => (
            <div key={cat.id} className="bg-surface-0 rounded-lg shadow">
              <div className="p-4 border-b flex items-center justify-between">
                <div className="flex items-center space-x-3">
                  <FolderOpen className="w-5 h-5 text-text-disabled" />
                  <div>
                    <h3 className="font-semibold text-text-primary">{cat.name}</h3>
                    <div className="flex items-center space-x-3 text-xs text-text-tertiary mt-0.5">
                      {cat.self_signup && (
                        <span className="inline-flex items-center space-x-1">
                          {cat.self_signup === 'enabled' ? (
                            <ToggleRight className="w-3.5 h-3.5 text-accent-success" />
                          ) : (
                            <ToggleLeft className="w-3.5 h-3.5 text-accent-warning" />
                          )}
                          <span>Self-signup: {cat.self_signup}</span>
                        </span>
                      )}
                      {cat.group_limit != null && <span>Limit: {cat.group_limit}</span>}
                      {cat.auto_leader && <span>Leader: {cat.auto_leader}</span>}
                    </div>
                  </div>
                </div>
                {isTeacher && (
                  <div className="flex items-center space-x-2">
                    <button
                      onClick={() => openCreateGroup(cat.id)}
                      className="inline-flex items-center space-x-1 text-brand-600 hover:text-brand-800 text-sm"
                      title="Add Group"
                    >
                      <Plus className="w-4 h-4" />
                      <span>Add Group</span>
                    </button>
                    <button
                      onClick={() => openEditCategory(cat)}
                      className="text-text-disabled hover:text-text-secondary"
                      title="Edit Category"
                    >
                      <Edit2 className="w-4 h-4" />
                    </button>
                    <button
                      onClick={() => handleDeleteCategory(cat.id)}
                      className="text-text-disabled hover:text-accent-danger"
                      title="Delete Category"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </div>
                )}
              </div>

              {/* Groups within category */}
              <div className="divide-y">
                {(!groupsByCategory[cat.id] || groupsByCategory[cat.id].length === 0) ? (
                  <div className="p-4 text-center text-text-disabled text-sm">No groups in this category.</div>
                ) : (
                  groupsByCategory[cat.id].map((group) => (
                    <div key={group.id} className="p-4">
                      <div className="flex items-center justify-between mb-2">
                        <div className="flex items-center space-x-2">
                          <Users className="w-4 h-4 text-text-disabled" />
                          <span className="font-medium text-text-primary">{group.name}</span>
                          {group.max_membership != null && (
                            <span className="text-xs text-text-tertiary">
                              ({getMembersInGroup(group.id).length}/{group.max_membership} members)
                            </span>
                          )}
                          {group.max_membership == null && (
                            <span className="text-xs text-text-tertiary">
                              ({getMembersInGroup(group.id).length} members)
                            </span>
                          )}
                        </div>
                        {isTeacher && (
                          <div className="flex items-center space-x-2">
                            <button
                              onClick={() => {
                                setAddingMemberGroup(addingMemberGroup === group.id ? null : group.id);
                                setSelectedUserId('');
                              }}
                              className="text-accent-success hover:text-accent-success text-sm inline-flex items-center space-x-1"
                              title="Add Member"
                            >
                              <UserPlus className="w-4 h-4" />
                            </button>
                            <button
                              onClick={() => openEditGroup(group, cat.id)}
                              className="text-text-disabled hover:text-text-secondary"
                              title="Edit Group"
                            >
                              <Edit2 className="w-4 h-4" />
                            </button>
                            <button
                              onClick={() => handleDeleteGroup(group.id)}
                              className="text-text-disabled hover:text-accent-danger"
                              title="Delete Group"
                            >
                              <Trash2 className="w-4 h-4" />
                            </button>
                          </div>
                        )}
                      </div>

                      {group.description && (
                        <p className="text-sm text-text-tertiary mb-2">{group.description}</p>
                      )}

                      {/* Add member row */}
                      {addingMemberGroup === group.id && (
                        <div className="flex items-center space-x-2 mb-2 bg-surface-1 p-2 rounded">
                          <select
                            value={selectedUserId}
                            onChange={(e) => setSelectedUserId(e.target.value)}
                            className="flex-1 border border-border-strong rounded px-2 py-1 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                          >
                            <option value="">Select a user...</option>
                            {getAvailableUsers(group.id).map((u) => (
                              <option key={u.id} value={u.id}>
                                {u.name} ({u.email || u.login_id || `ID: ${u.id}`})
                              </option>
                            ))}
                          </select>
                          <button
                            onClick={() => handleAddMember(group.id)}
                            disabled={!selectedUserId || saving}
                            className="bg-accent-success text-white px-3 py-1 rounded text-sm hover:bg-accent-success/90 disabled:opacity-50"
                          >
                            Add
                          </button>
                          <button
                            onClick={() => { setAddingMemberGroup(null); setSelectedUserId(''); }}
                            className="text-text-disabled hover:text-text-secondary"
                          >
                            <X className="w-4 h-4" />
                          </button>
                        </div>
                      )}

                      {/* Members list */}
                      {getMembersInGroup(group.id).length > 0 && (
                        <div className="flex flex-wrap gap-2">
                          {getMembersInGroup(group.id).map((member) => (
                            <span
                              key={member.id}
                              className="inline-flex items-center space-x-1 bg-surface-2 text-text-secondary text-xs px-2 py-1 rounded-full"
                            >
                              <span>{member.user ? member.user.name : `User ${member.user_id}`}</span>
                              {member.moderator && (
                                <span className="text-brand-600 font-semibold">(mod)</span>
                              )}
                              {isTeacher && (
                                <button
                                  onClick={() => handleRemoveMember(member.id)}
                                  className="text-text-disabled hover:text-accent-danger ml-1"
                                  title="Remove member"
                                >
                                  <UserMinus className="w-3 h-3" />
                                </button>
                              )}
                            </span>
                          ))}
                        </div>
                      )}
                    </div>
                  ))
                )}
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Category Modal */}
      {showCategoryModal && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-surface-0 rounded-lg shadow-lg p-6 w-full max-w-md mx-4">
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-lg font-semibold">{editingCategory ? 'Edit Category' : 'New Category'}</h3>
              <button onClick={() => setShowCategoryModal(false)} className="text-text-disabled hover:text-text-secondary">
                <X className="w-5 h-5" />
              </button>
            </div>
            <form onSubmit={handleSaveCategory} className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-text-secondary mb-1">Name</label>
                <input
                  type="text"
                  value={categoryForm.name}
                  onChange={(e) => setCategoryForm({ ...categoryForm, name: e.target.value })}
                  className="w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                  required
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-text-secondary mb-1">Self Signup</label>
                <select
                  value={categoryForm.self_signup}
                  onChange={(e) => setCategoryForm({ ...categoryForm, self_signup: e.target.value })}
                  className="w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                >
                  <option value="">Disabled</option>
                  <option value="enabled">Enabled</option>
                  <option value="restricted">Restricted</option>
                </select>
              </div>
              <div>
                <label className="block text-sm font-medium text-text-secondary mb-1">Group Limit</label>
                <input
                  type="number"
                  value={categoryForm.group_limit}
                  onChange={(e) => setCategoryForm({ ...categoryForm, group_limit: e.target.value })}
                  className="w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                  placeholder="No limit"
                  min="1"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-text-secondary mb-1">Auto Leader</label>
                <select
                  value={categoryForm.auto_leader}
                  onChange={(e) => setCategoryForm({ ...categoryForm, auto_leader: e.target.value })}
                  className="w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                >
                  <option value="">None</option>
                  <option value="first">First Member</option>
                  <option value="random">Random</option>
                </select>
              </div>
              <div className="flex justify-end space-x-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowCategoryModal(false)}
                  className="px-4 py-2 text-sm text-text-secondary border border-border-strong rounded-md hover:bg-surface-1"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={saving}
                  className="bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm font-medium disabled:opacity-50"
                >
                  {saving ? 'Saving...' : editingCategory ? 'Update' : 'Create'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Group Modal */}
      {showGroupModal && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-surface-0 rounded-lg shadow-lg p-6 w-full max-w-md mx-4">
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-lg font-semibold">{editingGroup ? 'Edit Group' : 'New Group'}</h3>
              <button onClick={() => setShowGroupModal(false)} className="text-text-disabled hover:text-text-secondary">
                <X className="w-5 h-5" />
              </button>
            </div>
            <form onSubmit={handleSaveGroup} className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-text-secondary mb-1">Name</label>
                <input
                  type="text"
                  value={groupForm.name}
                  onChange={(e) => setGroupForm({ ...groupForm, name: e.target.value })}
                  className="w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                  required
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-text-secondary mb-1">Description</label>
                <textarea
                  value={groupForm.description}
                  onChange={(e) => setGroupForm({ ...groupForm, description: e.target.value })}
                  className="w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                  rows={3}
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-text-secondary mb-1">Max Membership</label>
                <input
                  type="number"
                  value={groupForm.max_membership}
                  onChange={(e) => setGroupForm({ ...groupForm, max_membership: e.target.value })}
                  className="w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                  placeholder="No limit"
                  min="1"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-text-secondary mb-1">Join Level</label>
                <select
                  value={groupForm.join_level}
                  onChange={(e) => setGroupForm({ ...groupForm, join_level: e.target.value })}
                  className="w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                >
                  <option value="invitation_only">Invitation Only</option>
                  <option value="parent_context_auto_join">Auto Join</option>
                  <option value="parent_context_request">Request to Join</option>
                </select>
              </div>
              <div className="flex justify-end space-x-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowGroupModal(false)}
                  className="px-4 py-2 text-sm text-text-secondary border border-border-strong rounded-md hover:bg-surface-1"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={saving}
                  className="bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm font-medium disabled:opacity-50"
                >
                  {saving ? 'Saving...' : editingGroup ? 'Update' : 'Create'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </Layout>
  );
};

export default GroupsPage;
