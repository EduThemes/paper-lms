import React, { useState, useEffect, useCallback } from 'react';
import { useParams, Link } from 'react-router-dom';
import {
  Plus,
  Trash2,
  ChevronDown,
  ChevronRight,
  Target,
  FolderOpen,
  BarChart3,
  CheckCircle2,
  XCircle,
  MinusCircle,
  Edit2,
  Save,
  X,
} from 'lucide-react';
import { api } from '../services/api';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';

const CALCULATION_METHODS = [
  { value: 'decaying_average', label: 'Decaying Average', description: 'Average of scores weighted toward recent results' },
  { value: 'n_mastery', label: 'n Number of Times', description: 'Must achieve mastery n times' },
  { value: 'latest', label: 'Most Recent Score', description: 'Uses the most recent score' },
  { value: 'highest', label: 'Highest Score', description: 'Uses the highest score achieved' },
  { value: 'average', label: 'Average', description: 'Simple average of all scores' },
];

const DEFAULT_RATINGS = [
  { description: 'Exceeds Mastery', points: 4, color: 'bg-accent-success' },
  { description: 'Mastery', points: 3, color: 'bg-brand-500' },
  { description: 'Near Mastery', points: 2, color: 'bg-accent-warning' },
  { description: 'Below Mastery', points: 1, color: 'bg-orange-500' },
  { description: 'No Evidence', points: 0, color: 'bg-accent-danger' },
];

const getRatingColor = (index, total) => {
  const colors = ['bg-accent-success', 'bg-brand-500', 'bg-accent-warning', 'bg-orange-500', 'bg-accent-danger'];
  if (total <= colors.length) {
    return colors[index] || colors[colors.length - 1];
  }
  const colorIndex = Math.floor((index / (total - 1)) * (colors.length - 1));
  return colors[colorIndex] || colors[colors.length - 1];
};

const LearningOutcomesPage = () => {
  const { courseId } = useParams();
  const [groups, setGroups] = useState([]);
  const [groupOutcomes, setGroupOutcomes] = useState({});
  const [expandedGroups, setExpandedGroups] = useState({});
  const [selectedOutcome, setSelectedOutcome] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  // Create group form
  const [showCreateGroup, setShowCreateGroup] = useState(false);
  const [newGroupTitle, setNewGroupTitle] = useState('');
  const [newGroupDescription, setNewGroupDescription] = useState('');

  // Create outcome form
  const [showCreateOutcome, setShowCreateOutcome] = useState(null); // group ID or null
  const [newOutcomeTitle, setNewOutcomeTitle] = useState('');
  const [newOutcomeDescription, setNewOutcomeDescription] = useState('');
  const [newOutcomeMasteryPoints, setNewOutcomeMasteryPoints] = useState(3);
  const [newOutcomeCalcMethod, setNewOutcomeCalcMethod] = useState('decaying_average');
  const [newOutcomeCalcInt, setNewOutcomeCalcInt] = useState(65);
  const [newOutcomeRatings, setNewOutcomeRatings] = useState(
    DEFAULT_RATINGS.map((r) => ({ ...r }))
  );

  // Rollup view
  const [showRollup, setShowRollup] = useState(false);
  const [rollupData, setRollupData] = useState(null);
  const [rollupLoading, setRollupLoading] = useState(false);

  const fetchGroups = useCallback(async () => {
    try {
      setLoading(true);
      const result = await api.getCourseOutcomeGroups(courseId);
      const groupList = Array.isArray(result) ? result : result.data || [];
      setGroups(groupList);

      // Auto-expand and load outcomes for each group
      const expanded = {};
      const outcomes = {};
      for (const group of groupList) {
        expanded[group.id] = true;
        try {
          const outcomeResult = await api.getOutcomeGroupOutcomes(courseId, group.id);
          outcomes[group.id] = Array.isArray(outcomeResult)
            ? outcomeResult
            : outcomeResult.data || [];
        } catch {
          outcomes[group.id] = [];
        }
      }
      setExpandedGroups(expanded);
      setGroupOutcomes(outcomes);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [courseId]);

  useEffect(() => {
    fetchGroups();
  }, [fetchGroups]);

  const toggleGroup = async (groupId) => {
    const isExpanding = !expandedGroups[groupId];
    setExpandedGroups((prev) => ({ ...prev, [groupId]: isExpanding }));

    // Fetch outcomes if expanding and not yet loaded
    if (isExpanding && !groupOutcomes[groupId]) {
      try {
        const result = await api.getOutcomeGroupOutcomes(courseId, groupId);
        setGroupOutcomes((prev) => ({
          ...prev,
          [groupId]: Array.isArray(result) ? result : result.data || [],
        }));
      } catch {
        setGroupOutcomes((prev) => ({ ...prev, [groupId]: [] }));
      }
    }
  };

  const handleCreateGroup = async (e) => {
    e.preventDefault();
    try {
      await api.createOutcomeGroup(courseId, {
        title: newGroupTitle,
        description: newGroupDescription,
      });
      setNewGroupTitle('');
      setNewGroupDescription('');
      setShowCreateGroup(false);
      fetchGroups();
    } catch (err) {
      setError(err.message);
    }
  };

  const handleDeleteGroup = async (groupId) => {
    if (!window.confirm('Delete this outcome group and all its outcomes?')) return;
    try {
      await api.deleteOutcomeGroup(courseId, groupId);
      fetchGroups();
    } catch (err) {
      setError(err.message);
    }
  };

  const handleCreateOutcome = async (e) => {
    e.preventDefault();
    const groupId = showCreateOutcome;
    try {
      await api.createOutcome(courseId, groupId, {
        title: newOutcomeTitle,
        description: newOutcomeDescription,
        mastery_points: parseFloat(newOutcomeMasteryPoints) || 3,
        calculation_method: newOutcomeCalcMethod,
        calculation_int: parseInt(newOutcomeCalcInt) || 65,
        ratings: JSON.stringify(newOutcomeRatings),
      });

      // Reset form
      setNewOutcomeTitle('');
      setNewOutcomeDescription('');
      setNewOutcomeMasteryPoints(3);
      setNewOutcomeCalcMethod('decaying_average');
      setNewOutcomeCalcInt(65);
      setNewOutcomeRatings(DEFAULT_RATINGS.map((r) => ({ ...r })));
      setShowCreateOutcome(null);

      // Refresh outcomes for this group
      try {
        const result = await api.getOutcomeGroupOutcomes(courseId, groupId);
        setGroupOutcomes((prev) => ({
          ...prev,
          [groupId]: Array.isArray(result) ? result : result.data || [],
        }));
      } catch {
        // Ignore
      }
    } catch (err) {
      setError(err.message);
    }
  };

  const handleDeleteOutcome = async (groupId, outcomeId) => {
    if (!window.confirm('Delete this outcome?')) return;
    try {
      await api.deleteOutcome(courseId, groupId, outcomeId);
      // Refresh outcomes for this group
      try {
        const result = await api.getOutcomeGroupOutcomes(courseId, groupId);
        setGroupOutcomes((prev) => ({
          ...prev,
          [groupId]: Array.isArray(result) ? result : result.data || [],
        }));
      } catch {
        setGroupOutcomes((prev) => ({ ...prev, [groupId]: [] }));
      }
      if (selectedOutcome?.id === outcomeId) {
        setSelectedOutcome(null);
      }
    } catch (err) {
      setError(err.message);
    }
  };

  const handleViewRollup = async () => {
    setShowRollup(!showRollup);
    if (!showRollup && !rollupData) {
      setRollupLoading(true);
      try {
        const result = await api.getOutcomeRollup(courseId);
        setRollupData(result);
      } catch (err) {
        setError('Could not load outcome rollup: ' + err.message);
      } finally {
        setRollupLoading(false);
      }
    }
  };

  const addRating = () => {
    setNewOutcomeRatings([
      ...newOutcomeRatings,
      { description: '', points: 0 },
    ]);
  };

  const updateRating = (index, field, value) => {
    const updated = [...newOutcomeRatings];
    updated[index] = { ...updated[index], [field]: value };
    setNewOutcomeRatings(updated);
  };

  const removeRating = (index) => {
    if (newOutcomeRatings.length <= 2) return;
    setNewOutcomeRatings(newOutcomeRatings.filter((_, i) => i !== index));
  };

  const parseRatings = (outcome) => {
    if (!outcome.ratings) return DEFAULT_RATINGS;
    if (Array.isArray(outcome.ratings)) return outcome.ratings;
    try {
      const parsed = JSON.parse(outcome.ratings);
      return Array.isArray(parsed) ? parsed : DEFAULT_RATINGS;
    } catch {
      return DEFAULT_RATINGS;
    }
  };

  const getCalcMethodLabel = (method) => {
    const found = CALCULATION_METHODS.find((m) => m.value === method);
    return found ? found.label : method || 'Decaying Average';
  };

  if (loading) {
    return (
      <Layout>
        <div className="flex items-center justify-center py-12 gap-2 text-text-tertiary">
  <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" /></svg>
  Loading outcomes...
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
          <h2 className="text-2xl font-bold text-text-primary">Learning Outcomes</h2>
          <div className="flex items-center space-x-2">
            <button
              onClick={handleViewRollup}
              className={`flex items-center space-x-2 px-4 py-2 rounded-md text-sm border ${
                showRollup
                  ? 'bg-purple-50 border-purple-300 text-purple-700'
                  : 'bg-surface-0 border-border-strong text-text-secondary hover:bg-surface-1'
              }`}
            >
              <BarChart3 className="w-4 h-4" />
              <span>Mastery Rollup</span>
            </button>
            <button
              onClick={() => setShowCreateGroup(!showCreateGroup)}
              className="flex items-center space-x-2 bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm"
            >
              <Plus className="w-4 h-4" />
              <span>New Group</span>
            </button>
          </div>
        </div>
      </div>

      {error && (
        <div className="bg-accent-danger/10 text-accent-danger p-3 rounded mb-4 flex items-center justify-between">
          <span>{error}</span>
          <button onClick={() => setError(null)} className="text-accent-danger hover:text-accent-danger">
            <X className="w-4 h-4" />
          </button>
        </div>
      )}

      {/* Create Group Form */}
      {showCreateGroup && (
        <div className="bg-surface-0 rounded-lg shadow p-6 mb-6">
          <h3 className="font-semibold mb-4">Create Outcome Group</h3>
          <form onSubmit={handleCreateGroup} className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">Title</label>
              <input
                type="text"
                value={newGroupTitle}
                onChange={(e) => setNewGroupTitle(e.target.value)}
                placeholder="e.g., Common Core Math Standards"
                className="w-full border border-border-strong rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                required
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">
                Description (optional)
              </label>
              <textarea
                value={newGroupDescription}
                onChange={(e) => setNewGroupDescription(e.target.value)}
                placeholder="Describe this group of outcomes..."
                rows={2}
                className="w-full border border-border-strong rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
              />
            </div>
            <div className="flex space-x-3">
              <button
                type="submit"
                className="bg-brand-600 text-white px-4 py-2 rounded hover:bg-brand-700 text-sm"
              >
                Create Group
              </button>
              <button
                type="button"
                onClick={() => setShowCreateGroup(false)}
                className="text-text-tertiary hover:text-text-secondary text-sm"
              >
                Cancel
              </button>
            </div>
          </form>
        </div>
      )}

      {/* Rollup View */}
      {showRollup && (
        <div className="bg-surface-0 rounded-lg shadow mb-6 overflow-hidden">
          <div className="p-4 border-b bg-purple-50">
            <h3 className="font-semibold text-purple-800 flex items-center space-x-2">
              <BarChart3 className="w-5 h-5" />
              <span>Mastery Gradebook Summary</span>
            </h3>
            <p className="text-sm text-purple-600 mt-1">
              Overview of student mastery across all outcomes
            </p>
          </div>
          {rollupLoading ? (
            <div className="p-8 text-center text-text-tertiary">Loading rollup data...</div>
          ) : rollupData ? (
            <>
              {/* Desktop: wide table */}
              <div className="hidden md:block">
                <div className="overflow-x-auto -mx-4 px-4 md:mx-0 md:px-0">
                  <table className="min-w-full border-collapse" role="grid">
                    <thead>
                      <tr className="bg-surface-1">
                        <th className="px-4 py-3 text-left text-xs font-medium text-text-tertiary uppercase border-b border-r min-w-[200px]">
                          Student
                        </th>
                        {(rollupData.outcomes || []).map((outcome) => (
                          <th
                            key={outcome.id}
                            className="px-3 py-3 text-center text-xs font-medium text-text-tertiary border-b border-r min-w-[120px]"
                          >
                            <span className="block truncate" title={outcome.title}>
                              {outcome.title}
                            </span>
                            <span className="text-text-disabled font-normal">
                              {outcome.mastery_points || 3} pts
                            </span>
                          </th>
                        ))}
                      </tr>
                    </thead>
                    <tbody className="divide-y">
                      {(rollupData.students || []).length === 0 ? (
                        <tr>
                          <td
                            colSpan={(rollupData.outcomes?.length || 0) + 1}
                            className="px-4 py-8 text-center text-text-tertiary"
                          >
                            No rollup data available yet.
                          </td>
                        </tr>
                      ) : (
                        (rollupData.students || []).map((student) => (
                          <tr key={student.user_id} className="hover:bg-surface-1">
                            <td className="px-4 py-2 text-sm font-medium text-text-primary border-r whitespace-nowrap">
                              {student.user_name || `User ${student.user_id}`}
                            </td>
                            {(rollupData.outcomes || []).map((outcome) => {
                              const score = student.scores?.[outcome.id];
                              const mastery = outcome.mastery_points || 3;
                              const isMastered = score !== null && score !== undefined && score >= mastery;
                              return (
                                <td
                                  key={`${student.user_id}-${outcome.id}`}
                                  className={`px-3 py-2 text-center text-sm border-r ${
                                    score === null || score === undefined
                                      ? ''
                                      : isMastered
                                      ? 'bg-accent-success/10'
                                      : 'bg-accent-danger/10'
                                  }`}
                                >
                                  {score !== null && score !== undefined ? (
                                    <div className="flex items-center justify-center space-x-1">
                                      {isMastered ? (
                                        <CheckCircle2 className="w-3.5 h-3.5 text-accent-success" />
                                      ) : (
                                        <XCircle className="w-3.5 h-3.5 text-accent-danger" />
                                      )}
                                      <span
                                        className={`font-medium ${
                                          isMastered ? 'text-accent-success' : 'text-accent-danger'
                                        }`}
                                      >
                                        {score}
                                      </span>
                                    </div>
                                  ) : (
                                    <span className="text-text-disabled">-</span>
                                  )}
                                </td>
                              );
                            })}
                          </tr>
                        ))
                      )}
                    </tbody>
                  </table>
                </div>
              </div>

              {/* Mobile: stacked card layout (one card per student) */}
              <div className="md:hidden divide-y">
                {(rollupData.students || []).length === 0 ? (
                  <div className="px-4 py-8 text-center text-text-tertiary">
                    No rollup data available yet.
                  </div>
                ) : (
                  (rollupData.students || []).map((student) => (
                    <div key={student.user_id} className="p-4">
                      <p className="text-sm font-semibold text-text-primary mb-2">
                        {student.user_name || `User ${student.user_id}`}
                      </p>
                      <div className="space-y-1">
                        {(rollupData.outcomes || []).map((outcome) => {
                          const score = student.scores?.[outcome.id];
                          const mastery = outcome.mastery_points || 3;
                          const hasScore = score !== null && score !== undefined;
                          const isMastered = hasScore && score >= mastery;
                          return (
                            <div
                              key={`${student.user_id}-${outcome.id}`}
                              className={`flex items-center justify-between px-2 py-1.5 rounded text-sm ${
                                !hasScore ? 'bg-surface-1' : isMastered ? 'bg-accent-success/10' : 'bg-accent-danger/10'
                              }`}
                            >
                              <span className="text-xs text-text-secondary truncate pr-2 flex-1" title={outcome.title}>
                                {outcome.title}
                              </span>
                              <div className="flex items-center space-x-1 flex-shrink-0">
                                {!hasScore ? (
                                  <span className="text-text-disabled text-xs">No data</span>
                                ) : (
                                  <>
                                    {isMastered ? (
                                      <CheckCircle2 className="w-3.5 h-3.5 text-accent-success" />
                                    ) : (
                                      <XCircle className="w-3.5 h-3.5 text-accent-danger" />
                                    )}
                                    <span
                                      className={`font-medium text-xs ${
                                        isMastered ? 'text-accent-success' : 'text-accent-danger'
                                      }`}
                                    >
                                      {score}/{mastery}
                                    </span>
                                  </>
                                )}
                              </div>
                            </div>
                          );
                        })}
                      </div>
                    </div>
                  ))
                )}
              </div>
            </>
          ) : (
            <div className="p-8 text-center text-text-tertiary">
              No rollup data available. Outcomes must be assessed to view results.
            </div>
          )}
        </div>
      )}

      {/* Main Content: Groups & Outcomes + Detail Panel */}
      <div className="flex gap-4">
        {/* Groups & Outcomes List */}
        <div className="flex-1">
          {groups.length === 0 && !showCreateGroup ? (
            <div className="bg-surface-0 rounded-lg shadow p-8 text-center">
              <Target className="w-12 h-12 text-text-disabled mx-auto mb-3" />
              <p className="text-text-tertiary mb-4">No outcome groups yet.</p>
              <button
                onClick={() => setShowCreateGroup(true)}
                className="text-brand-600 hover:underline text-sm"
              >
                Create your first outcome group
              </button>
            </div>
          ) : (
            <div className="space-y-3">
              {groups.map((group) => (
                <div key={group.id} className="bg-surface-0 rounded-lg shadow overflow-hidden">
                  {/* Group Header */}
                  <div
                    className="flex items-center justify-between px-4 py-3 bg-surface-1 cursor-pointer hover:bg-surface-2 transition-colors"
                    onClick={() => toggleGroup(group.id)}
                  >
                    <div className="flex items-center space-x-3">
                      {expandedGroups[group.id] ? (
                        <ChevronDown className="w-4 h-4 text-text-tertiary" />
                      ) : (
                        <ChevronRight className="w-4 h-4 text-text-tertiary" />
                      )}
                      <FolderOpen className="w-5 h-5 text-accent-warning" />
                      <div>
                        <p className="font-semibold text-text-primary">{group.title}</p>
                        {group.description && (
                          <p className="text-xs text-text-tertiary mt-0.5">{group.description}</p>
                        )}
                      </div>
                    </div>
                    <div className="flex items-center space-x-2">
                      <span className="text-xs text-text-disabled">
                        {(groupOutcomes[group.id] || []).length} outcomes
                      </span>
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          setShowCreateOutcome(
                            showCreateOutcome === group.id ? null : group.id
                          );
                        }}
                        className="text-brand-500 hover:text-brand-700 p-1"
                        title="Add outcome"
                      >
                        <Plus className="w-4 h-4" />
                      </button>
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          handleDeleteGroup(group.id);
                        }}
                        className="text-text-disabled hover:text-accent-danger p-1"
                        title="Delete group"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </div>

                  {/* Outcomes List */}
                  {expandedGroups[group.id] && (
                    <div>
                      {/* Create Outcome Form */}
                      {showCreateOutcome === group.id && (
                        <div className="p-4 border-b bg-brand-50">
                          <h4 className="font-medium text-sm text-brand-800 mb-3">
                            New Outcome in "{group.title}"
                          </h4>
                          <form onSubmit={handleCreateOutcome} className="space-y-3">
                            <div className="grid grid-cols-2 gap-3">
                              <div className="col-span-2">
                                <input
                                  type="text"
                                  value={newOutcomeTitle}
                                  onChange={(e) => setNewOutcomeTitle(e.target.value)}
                                  placeholder="Outcome title"
                                  className="w-full border border-border-strong rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                                  required
                                />
                              </div>
                              <div className="col-span-2">
                                <textarea
                                  value={newOutcomeDescription}
                                  onChange={(e) => setNewOutcomeDescription(e.target.value)}
                                  placeholder="Description (optional)"
                                  rows={2}
                                  className="w-full border border-border-strong rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                                />
                              </div>
                              <div>
                                <label className="block text-xs font-medium text-text-secondary mb-1">
                                  Calculation Method
                                </label>
                                <select
                                  value={newOutcomeCalcMethod}
                                  onChange={(e) => setNewOutcomeCalcMethod(e.target.value)}
                                  className="w-full border border-border-strong rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                                >
                                  {CALCULATION_METHODS.map((m) => (
                                    <option key={m.value} value={m.value}>
                                      {m.label}
                                    </option>
                                  ))}
                                </select>
                              </div>
                              <div className="grid grid-cols-2 gap-2">
                                <div>
                                  <label className="block text-xs font-medium text-text-secondary mb-1">
                                    Mastery Points
                                  </label>
                                  <input
                                    type="number"
                                    min="0"
                                    step="0.5"
                                    value={newOutcomeMasteryPoints}
                                    onChange={(e) =>
                                      setNewOutcomeMasteryPoints(e.target.value)
                                    }
                                    className="w-full border border-border-strong rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                                  />
                                </div>
                                {(newOutcomeCalcMethod === 'decaying_average' ||
                                  newOutcomeCalcMethod === 'n_mastery') && (
                                  <div>
                                    <label className="block text-xs font-medium text-text-secondary mb-1">
                                      {newOutcomeCalcMethod === 'decaying_average'
                                        ? 'Decay %'
                                        : 'N Count'}
                                    </label>
                                    <input
                                      type="number"
                                      min="1"
                                      max={
                                        newOutcomeCalcMethod === 'decaying_average'
                                          ? 99
                                          : 10
                                      }
                                      value={newOutcomeCalcInt}
                                      onChange={(e) => setNewOutcomeCalcInt(e.target.value)}
                                      className="w-full border border-border-strong rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                                    />
                                  </div>
                                )}
                              </div>
                            </div>

                            {/* Ratings */}
                            <div>
                              <label className="block text-xs font-medium text-text-secondary mb-2">
                                Ratings
                              </label>
                              <div className="space-y-2">
                                {newOutcomeRatings.map((rating, idx) => (
                                  <div key={idx} className="flex items-center space-x-2">
                                    <div
                                      className={`w-3 h-3 rounded-full flex-shrink-0 ${getRatingColor(
                                        idx,
                                        newOutcomeRatings.length
                                      )}`}
                                    />
                                    <input
                                      type="text"
                                      value={rating.description}
                                      onChange={(e) =>
                                        updateRating(idx, 'description', e.target.value)
                                      }
                                      placeholder="Rating description"
                                      className="flex-1 border border-border-strong rounded px-2 py-1 text-sm"
                                    />
                                    <input
                                      type="number"
                                      min="0"
                                      step="0.5"
                                      value={rating.points}
                                      onChange={(e) =>
                                        updateRating(
                                          idx,
                                          'points',
                                          parseFloat(e.target.value) || 0
                                        )
                                      }
                                      className="w-20 border border-border-strong rounded px-2 py-1 text-sm"
                                    />
                                    <span className="text-xs text-text-disabled">pts</span>
                                    {newOutcomeRatings.length > 2 && (
                                      <button
                                        type="button"
                                        onClick={() => removeRating(idx)}
                                        className="text-accent-danger hover:text-accent-danger"
                                      >
                                        <X className="w-3.5 h-3.5" />
                                      </button>
                                    )}
                                  </div>
                                ))}
                              </div>
                              <button
                                type="button"
                                onClick={addRating}
                                className="text-brand-600 hover:underline text-xs mt-1"
                              >
                                + Add Rating
                              </button>
                            </div>

                            <div className="flex space-x-3">
                              <button
                                type="submit"
                                className="bg-brand-600 text-white px-4 py-2 rounded hover:bg-brand-700 text-sm flex items-center space-x-1"
                              >
                                <Save className="w-3.5 h-3.5" />
                                <span>Create Outcome</span>
                              </button>
                              <button
                                type="button"
                                onClick={() => setShowCreateOutcome(null)}
                                className="text-text-tertiary hover:text-text-secondary text-sm"
                              >
                                Cancel
                              </button>
                            </div>
                          </form>
                        </div>
                      )}

                      {/* Outcomes */}
                      {(groupOutcomes[group.id] || []).length === 0 ? (
                        <div className="px-4 py-6 text-center text-text-disabled text-sm">
                          No outcomes in this group.{' '}
                          <button
                            onClick={() => setShowCreateOutcome(group.id)}
                            className="text-brand-500 hover:underline"
                          >
                            Add one
                          </button>
                        </div>
                      ) : (
                        <div className="divide-y">
                          {(groupOutcomes[group.id] || []).map((outcome) => {
                            const ratings = parseRatings(outcome);
                            const isSelected = selectedOutcome?.id === outcome.id;
                            return (
                              <div
                                key={outcome.id}
                                className={`px-4 py-3 cursor-pointer transition-colors ${
                                  isSelected
                                    ? 'bg-brand-50 border-l-4 border-l-blue-500'
                                    : 'hover:bg-surface-1 border-l-4 border-l-transparent'
                                }`}
                                onClick={() =>
                                  setSelectedOutcome(isSelected ? null : outcome)
                                }
                              >
                                <div className="flex items-center justify-between">
                                  <div className="flex items-center space-x-3">
                                    <Target className="w-4 h-4 text-brand-500 flex-shrink-0" />
                                    <div>
                                      <p className="font-medium text-sm text-text-primary">
                                        {outcome.title}
                                      </p>
                                      {outcome.description && (
                                        <p className="text-xs text-text-tertiary mt-0.5 line-clamp-1">
                                          {outcome.description}
                                        </p>
                                      )}
                                    </div>
                                  </div>
                                  <div className="flex items-center space-x-3">
                                    <div className="flex items-center space-x-1">
                                      {ratings.slice(0, 5).map((r, idx) => (
                                        <div
                                          key={idx}
                                          className={`w-2 h-2 rounded-full ${getRatingColor(
                                            idx,
                                            ratings.length
                                          )}`}
                                          title={`${r.description}: ${r.points} pts`}
                                        />
                                      ))}
                                    </div>
                                    <span className="text-xs text-text-disabled whitespace-nowrap">
                                      Mastery: {outcome.mastery_points ?? 3}
                                    </span>
                                    <button
                                      onClick={(e) => {
                                        e.stopPropagation();
                                        handleDeleteOutcome(group.id, outcome.id);
                                      }}
                                      className="text-text-disabled hover:text-accent-danger p-1"
                                    >
                                      <Trash2 className="w-3.5 h-3.5" />
                                    </button>
                                  </div>
                                </div>
                              </div>
                            );
                          })}
                        </div>
                      )}
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Detail Panel */}
        {selectedOutcome && (
          <div className="w-96 flex-shrink-0">
            <div className="bg-surface-0 rounded-lg shadow sticky top-4">
              <div className="p-4 border-b bg-surface-1">
                <div className="flex items-center justify-between">
                  <h3 className="font-semibold text-text-primary">Outcome Details</h3>
                  <button
                    onClick={() => setSelectedOutcome(null)}
                    className="text-text-disabled hover:text-text-secondary"
                  >
                    <X className="w-4 h-4" />
                  </button>
                </div>
              </div>
              <div className="p-4 space-y-4">
                <div>
                  <h4 className="text-lg font-semibold text-text-primary">
                    {selectedOutcome.title}
                  </h4>
                  {selectedOutcome.description && (
                    <p className="text-sm text-text-secondary mt-1">
                      {selectedOutcome.description}
                    </p>
                  )}
                </div>

                {/* Calculation Method */}
                <div className="bg-surface-1 rounded-lg p-3">
                  <p className="text-xs font-medium text-text-tertiary uppercase mb-1">
                    Calculation Method
                  </p>
                  <p className="text-sm font-medium text-text-primary">
                    {getCalcMethodLabel(selectedOutcome.calculation_method)}
                  </p>
                  {selectedOutcome.calculation_int && (
                    <p className="text-xs text-text-tertiary mt-0.5">
                      {selectedOutcome.calculation_method === 'decaying_average'
                        ? `${selectedOutcome.calculation_int}% weighted to most recent`
                        : selectedOutcome.calculation_method === 'n_mastery'
                        ? `Must achieve mastery ${selectedOutcome.calculation_int} times`
                        : ''}
                    </p>
                  )}
                </div>

                {/* Mastery */}
                <div className="bg-brand-50 rounded-lg p-3">
                  <p className="text-xs font-medium text-brand-600 uppercase mb-1">
                    Mastery Threshold
                  </p>
                  <p className="text-lg font-bold text-brand-800">
                    {selectedOutcome.mastery_points ?? 3} points
                  </p>
                </div>

                {/* Ratings */}
                <div>
                  <p className="text-xs font-medium text-text-tertiary uppercase mb-2">
                    Proficiency Ratings
                  </p>
                  <div className="space-y-2">
                    {parseRatings(selectedOutcome).map((rating, idx) => {
                      const ratings = parseRatings(selectedOutcome);
                      const isMastery =
                        rating.points >= (selectedOutcome.mastery_points ?? 3);
                      return (
                        <div
                          key={idx}
                          className={`flex items-center space-x-3 p-2 rounded ${
                            isMastery ? 'bg-accent-success/10' : 'bg-surface-1'
                          }`}
                        >
                          <div
                            className={`w-4 h-4 rounded-full flex-shrink-0 ${getRatingColor(
                              idx,
                              ratings.length
                            )}`}
                          />
                          <div className="flex-1">
                            <p className="text-sm font-medium text-text-primary">
                              {rating.description}
                            </p>
                          </div>
                          <div className="text-right">
                            <span className="text-sm font-semibold text-text-secondary">
                              {rating.points}
                            </span>
                            <span className="text-xs text-text-disabled ml-1">pts</span>
                          </div>
                          {isMastery && (
                            <CheckCircle2 className="w-4 h-4 text-accent-success flex-shrink-0" />
                          )}
                        </div>
                      );
                    })}
                  </div>
                </div>

                {/* Visual: Ratings bar */}
                <div>
                  <p className="text-xs font-medium text-text-tertiary uppercase mb-2">
                    Proficiency Scale
                  </p>
                  <div className="flex rounded-full overflow-hidden h-3">
                    {parseRatings(selectedOutcome).map((rating, idx) => {
                      const ratings = parseRatings(selectedOutcome);
                      return (
                        <div
                          key={idx}
                          className={`flex-1 ${getRatingColor(idx, ratings.length)}`}
                          title={`${rating.description}: ${rating.points} pts`}
                        />
                      );
                    })}
                  </div>
                  <div className="flex justify-between mt-1">
                    <span className="text-xs text-text-disabled">Low</span>
                    <span className="text-xs text-text-disabled">High</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        )}
      </div>
    </Layout>
  );
};

export default LearningOutcomesPage;
