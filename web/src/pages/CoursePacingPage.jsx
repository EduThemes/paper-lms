import React, { useState, useEffect, useCallback } from 'react';
import { useParams, Link, Navigate } from 'react-router-dom';
import { Clock, Calendar, Play, Save, Settings, ChevronDown, ChevronRight, Users, User } from 'lucide-react';
import { api } from '../services/api';
import useIsTeacher from '../hooks/useIsTeacher';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';

const CoursePacingPage = () => {
  const { courseId } = useParams();
  const isTeacher = useIsTeacher(courseId);
  const [pace, setPace] = useState(null);
  const [paces, setPaces] = useState([]);
  const [moduleItems, setModuleItems] = useState([]);
  const [timeline, setTimeline] = useState([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState(null);
  const [showSettings, setShowSettings] = useState(false);
  const [paceType, setPaceType] = useState('course'); // course, section, student
  const [sectionId, setSectionId] = useState('');
  const [studentId, setStudentId] = useState('');

  const fetchPaces = useCallback(async () => {
    try {
      setLoading(true);
      const result = await api.getCoursePaces(courseId);
      setPaces(result.data || []);

      if (result.data && result.data.length > 0) {
        const defaultPace = result.data.find(
          (p) => !p.user_id && !p.course_section_id
        ) || result.data[0];
        setPace(defaultPace);
        await fetchPaceDetails(defaultPace.id);
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [courseId]);

  const fetchPaceDetails = async (paceId) => {
    try {
      const [itemsResult, timelineResult] = await Promise.all([
        api.getCoursePaceModuleItems(courseId, paceId),
        api.getCoursePaceTimeline(courseId, paceId),
      ]);
      setModuleItems(itemsResult.data || []);
      setTimeline(timelineResult.data || []);
    } catch (err) {
      setError(err.message);
    }
  };

  useEffect(() => {
    fetchPaces();
  }, [fetchPaces]);

  const handleDurationChange = (index, value) => {
    const newItems = [...moduleItems];
    newItems[index] = { ...newItems[index], duration: Math.max(1, parseInt(value) || 1) };
    setModuleItems(newItems);
  };

  const handleSaveItems = async () => {
    if (!pace) return;
    setSaving(true);
    try {
      const items = moduleItems.map((item) => ({
        module_item_id: item.module_item_id,
        duration: item.duration,
      }));
      await api.updateCoursePaceModuleItems(courseId, pace.id, items);
      const timelineResult = await api.getCoursePaceTimeline(courseId, pace.id);
      setTimeline(timelineResult.data || []);
    } catch (err) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  const handleUpdateSettings = async () => {
    if (!pace) return;
    setSaving(true);
    try {
      const updated = await api.updateCoursePace(courseId, pace.id, {
        exclude_weekends: pace.exclude_weekends,
        hard_end_dates: pace.hard_end_dates,
        end_date: pace.end_date,
      });
      setPace(updated.data || updated);
      const timelineResult = await api.getCoursePaceTimeline(courseId, pace.id);
      setTimeline(timelineResult.data || []);
    } catch (err) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  const handlePublish = async () => {
    if (!pace) return;
    setSaving(true);
    try {
      const published = await api.publishCoursePace(courseId, pace.id);
      setPace(published.data || published);
    } catch (err) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  const handleCreatePace = async () => {
    setSaving(true);
    try {
      const body = {};
      if (paceType === 'section' && sectionId) {
        body.course_section_id = parseInt(sectionId);
      } else if (paceType === 'student' && studentId) {
        body.user_id = parseInt(studentId);
      }
      const created = await api.createCoursePace(courseId, body);
      const newPace = created.data || created;
      setPace(newPace);
      await fetchPaces();
      await fetchPaceDetails(newPace.id);
    } catch (err) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  const handleSelectPace = async (selectedPace) => {
    setPace(selectedPace);
    setLoading(true);
    try {
      await fetchPaceDetails(selectedPace.id);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const getTimelineForItem = (moduleItemId) => {
    return timeline.find((t) => t.module_item_id === moduleItemId);
  };

  const formatDate = (dateStr) => {
    if (!dateStr) return '';
    return new Date(dateStr).toLocaleDateString('en-US', {
      weekday: 'short',
      month: 'short',
      day: 'numeric',
      year: 'numeric',
    });
  };

  const getPaceLabel = (p) => {
    if (p.user_id) return `Student #${p.user_id}`;
    if (p.course_section_id) return `Section #${p.course_section_id}`;
    return 'Course Default';
  };

  if (isTeacher === false) return <Navigate to={`/courses/${courseId}`} replace />;
  if (isTeacher === null) return <Layout><div className="flex items-center justify-center py-12 gap-2 text-text-tertiary">
  <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" /></svg>
  Loading...
</div></Layout>;

  if (loading && !pace) {
    return (
      <Layout>
        <div className="flex items-center justify-center py-12 gap-2 text-text-tertiary">
  <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" /></svg>
  Loading course pacing...
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
          <h2 className="text-2xl font-bold flex items-center space-x-2">
            <Clock className="w-6 h-6 text-indigo-600" />
            <span>Course Pacing</span>
          </h2>
          <div className="flex items-center space-x-2">
            <button
              onClick={() => setShowSettings(!showSettings)}
              className="flex items-center space-x-1 bg-surface-2 text-text-secondary px-3 py-2 rounded-md hover:bg-border-default text-sm"
            >
              <Settings className="w-4 h-4" />
              <span>Settings</span>
            </button>
            {pace && pace.workflow_state !== 'active' && (
              <button
                onClick={handlePublish}
                disabled={saving}
                className="flex items-center space-x-1 bg-accent-success text-white px-4 py-2 rounded-md hover:bg-accent-success/90 text-sm disabled:opacity-50"
              >
                <Play className="w-4 h-4" />
                <span>{saving ? 'Publishing...' : 'Publish'}</span>
              </button>
            )}
          </div>
        </div>
      </div>

      {error && (
        <div className="bg-accent-danger/10 text-accent-danger p-3 rounded mb-4">
          {error}
          <button onClick={() => setError(null)} className="ml-2 underline text-sm">
            Dismiss
          </button>
        </div>
      )}

      {/* Pace Type Selector */}
      <div className="bg-surface-0 rounded-lg shadow p-4 mb-4">
        <h3 className="text-sm font-semibold text-text-secondary mb-3">Pace Type</h3>
        <div className="flex items-center space-x-4 mb-3">
          <label className="flex items-center space-x-2 cursor-pointer">
            <input
              type="radio"
              name="paceType"
              value="course"
              checked={paceType === 'course'}
              onChange={() => setPaceType('course')}
              className="text-indigo-600"
            />
            <Clock className="w-4 h-4 text-text-tertiary" />
            <span className="text-sm">Course Default</span>
          </label>
          <label className="flex items-center space-x-2 cursor-pointer">
            <input
              type="radio"
              name="paceType"
              value="section"
              checked={paceType === 'section'}
              onChange={() => setPaceType('section')}
              className="text-indigo-600"
            />
            <Users className="w-4 h-4 text-text-tertiary" />
            <span className="text-sm">Section</span>
          </label>
          <label className="flex items-center space-x-2 cursor-pointer">
            <input
              type="radio"
              name="paceType"
              value="student"
              checked={paceType === 'student'}
              onChange={() => setPaceType('student')}
              className="text-indigo-600"
            />
            <User className="w-4 h-4 text-text-tertiary" />
            <span className="text-sm">Student</span>
          </label>
        </div>

        {paceType === 'section' && (
          <div className="flex items-center space-x-2 mb-2">
            <input
              type="number"
              placeholder="Section ID"
              value={sectionId}
              onChange={(e) => setSectionId(e.target.value)}
              className="border border-border-strong rounded px-3 py-1.5 text-sm w-32"
            />
          </div>
        )}

        {paceType === 'student' && (
          <div className="flex items-center space-x-2 mb-2">
            <input
              type="number"
              placeholder="Student ID"
              value={studentId}
              onChange={(e) => setStudentId(e.target.value)}
              className="border border-border-strong rounded px-3 py-1.5 text-sm w-32"
            />
          </div>
        )}

        {/* Existing paces list */}
        {paces.length > 0 && (
          <div className="mt-3 border-t pt-3">
            <p className="text-xs text-text-tertiary mb-2">Existing paces:</p>
            <div className="flex flex-wrap gap-2">
              {paces.map((p) => (
                <button
                  key={p.id}
                  onClick={() => handleSelectPace(p)}
                  className={`px-3 py-1 rounded text-xs border ${
                    pace && pace.id === p.id
                      ? 'bg-indigo-50 border-indigo-300 text-indigo-700'
                      : 'bg-surface-1 border-border-default text-text-secondary hover:bg-surface-2'
                  }`}
                >
                  {getPaceLabel(p)}
                  {p.workflow_state === 'active' && (
                    <span className="ml-1 inline-block w-2 h-2 bg-green-400 rounded-full" />
                  )}
                </button>
              ))}
            </div>
          </div>
        )}

        {!pace && (
          <button
            onClick={handleCreatePace}
            disabled={saving}
            className="mt-3 bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm disabled:opacity-50"
          >
            {saving ? 'Creating...' : 'Create Pace'}
          </button>
        )}
      </div>

      {/* Settings Panel */}
      {showSettings && pace && (
        <div className="bg-surface-0 rounded-lg shadow p-4 mb-4">
          <h3 className="text-sm font-semibold text-text-secondary mb-3">Pacing Settings</h3>
          <div className="space-y-3">
            <label className="flex items-center space-x-3">
              <input
                type="checkbox"
                checked={pace.exclude_weekends}
                onChange={(e) => setPace({ ...pace, exclude_weekends: e.target.checked })}
                className="rounded text-indigo-600"
              />
              <span className="text-sm text-text-secondary">Skip weekends</span>
            </label>
            <label className="flex items-center space-x-3">
              <input
                type="checkbox"
                checked={pace.hard_end_dates}
                onChange={(e) => setPace({ ...pace, hard_end_dates: e.target.checked })}
                className="rounded text-indigo-600"
              />
              <span className="text-sm text-text-secondary">Require completion by end date</span>
            </label>
            <div>
              <label className="block text-sm text-text-secondary mb-1">End date</label>
              <input
                type="date"
                value={pace.end_date ? pace.end_date.slice(0, 10) : ''}
                onChange={(e) => setPace({ ...pace, end_date: e.target.value || null })}
                className="border border-border-strong rounded px-3 py-1.5 text-sm"
              />
            </div>
            <button
              onClick={handleUpdateSettings}
              disabled={saving}
              className="bg-indigo-600 text-white px-4 py-2 rounded-md hover:bg-indigo-700 text-sm disabled:opacity-50"
            >
              {saving ? 'Saving...' : 'Save Settings'}
            </button>
          </div>
        </div>
      )}

      {/* Pace Status */}
      {pace && (
        <div className="bg-surface-0 rounded-lg shadow p-4 mb-4">
          <div className="flex items-center justify-between">
            <div>
              <h3 className="font-medium">{getPaceLabel(pace)}</h3>
              <p className="text-sm text-text-tertiary">
                Status:{' '}
                <span
                  className={`font-medium ${
                    pace.workflow_state === 'active' ? 'text-accent-success' : 'text-accent-warning'
                  }`}
                >
                  {pace.workflow_state}
                </span>
                {pace.published_at && (
                  <span className="ml-2 text-text-disabled">
                    Published {formatDate(pace.published_at)}
                  </span>
                )}
              </p>
            </div>
            <div className="text-sm text-text-tertiary">
              {pace.exclude_weekends && <span className="mr-3">Weekends excluded</span>}
              {pace.hard_end_dates && <span>Hard end dates</span>}
            </div>
          </div>
        </div>
      )}

      {/* Timeline / Module Items */}
      {pace && (
        <div className="bg-surface-0 rounded-lg shadow">
          <div className="flex items-center justify-between px-4 py-3 border-b">
            <h3 className="font-semibold text-text-secondary flex items-center space-x-2">
              <Calendar className="w-5 h-5 text-indigo-500" />
              <span>Module Items Timeline</span>
            </h3>
            <button
              onClick={handleSaveItems}
              disabled={saving}
              className="flex items-center space-x-1 bg-brand-600 text-white px-3 py-1.5 rounded-md hover:bg-brand-700 text-sm disabled:opacity-50"
            >
              <Save className="w-4 h-4" />
              <span>{saving ? 'Saving...' : 'Save Durations'}</span>
            </button>
          </div>

          {moduleItems.length === 0 ? (
            <div className="p-8 text-center text-text-tertiary">
              No module items configured for this pace. Add module items to your course first, then configure their pacing durations here.
            </div>
          ) : (
            <div className="divide-y">
              {moduleItems.map((item, index) => {
                const timelineEntry = getTimelineForItem(item.module_item_id);
                return (
                  <div
                    key={item.id || item.module_item_id}
                    className="flex items-center justify-between px-4 py-3 hover:bg-surface-1"
                  >
                    <div className="flex items-center space-x-3 flex-1">
                      <div className="w-8 h-8 bg-indigo-100 rounded-full flex items-center justify-center text-indigo-600 text-sm font-medium">
                        {index + 1}
                      </div>
                      <div>
                        <p className="text-sm font-medium text-text-primary">
                          Module Item #{item.module_item_id}
                        </p>
                        {timelineEntry && (
                          <p className="text-xs text-text-tertiary">
                            Due: {formatDate(timelineEntry.projected_date)}
                          </p>
                        )}
                      </div>
                    </div>
                    <div className="flex items-center space-x-2">
                      <label className="text-xs text-text-tertiary">Days:</label>
                      <input
                        type="number"
                        min="1"
                        value={item.duration}
                        onChange={(e) => handleDurationChange(index, e.target.value)}
                        className="w-16 border border-border-strong rounded px-2 py-1 text-sm text-center"
                      />
                    </div>
                    {timelineEntry && (
                      <div className="ml-4 w-32 bg-surface-2 rounded-full h-2 overflow-hidden">
                        <div
                          className="bg-indigo-500 h-2 rounded-full"
                          style={{
                            width: `${Math.min(100, (item.duration / Math.max(...moduleItems.map((i) => i.duration))) * 100)}%`,
                          }}
                        />
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          )}

          {/* Timeline Summary */}
          {timeline.length > 0 && (
            <div className="border-t px-4 py-3 bg-surface-1">
              <div className="flex items-center justify-between text-sm text-text-secondary">
                <span>
                  Total items: <strong>{timeline.length}</strong>
                </span>
                <span>
                  Total days:{' '}
                  <strong>{moduleItems.reduce((sum, i) => sum + i.duration, 0)}</strong>
                </span>
                {timeline.length > 0 && (
                  <span>
                    Projected end:{' '}
                    <strong>{formatDate(timeline[timeline.length - 1].projected_date)}</strong>
                  </span>
                )}
              </div>
            </div>
          )}
        </div>
      )}
    </Layout>
  );
};

export default CoursePacingPage;
