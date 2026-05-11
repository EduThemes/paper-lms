import React, { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import { Megaphone, AlertTriangle, Clock, CheckCircle, Eye, Users, Plus, Edit2, Trash2, ChevronDown, ChevronUp } from 'lucide-react';
import { api } from '../services/api';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import { useAuth } from '../contexts/AuthContext';
import RichContentEditorV2 from '../components/rce/RichContentEditorV2';
import useCrossCourseCheck from '../hooks/useCrossCourseCheck';
import CrossCourseWarningDialog from '../components/CrossCourseWarningDialog';
import { Skeleton } from '@/components/ui/skeleton';

const AnnouncementsPage = () => {
  const { courseId } = useParams();
  const { user } = useAuth();
  const [announcements, setAnnouncements] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [showForm, setShowForm] = useState(false);
  const [creating, setCreating] = useState(false);
  const [expandedReceipts, setExpandedReceipts] = useState({});
  const [receiptData, setReceiptData] = useState({});
  const [editingId, setEditingId] = useState(null);

  const [formData, setFormData] = useState({
    title: '',
    message: '',
    priority: 'normal',
    target_audience: 'all',
    require_acknowledgement: false,
    allow_comments: false,
    workflow_state: 'published',
    delayed_post_at: '',
    schedule: false,
  });

  const [userNames, setUserNames] = useState({});
  const [isInstructor, setIsInstructor] = useState(false);
  const { issues: crossCourseIssues, checkAndSave, dismiss: dismissCrossCourse, confirm: confirmCrossCourse } = useCrossCourseCheck(courseId);

  const fetchAnnouncements = async () => {
    try {
      const result = await api.request(`/courses/${courseId}/announcements`);
      setAnnouncements(result.data || []);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchAnnouncements();
    // Detect teacher role via enrollment (not user.role which is global)
    if (courseId && user) {
      api.getEnrollments(courseId, 1, 200)
        .then((result) => {
          const enrollments = result.data || [];
          const myEnrollment = enrollments.find(e =>
            e.user_id === user.id || e.user?.id === user.id
          );
          const teacherRole =
            user.role === 'admin' ||
            myEnrollment?.type === 'TeacherEnrollment' ||
            myEnrollment?.type === 'TaEnrollment' ||
            myEnrollment?.role === 'TeacherEnrollment' ||
            myEnrollment?.role === 'TaEnrollment';
          setIsInstructor(teacherRole);
          // Build user name map for read receipts
          if (teacherRole) {
            const names = {};
            for (const e of (result.data || [])) {
              const uid = e.user_id || e.user?.id;
              if (uid) {
                names[uid] = e.user?.name || e.user?.display_name || `User #${uid}`;
              }
            }
            setUserNames(names);
          }
        })
        .catch(() => {});
    }
  }, [courseId]);

  const resetForm = () => {
    setFormData({
      title: '',
      message: '',
      priority: 'normal',
      target_audience: 'all',
      require_acknowledgement: false,
      allow_comments: false,
      workflow_state: 'published',
      delayed_post_at: '',
      schedule: false,
    });
    setEditingId(null);
  };

  const doCreate = async () => {
    setCreating(true);
    try {
      const payload = {
        title: formData.title,
        message: formData.message,
        priority: formData.priority,
        target_audience: formData.target_audience,
        require_acknowledgement: formData.require_acknowledgement,
        allow_comments: formData.allow_comments,
        workflow_state: formData.schedule ? 'scheduled' : 'published',
      };

      if (formData.schedule && formData.delayed_post_at) {
        payload.delayed_post_at = new Date(formData.delayed_post_at).toISOString();
      }

      if (editingId) {
        await api.request(`/announcements/${editingId}`, {
          method: 'PUT',
          body: JSON.stringify(payload),
        });
      } else {
        await api.request(`/courses/${courseId}/announcements`, {
          method: 'POST',
          body: JSON.stringify(payload),
        });
      }

      resetForm();
      setShowForm(false);
      setLoading(true);
      await fetchAnnouncements();
    } catch (err) {
      setError(err.message);
    } finally {
      setCreating(false);
    }
  };

  const handleCreate = (e) => {
    e.preventDefault();
    checkAndSave(formData.message, doCreate);
  };

  const handleDelete = async (id) => {
    if (!window.confirm('Are you sure you want to delete this announcement?')) return;
    try {
      await api.request(`/announcements/${id}`, { method: 'DELETE' });
      setLoading(true);
      await fetchAnnouncements();
    } catch (err) {
      setError(err.message);
    }
  };

  const handleEdit = (announcement) => {
    setFormData({
      title: announcement.title,
      message: announcement.message,
      priority: announcement.priority,
      target_audience: announcement.target_audience,
      require_acknowledgement: announcement.require_acknowledgement,
      allow_comments: announcement.allow_comments,
      workflow_state: announcement.workflow_state,
      delayed_post_at: announcement.delayed_post_at || '',
      schedule: announcement.workflow_state === 'scheduled',
    });
    setEditingId(announcement.id);
    setShowForm(true);
  };

  const handleMarkRead = async (id) => {
    try {
      await api.request(`/announcements/${id}/read`, { method: 'POST' });
      setAnnouncements((prev) =>
        prev.map((a) => (a.id === id ? { ...a, is_read: true } : a))
      );
    } catch (err) {
      setError(err.message);
    }
  };

  const handleAcknowledge = async (id) => {
    try {
      await api.request(`/announcements/${id}/acknowledge`, { method: 'POST' });
      setAnnouncements((prev) =>
        prev.map((a) => (a.id === id ? { ...a, is_acknowledged: true, is_read: true } : a))
      );
    } catch (err) {
      setError(err.message);
    }
  };

  const toggleReceipts = async (id) => {
    if (expandedReceipts[id]) {
      setExpandedReceipts((prev) => ({ ...prev, [id]: false }));
      return;
    }

    try {
      const result = await api.request(`/announcements/${id}/read_receipts`);
      setReceiptData((prev) => ({ ...prev, [id]: result.data }));
      setExpandedReceipts((prev) => ({ ...prev, [id]: true }));
    } catch (err) {
      setError(err.message);
    }
  };

  const formatDate = (dateStr) => {
    if (!dateStr) return '';
    return new Date(dateStr).toLocaleDateString(undefined, {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  const getPriorityStyles = (priority) => {
    if (priority === 'urgent') {
      return 'border-l-4 border-accent-danger bg-accent-danger/10';
    }
    return 'border-l-4 border-brand-500 bg-surface-0';
  };

  if (loading) {
    return (
      <Layout>
        <CourseNav />
        <div className="space-y-3 p-6" role="status" aria-label="Loading announcements">
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
        <div className="text-center py-12" role="alert">
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
          <div className="flex items-center space-x-2">
            <Megaphone className="w-6 h-6 text-text-secondary" aria-hidden="true" />
            <h2 className="text-2xl font-bold text-text-primary">Announcements</h2>
          </div>
          {isInstructor && (
            <button
              onClick={() => {
                if (showForm) {
                  resetForm();
                }
                setShowForm(!showForm);
              }}
              className="inline-flex items-center space-x-2 bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm font-medium"
              aria-expanded={showForm}
              aria-controls="announcement-form"
            >
              <Plus className="w-4 h-4" aria-hidden="true" />
              <span>{showForm ? 'Cancel' : 'New Announcement'}</span>
            </button>
          )}
        </div>
      </div>

      {showForm && (
        <div id="announcement-form" className="bg-surface-0 rounded-lg shadow p-6 mb-6" role="form" aria-label={editingId ? 'Edit Announcement' : 'Create Announcement'}>
          <h3 className="font-semibold mb-4">{editingId ? 'Edit Announcement' : 'Create Announcement'}</h3>
          <form onSubmit={handleCreate} className="space-y-4">
            <div>
              <label htmlFor="ann-title" className="block text-sm font-medium text-text-secondary mb-1">Title</label>
              <input
                id="ann-title"
                type="text"
                value={formData.title}
                onChange={(e) => setFormData({ ...formData, title: e.target.value })}
                className="w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                required
                aria-required="true"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">Message</label>
              <RichContentEditorV2
                value={formData.message}
                onChange={(html) => setFormData((prev) => ({ ...prev, message: html }))}
                placeholder="Announcement message..."
                courseId={courseId}
                autoSaveKey={editingId ? `announcement-${editingId}` : 'new-announcement'}
              />
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <label htmlFor="ann-priority" className="block text-sm font-medium text-text-secondary mb-1">Priority</label>
                <select
                  id="ann-priority"
                  value={formData.priority}
                  onChange={(e) => setFormData({ ...formData, priority: e.target.value })}
                  className="w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                >
                  <option value="normal">Normal</option>
                  <option value="urgent">Urgent</option>
                </select>
              </div>
              <div>
                <label htmlFor="ann-audience" className="block text-sm font-medium text-text-secondary mb-1">Audience</label>
                <select
                  id="ann-audience"
                  value={formData.target_audience}
                  onChange={(e) => setFormData({ ...formData, target_audience: e.target.value })}
                  className="w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                >
                  <option value="all">All</option>
                  <option value="students">Students Only</option>
                  <option value="teachers">Teachers Only</option>
                  <option value="observers">Observers Only</option>
                </select>
              </div>
            </div>

            <div className="flex flex-wrap items-center gap-6">
              <div className="flex items-center space-x-2">
                <input
                  type="checkbox"
                  id="ann-schedule"
                  checked={formData.schedule}
                  onChange={(e) => setFormData({ ...formData, schedule: e.target.checked })}
                  className="rounded border-border-strong"
                />
                <label htmlFor="ann-schedule" className="text-sm text-text-secondary">Schedule for later</label>
              </div>
              <div className="flex items-center space-x-2">
                <input
                  type="checkbox"
                  id="ann-require-ack"
                  checked={formData.require_acknowledgement}
                  onChange={(e) => setFormData({ ...formData, require_acknowledgement: e.target.checked })}
                  className="rounded border-border-strong"
                />
                <label htmlFor="ann-require-ack" className="text-sm text-text-secondary">Require Acknowledgement</label>
              </div>
              <div className="flex items-center space-x-2">
                <input
                  type="checkbox"
                  id="ann-allow-comments"
                  checked={formData.allow_comments}
                  onChange={(e) => setFormData({ ...formData, allow_comments: e.target.checked })}
                  className="rounded border-border-strong"
                />
                <label htmlFor="ann-allow-comments" className="text-sm text-text-secondary">Allow Comments</label>
              </div>
            </div>

            {formData.schedule && (
              <div>
                <label htmlFor="ann-delayed-post" className="block text-sm font-medium text-text-secondary mb-1">
                  <Clock className="w-4 h-4 inline mr-1" aria-hidden="true" />
                  Scheduled Post Date
                </label>
                <input
                  id="ann-delayed-post"
                  type="datetime-local"
                  value={formData.delayed_post_at}
                  onChange={(e) => setFormData({ ...formData, delayed_post_at: e.target.value })}
                  className="border border-border-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                  required={formData.schedule}
                  aria-required={formData.schedule}
                />
              </div>
            )}

            <div className="flex justify-end space-x-3">
              <button
                type="button"
                onClick={() => {
                  resetForm();
                  setShowForm(false);
                }}
                className="px-4 py-2 border border-border-strong rounded-md text-sm font-medium text-text-secondary hover:bg-surface-1"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={creating}
                className="bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm font-medium disabled:opacity-50"
              >
                {creating ? 'Saving...' : editingId ? 'Update Announcement' : 'Post Announcement'}
              </button>
            </div>
          </form>
        </div>
      )}

      {/* Urgent announcements banner */}
      {announcements.filter((a) => a.priority === 'urgent' && !a.is_read).length > 0 && (
        <div className="bg-accent-danger text-white rounded-lg p-4 mb-6 flex items-center space-x-3" role="alert" aria-live="assertive">
          <AlertTriangle className="w-5 h-5 flex-shrink-0" aria-hidden="true" />
          <div>
            <strong>Urgent Announcements:</strong>{' '}
            You have {announcements.filter((a) => a.priority === 'urgent' && !a.is_read).length} unread urgent announcement(s).
          </div>
        </div>
      )}

      {/* Announcements list */}
      <div className="space-y-4" role="feed" aria-label="Announcements list">
        {announcements.length === 0 ? (
          <div className="bg-surface-0 rounded-lg shadow p-6 text-center text-text-tertiary">
            No announcements yet.
          </div>
        ) : (
          announcements.map((announcement) => (
            <article
              key={announcement.id}
              className={`rounded-lg shadow ${getPriorityStyles(announcement.priority)} relative`}
              aria-label={`Announcement: ${announcement.title}`}
            >
              {/* Unread badge */}
              {!announcement.is_read && (
                <span className="absolute top-3 right-3 inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-brand-100 text-brand-800" aria-label="Unread">
                  New
                </span>
              )}

              <div className="p-5">
                <div className="flex items-start justify-between">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center space-x-2 mb-1">
                      {announcement.priority === 'urgent' && (
                        <AlertTriangle className="w-4 h-4 text-accent-danger flex-shrink-0" aria-label="Urgent" />
                      )}
                      <h3 className="text-lg font-semibold text-text-primary truncate">{announcement.title}</h3>
                      {announcement.workflow_state === 'scheduled' && (
                        <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-accent-warning/20 text-accent-warning">
                          <Clock className="w-3 h-3 mr-1" aria-hidden="true" />
                          Scheduled
                        </span>
                      )}
                      {announcement.require_acknowledgement && (
                        <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-purple-100 text-purple-800">
                          Ack Required
                        </span>
                      )}
                    </div>

                    <p className="text-text-secondary text-sm mt-2 whitespace-pre-wrap">{announcement.message}</p>

                    <div className="flex items-center space-x-4 mt-3 text-xs text-text-disabled">
                      <span>
                        {announcement.posted_at
                          ? `Posted ${formatDate(announcement.posted_at)}`
                          : `Created ${formatDate(announcement.created_at)}`}
                      </span>
                      <span className="capitalize">Audience: {announcement.target_audience}</span>
                      {announcement.delayed_post_at && announcement.workflow_state === 'scheduled' && (
                        <span>Scheduled for {formatDate(announcement.delayed_post_at)}</span>
                      )}
                    </div>
                  </div>
                </div>

                {/* Action buttons */}
                <div className="flex items-center justify-between mt-4 pt-3 border-t border-border-default">
                  <div className="flex items-center space-x-3">
                    {!announcement.is_read && (
                      <button
                        onClick={() => handleMarkRead(announcement.id)}
                        className="inline-flex items-center space-x-1 text-sm text-brand-600 hover:text-brand-800"
                        aria-label={`Mark "${announcement.title}" as read`}
                      >
                        <Eye className="w-4 h-4" aria-hidden="true" />
                        <span>Mark as Read</span>
                      </button>
                    )}
                    {announcement.is_read && (
                      <span className="inline-flex items-center space-x-1 text-sm text-accent-success">
                        <CheckCircle className="w-4 h-4" aria-hidden="true" />
                        <span>Read</span>
                      </span>
                    )}

                    {announcement.require_acknowledgement && !announcement.is_acknowledged && (
                      <button
                        onClick={() => handleAcknowledge(announcement.id)}
                        className="inline-flex items-center space-x-1 text-sm bg-purple-600 text-white px-3 py-1 rounded-md hover:bg-purple-700"
                        aria-label={`Acknowledge "${announcement.title}"`}
                      >
                        <CheckCircle className="w-4 h-4" aria-hidden="true" />
                        <span>Acknowledge</span>
                      </button>
                    )}
                    {announcement.require_acknowledgement && announcement.is_acknowledged && (
                      <span className="inline-flex items-center space-x-1 text-sm text-purple-600">
                        <CheckCircle className="w-4 h-4" aria-hidden="true" />
                        <span>Acknowledged</span>
                      </span>
                    )}
                  </div>

                  <div className="flex items-center space-x-2">
                    {isInstructor && (
                      <>
                        <button
                          onClick={() => toggleReceipts(announcement.id)}
                          className="inline-flex items-center space-x-1 text-sm text-text-tertiary hover:text-text-secondary"
                          aria-expanded={expandedReceipts[announcement.id] || false}
                          aria-controls={`receipts-${announcement.id}`}
                          aria-label={`View read receipts for "${announcement.title}"`}
                        >
                          <Users className="w-4 h-4" aria-hidden="true" />
                          <span>Read Receipts</span>
                          {expandedReceipts[announcement.id] ? (
                            <ChevronUp className="w-4 h-4" aria-hidden="true" />
                          ) : (
                            <ChevronDown className="w-4 h-4" aria-hidden="true" />
                          )}
                        </button>
                        <button
                          onClick={() => handleEdit(announcement)}
                          className="text-text-disabled hover:text-text-secondary"
                          aria-label={`Edit "${announcement.title}"`}
                        >
                          <Edit2 className="w-4 h-4" aria-hidden="true" />
                        </button>
                        <button
                          onClick={() => handleDelete(announcement.id)}
                          className="text-text-disabled hover:text-accent-danger"
                          aria-label={`Delete "${announcement.title}"`}
                        >
                          <Trash2 className="w-4 h-4" aria-hidden="true" />
                        </button>
                      </>
                    )}
                  </div>
                </div>

                {/* Read receipts panel (instructor only) */}
                {isInstructor && expandedReceipts[announcement.id] && receiptData[announcement.id] && (
                  <div
                    id={`receipts-${announcement.id}`}
                    className="mt-4 pt-3 border-t border-border-default"
                    role="region"
                    aria-label={`Read receipts for "${announcement.title}"`}
                  >
                    <div className="flex items-center space-x-4 mb-3">
                      <div className="text-sm font-medium text-text-secondary">
                        Read: {receiptData[announcement.id].stats?.read_count || 0} / {receiptData[announcement.id].stats?.total_audience || 0}
                      </div>
                      {announcement.require_acknowledgement && (
                        <div className="text-sm font-medium text-purple-700">
                          Acknowledged: {receiptData[announcement.id].stats?.acknowledged_count || 0} / {receiptData[announcement.id].stats?.total_audience || 0}
                        </div>
                      )}
                    </div>

                    {receiptData[announcement.id].receipts && receiptData[announcement.id].receipts.length > 0 ? (
                      <div className="bg-surface-1 rounded-md overflow-hidden">
                        <table className="w-full text-sm" aria-label="Read receipt details">
                          <thead>
                            <tr className="border-b border-border-default">
                              <th className="text-left px-3 py-2 font-medium text-text-secondary" scope="col">Student</th>
                              <th className="text-left px-3 py-2 font-medium text-text-secondary" scope="col">Read At</th>
                              {announcement.require_acknowledgement && (
                                <th className="text-left px-3 py-2 font-medium text-text-secondary" scope="col">Acknowledged</th>
                              )}
                            </tr>
                          </thead>
                          <tbody className="divide-y divide-border-default">
                            {receiptData[announcement.id].receipts.map((receipt) => (
                              <tr key={receipt.id}>
                                <td className="px-3 py-2 text-text-primary">{userNames[receipt.user_id] || `User #${receipt.user_id}`}</td>
                                <td className="px-3 py-2 text-text-tertiary">{formatDate(receipt.read_at)}</td>
                                {announcement.require_acknowledgement && (
                                  <td className="px-3 py-2">
                                    {receipt.acknowledged ? (
                                      <span className="inline-flex items-center text-accent-success">
                                        <CheckCircle className="w-3.5 h-3.5 mr-1" aria-hidden="true" />
                                        {formatDate(receipt.acknowledged_at)}
                                      </span>
                                    ) : (
                                      <span className="text-text-disabled">Not yet</span>
                                    )}
                                  </td>
                                )}
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </div>
                    ) : (
                      <p className="text-sm text-text-tertiary">No one has read this announcement yet.</p>
                    )}
                  </div>
                )}
              </div>
            </article>
          ))
        )}
      </div>
      <CrossCourseWarningDialog issues={crossCourseIssues} onGoBack={dismissCrossCourse} onSaveAnyway={confirmCrossCourse} />
    </Layout>
  );
};

export default AnnouncementsPage;
