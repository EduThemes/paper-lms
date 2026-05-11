import React, { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import { Plus, Trash2, Edit2, Award } from 'lucide-react';
import { api } from '../services/api';
import useIsTeacher from '../hooks/useIsTeacher';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import { Skeleton } from '@/components/ui/skeleton';

const RubricsPage = () => {
  const { courseId } = useParams();
  const isTeacher = useIsTeacher(courseId);
  const [rubrics, setRubrics] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [showCreate, setShowCreate] = useState(false);
  const [editingId, setEditingId] = useState(null);
  const [title, setTitle] = useState('');
  const [criteria, setCriteria] = useState([
    { id: 'c1', description: '', points: 0, ratings: [{ id: 'r1', description: 'Full Marks', points: 0 }, { id: 'r2', description: 'No Marks', points: 0 }] }
  ]);

  const fetchRubrics = async () => {
    try {
      const result = await api.getCourseRubrics(courseId, 1, 100);
      setRubrics(result.data || []);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { fetchRubrics(); }, [courseId]);

  const addCriterion = () => {
    const id = `c${Date.now()}`;
    setCriteria([...criteria, {
      id, description: '', points: 0,
      ratings: [{ id: `r${Date.now()}a`, description: 'Full Marks', points: 0 }, { id: `r${Date.now()}b`, description: 'No Marks', points: 0 }]
    }]);
  };

  const updateCriterion = (idx, field, value) => {
    const updated = [...criteria];
    updated[idx] = { ...updated[idx], [field]: value };
    setCriteria(updated);
  };

  const removeCriterion = (idx) => {
    setCriteria(criteria.filter((_, i) => i !== idx));
  };

  const resetForm = () => {
    setTitle('');
    setCriteria([{ id: 'c1', description: '', points: 0, ratings: [{ id: 'r1', description: 'Full Marks', points: 0 }, { id: 'r2', description: 'No Marks', points: 0 }] }]);
    setEditingId(null);
  };

  const handleCreate = async (e) => {
    e.preventDefault();
    try {
      const totalPoints = criteria.reduce((sum, c) => sum + (parseFloat(c.points) || 0), 0);
      const payload = {
        title,
        data: JSON.stringify(criteria),
        points_possible: totalPoints,
      };
      if (editingId) {
        await api.updateCourseRubric(courseId, editingId, payload);
      } else {
        await api.createCourseRubric(courseId, payload);
      }
      resetForm();
      setShowCreate(false);
      fetchRubrics();
    } catch (err) {
      setError(err.message);
    }
  };

  const handleEdit = (rubric) => {
    setTitle(rubric.title || '');
    // Parse rubric data — it's stored as JSONB
    let parsedCriteria;
    try {
      parsedCriteria = typeof rubric.data === 'string' ? JSON.parse(rubric.data) : rubric.data;
    } catch {
      parsedCriteria = [];
    }
    if (Array.isArray(parsedCriteria) && parsedCriteria.length > 0) {
      setCriteria(parsedCriteria);
    } else {
      setCriteria([{ id: 'c1', description: '', points: rubric.points_possible || 0, ratings: [{ id: 'r1', description: 'Full Marks', points: rubric.points_possible || 0 }, { id: 'r2', description: 'No Marks', points: 0 }] }]);
    }
    setEditingId(rubric.id);
    setShowCreate(true);
  };

  const handleDelete = async (rubricId) => {
    if (!window.confirm('Delete this rubric?')) return;
    try {
      await api.deleteCourseRubric(courseId, rubricId);
      fetchRubrics();
    } catch (err) {
      setError(err.message);
    }
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

  return (
    <Layout>
      <CourseNav />
      <div className="mb-6">
        <Link to={`/courses/${courseId}`} className="text-brand-600 hover:underline text-sm">← Back to Course</Link>
        <div className="flex items-center justify-between mt-2">
          <h2 className="text-2xl font-bold">Rubrics</h2>
          {isTeacher && (
            <button
              onClick={() => {
                if (showCreate) { resetForm(); }
                setShowCreate(!showCreate);
              }}
              className="flex items-center space-x-2 bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm"
            >
              {showCreate ? <Plus className="w-4 h-4 rotate-45" /> : <Plus className="w-4 h-4" />}
              <span>{showCreate ? 'Cancel' : 'New Rubric'}</span>
            </button>
          )}
        </div>
      </div>

      {error && <div className="bg-accent-danger/10 text-accent-danger p-3 rounded mb-4">{error}</div>}

      {showCreate && (
        <div className="bg-surface-0 rounded-lg shadow p-6 mb-6">
          <h3 className="font-semibold mb-4">{editingId ? 'Edit Rubric' : 'Create Rubric'}</h3>
          <form onSubmit={handleCreate} className="space-y-4">
            <input
              type="text"
              placeholder="Rubric title"
              value={title}
              onChange={e => setTitle(e.target.value)}
              className="w-full border border-border-strong rounded px-3 py-2"
              required
            />

            {criteria.map((c, idx) => (
              <div key={c.id} className="border border-border-default rounded p-4">
                <div className="flex items-center justify-between mb-2">
                  <span className="text-sm font-medium text-text-tertiary">Criterion {idx + 1}</span>
                  {criteria.length > 1 && (
                    <button type="button" onClick={() => removeCriterion(idx)} className="text-accent-danger hover:text-accent-danger">
                      <Trash2 className="w-4 h-4" />
                    </button>
                  )}
                </div>
                <div className="grid grid-cols-4 gap-3">
                  <input
                    type="text"
                    placeholder="Description"
                    value={c.description}
                    onChange={e => updateCriterion(idx, 'description', e.target.value)}
                    className="col-span-3 border border-border-strong rounded px-3 py-2 text-sm"
                  />
                  <input
                    type="number"
                    placeholder="Points"
                    value={c.points}
                    onChange={e => updateCriterion(idx, 'points', parseFloat(e.target.value) || 0)}
                    className="border border-border-strong rounded px-3 py-2 text-sm"
                  />
                </div>
              </div>
            ))}

            <button type="button" onClick={addCriterion} className="text-brand-600 hover:underline text-sm">
              + Add Criterion
            </button>

            <div className="flex space-x-3">
              <button type="submit" className="bg-brand-600 text-white px-4 py-2 rounded hover:bg-brand-700 text-sm">
                {editingId ? 'Update Rubric' : 'Create Rubric'}
              </button>
              <button type="button" onClick={() => { resetForm(); setShowCreate(false); }} className="text-text-tertiary hover:text-text-secondary text-sm">
                Cancel
              </button>
            </div>
          </form>
        </div>
      )}

      <div className="space-y-3">
        {rubrics.length === 0 ? (
          <div className="bg-surface-0 rounded-lg shadow p-8 text-center text-text-tertiary">No rubrics yet.</div>
        ) : (
          rubrics.map(rubric => (
            <div key={rubric.id} className="bg-surface-0 rounded-lg shadow p-4 flex items-center justify-between">
              <div className="flex items-center space-x-3">
                <Award className="w-5 h-5 text-purple-500" />
                <div>
                  <p className="font-medium">{rubric.title}</p>
                  <p className="text-sm text-text-tertiary">{rubric.points_possible} points</p>
                </div>
              </div>
              {isTeacher && (
                <div className="flex items-center gap-2">
                  <button
                    onClick={() => handleEdit(rubric)}
                    className="text-text-disabled hover:text-brand-600"
                    title="Edit rubric"
                  >
                    <Edit2 className="w-4 h-4" />
                  </button>
                  <button
                    onClick={() => handleDelete(rubric.id)}
                    className="text-text-disabled hover:text-accent-danger"
                    title="Delete rubric"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
              )}
            </div>
          ))
        )}
      </div>
    </Layout>
  );
};

export default RubricsPage;
