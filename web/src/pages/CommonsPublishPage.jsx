import React, { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Library } from 'lucide-react';
import { api } from '../services/api';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';

const SUBJECTS = ['', 'Math', 'ELA', 'Science', 'Social Studies', 'Art', 'Music', 'PE', 'World Languages'];
const GRADE_LEVELS = ['', 'K-2', '3-5', '6-8', '9-12'];
const RESOURCE_TYPES = [
  { value: 'course', label: 'The whole course (all assignments, pages, quizzes, modules, discussions)' },
  { value: 'assignment', label: 'A single assignment' },
  { value: 'page', label: 'A single page' },
  { value: 'quiz', label: 'A single quiz' },
  { value: 'module', label: 'A single module' },
  { value: 'discussion_topic', label: 'A single discussion' },
];

/**
 * CommonsPublishPage — form to publish course content to the Commons.
 * Reachable from a course context. For non-course resource types we
 * fetch the matching list (assignments / pages / etc.) so the teacher
 * can pick the specific resource to publish.
 */
const CommonsPublishPage = () => {
  const { courseId } = useParams();
  const navigate = useNavigate();
  const [resourceType, setResourceType] = useState('course');
  const [resourceId, setResourceId] = useState('');
  const [resourceOptions, setResourceOptions] = useState([]);
  const [loadingOptions, setLoadingOptions] = useState(false);
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [subject, setSubject] = useState('');
  const [gradeLevel, setGradeLevel] = useState('');
  const [tagsRaw, setTagsRaw] = useState('');
  const [thumbnailURL, setThumbnailURL] = useState('');
  const [visibility, setVisibility] = useState('account');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState(null);
  const [success, setSuccess] = useState(false);

  // Load picker options when a non-course resource type is selected.
  useEffect(() => {
    let alive = true;
    const loadOptions = async () => {
      if (resourceType === 'course') {
        setResourceOptions([]);
        setResourceId('');
        return;
      }
      setLoadingOptions(true);
      try {
        let result;
        switch (resourceType) {
          case 'assignment':
            result = await api.getAssignments?.(courseId);
            break;
          case 'page':
            result = await api.getPages?.(courseId);
            break;
          case 'quiz':
            result = await api.getQuizzes?.(courseId);
            break;
          case 'module':
            result = await api.getModules?.(courseId);
            break;
          case 'discussion_topic':
            result = await api.getDiscussionTopics?.(courseId);
            break;
          default:
            result = { data: [] };
        }
        const list = result?.data || [];
        if (alive) {
          setResourceOptions(
            list.map((item) => ({
              id: item.id,
              label: item.title || item.name || `#${item.id}`,
            })),
          );
        }
      } catch {
        if (alive) setResourceOptions([]);
      } finally {
        if (alive) setLoadingOptions(false);
      }
    };
    loadOptions();
    return () => {
      alive = false;
    };
  }, [resourceType, courseId]);

  const handleSubmit = async (e) => {
    e.preventDefault();
    setError(null);
    setSuccess(false);
    if (!title.trim()) {
      setError('Title is required.');
      return;
    }
    if (resourceType !== 'course' && !resourceId) {
      setError('Please pick a resource to publish.');
      return;
    }
    setSubmitting(true);
    try {
      const tags = tagsRaw
        .split(',')
        .map((t) => t.trim())
        .filter(Boolean);
      await api.publishCommons(courseId, {
        resource_type: resourceType,
        resource_id: resourceType === 'course' ? 0 : Number(resourceId),
        title: title.trim(),
        description: description.trim(),
        subject,
        grade_level: gradeLevel,
        tags,
        thumbnail_url: thumbnailURL.trim(),
        visibility,
      });
      setSuccess(true);
      setTimeout(() => navigate('/commons'), 800);
    } catch (err) {
      setError(err.message);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Layout>
      <CourseNav courseId={courseId} />
      <div className="p-6 max-w-3xl mx-auto">
        <div className="flex items-center gap-2 mb-1">
          <Library className="w-5 h-5 text-indigo-600" />
          <h1 className="text-2xl font-bold text-slate-900">Publish to Commons</h1>
        </div>
        <p className="text-sm text-slate-500 mb-6">
          Share course content with other teachers in your district.
        </p>

        <form onSubmit={handleSubmit} className="space-y-5 bg-surface-0 border border-slate-200 rounded-lg p-6">
          <div>
            <label className="block text-sm font-medium text-slate-700 mb-1">What do you want to publish?</label>
            <select
              value={resourceType}
              onChange={(e) => setResourceType(e.target.value)}
              className="w-full px-3 py-2 border border-slate-200 rounded text-sm bg-surface-0"
            >
              {RESOURCE_TYPES.map((rt) => (
                <option key={rt.value} value={rt.value}>{rt.label}</option>
              ))}
            </select>
          </div>

          {resourceType !== 'course' && (
            <div>
              <label className="block text-sm font-medium text-slate-700 mb-1">Pick the resource</label>
              <select
                value={resourceId}
                onChange={(e) => setResourceId(e.target.value)}
                className="w-full px-3 py-2 border border-slate-200 rounded text-sm bg-surface-0"
                disabled={loadingOptions}
              >
                <option value="">{loadingOptions ? 'Loading...' : 'Select...'}</option>
                {resourceOptions.map((opt) => (
                  <option key={opt.id} value={opt.id}>{opt.label}</option>
                ))}
              </select>
            </div>
          )}

          <div>
            <label className="block text-sm font-medium text-slate-700 mb-1">Title</label>
            <input
              type="text"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              className="w-full px-3 py-2 border border-slate-200 rounded text-sm"
              placeholder="e.g. 2nd-Grade Fractions Unit"
              required
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-slate-700 mb-1">Description</label>
            <textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={4}
              className="w-full px-3 py-2 border border-slate-200 rounded text-sm"
              placeholder="What's in this content? Who is it for?"
            />
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-slate-700 mb-1">Subject</label>
              <select
                value={subject}
                onChange={(e) => setSubject(e.target.value)}
                className="w-full px-3 py-2 border border-slate-200 rounded text-sm bg-surface-0"
              >
                {SUBJECTS.map((s) => (
                  <option key={s || 'none'} value={s}>{s || 'Choose...'}</option>
                ))}
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium text-slate-700 mb-1">Grade level</label>
              <select
                value={gradeLevel}
                onChange={(e) => setGradeLevel(e.target.value)}
                className="w-full px-3 py-2 border border-slate-200 rounded text-sm bg-surface-0"
              >
                {GRADE_LEVELS.map((g) => (
                  <option key={g || 'none'} value={g}>{g || 'Choose...'}</option>
                ))}
              </select>
            </div>
          </div>

          <div>
            <label className="block text-sm font-medium text-slate-700 mb-1">Tags (comma-separated)</label>
            <input
              type="text"
              value={tagsRaw}
              onChange={(e) => setTagsRaw(e.target.value)}
              className="w-full px-3 py-2 border border-slate-200 rounded text-sm"
              placeholder="e.g. fractions, hands-on, station"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-slate-700 mb-1">Thumbnail URL (optional)</label>
            <input
              type="url"
              value={thumbnailURL}
              onChange={(e) => setThumbnailURL(e.target.value)}
              className="w-full px-3 py-2 border border-slate-200 rounded text-sm"
              placeholder="https://..."
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-slate-700 mb-1">Visibility</label>
            <select
              value={visibility}
              onChange={(e) => setVisibility(e.target.value)}
              className="w-full px-3 py-2 border border-slate-200 rounded text-sm bg-surface-0"
            >
              <option value="account">My district only</option>
              <option value="public">Public</option>
            </select>
          </div>

          {error && (
            <div className="bg-accent-danger/10 border border-accent-danger/30 text-accent-danger text-sm p-3 rounded">
              {error}
            </div>
          )}
          {success && (
            <div className="bg-accent-success/10 border border-accent-success/30 text-accent-success text-sm p-3 rounded">
              Published! Redirecting to Commons...
            </div>
          )}

          <div className="flex justify-end gap-2">
            <button
              type="button"
              onClick={() => navigate(-1)}
              className="px-4 py-2 text-sm border border-slate-200 rounded hover:bg-slate-50"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={submitting}
              className="px-4 py-2 text-sm bg-indigo-600 text-white rounded hover:bg-indigo-700 disabled:opacity-50"
            >
              {submitting ? 'Publishing...' : 'Publish to Commons'}
            </button>
          </div>
        </form>
      </div>
    </Layout>
  );
};

export default CommonsPublishPage;
