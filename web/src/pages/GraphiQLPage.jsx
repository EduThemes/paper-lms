import React, { useState } from 'react';
import Layout from '../components/Layout';
import { getCSRFToken } from '../services/api';

const API_URL = import.meta.env.VITE_API_URL || '/api/v1';

const DEFAULT_QUERY = `{
  self {
    id
    name
    email
    created_at
  }
  allCourses(page: 1, perPage: 5) {
    id
    name
    course_code
    workflow_state
    assignments {
      id
      name
      points_possible
      due_at
    }
    modules {
      id
      name
      position
    }
  }
}`;

const GraphiQLPage = () => {
  const [query, setQuery] = useState(DEFAULT_QUERY);
  const [variables, setVariables] = useState('{}');
  const [result, setResult] = useState(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const [activeTab, setActiveTab] = useState('query');

  const executeQuery = async () => {
    setLoading(true);
    setError(null);
    setResult(null);

    try {
      let parsedVariables = {};
      if (variables.trim()) {
        try {
          parsedVariables = JSON.parse(variables);
        } catch (e) {
          setError('Invalid JSON in variables: ' + e.message);
          setLoading(false);
          return;
        }
      }

      const response = await fetch(API_URL + '/graphql', {
        method: 'POST',
        credentials: 'include',
        headers: {
          'Content-Type': 'application/json',
          'X-CSRF-Token': getCSRFToken(),
        },
        body: JSON.stringify({
          query: query,
          variables: parsedVariables,
        }),
      });

      const data = await response.json();
      setResult(data);
    } catch (err) {
      setError('Network error: ' + err.message);
    } finally {
      setLoading(false);
    }
  };

  const handleKeyDown = (e) => {
    if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
      e.preventDefault();
      executeQuery();
    }
    // Handle Tab key for indentation in textareas
    if (e.key === 'Tab') {
      e.preventDefault();
      const textarea = e.target;
      const start = textarea.selectionStart;
      const end = textarea.selectionEnd;
      const value = textarea.value;
      const newValue = value.substring(0, start) + '  ' + value.substring(end);
      if (activeTab === 'query') {
        setQuery(newValue);
      } else {
        setVariables(newValue);
      }
      // Restore cursor position after React re-render
      requestAnimationFrame(() => {
        textarea.selectionStart = start + 2;
        textarea.selectionEnd = start + 2;
      });
    }
  };

  const exampleQueries = [
    {
      name: 'Current User',
      query: `{
  self {
    id
    name
    email
    login_id
    created_at
  }
}`,
    },
    {
      name: 'All Courses with Assignments',
      query: `{
  allCourses(page: 1, perPage: 10) {
    id
    name
    course_code
    workflow_state
    assignments {
      id
      name
      points_possible
      due_at
    }
  }
}`,
    },
    {
      name: 'Single Course with Details',
      query: `{
  course(id: 1) {
    id
    name
    course_code
    workflow_state
    created_at
    assignments {
      id
      name
      description
      points_possible
      due_at
    }
    enrollments {
      id
      user_id
      type
      role
      workflow_state
    }
    modules {
      id
      name
      position
      workflow_state
    }
  }
}`,
    },
    {
      name: 'Single Assignment',
      query: `{
  assignment(id: 1) {
    id
    name
    description
    points_possible
    due_at
    course_id
    grading_type
    workflow_state
  }
}`,
    },
    {
      name: 'User by ID',
      query: `{
  user(id: 1) {
    id
    name
    email
    short_name
    locale
    time_zone
    created_at
  }
}`,
    },
  ];

  return (
    <Layout>
      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold text-text-primary">GraphQL Explorer</h1>
            <p className="mt-1 text-sm text-text-tertiary">
              Query the Paper LMS API using GraphQL. Press Ctrl+Enter (Cmd+Enter) to execute.
            </p>
          </div>
        </div>

        {/* Example Queries */}
        <div className="flex flex-wrap gap-2">
          <span className="text-sm text-text-tertiary self-center">Examples:</span>
          {exampleQueries.map((example) => (
            <button
              key={example.name}
              onClick={() => {
                setQuery(example.query);
                setVariables('{}');
              }}
              className="px-3 py-1 text-xs font-medium rounded-full bg-brand-100 text-brand-700 hover:bg-blue-200 transition-colors"
            >
              {example.name}
            </button>
          ))}
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          {/* Left Panel - Query Editor */}
          <div className="space-y-2">
            {/* Tabs */}
            <div className="flex border-b border-border-default">
              <button
                onClick={() => setActiveTab('query')}
                className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
                  activeTab === 'query'
                    ? 'border-brand-500 text-brand-600'
                    : 'border-transparent text-text-tertiary hover:text-text-secondary'
                }`}
              >
                Query
              </button>
              <button
                onClick={() => setActiveTab('variables')}
                className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
                  activeTab === 'variables'
                    ? 'border-brand-500 text-brand-600'
                    : 'border-transparent text-text-tertiary hover:text-text-secondary'
                }`}
              >
                Variables
              </button>
            </div>

            {/* Editor */}
            <div className="relative">
              {activeTab === 'query' ? (
                <textarea
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  onKeyDown={handleKeyDown}
                  className="w-full h-96 p-4 font-mono text-sm bg-gray-900 text-green-400 rounded-lg border border-gray-700 focus:outline-none focus:ring-2 focus:ring-brand-500 resize-none"
                  placeholder="Enter your GraphQL query..."
                  spellCheck="false"
                />
              ) : (
                <textarea
                  value={variables}
                  onChange={(e) => setVariables(e.target.value)}
                  onKeyDown={handleKeyDown}
                  className="w-full h-96 p-4 font-mono text-sm bg-gray-900 text-yellow-400 rounded-lg border border-gray-700 focus:outline-none focus:ring-2 focus:ring-brand-500 resize-none"
                  placeholder='{"key": "value"}'
                  spellCheck="false"
                />
              )}
            </div>

            {/* Execute Button */}
            <button
              onClick={executeQuery}
              disabled={loading || !query.trim()}
              className={`w-full flex items-center justify-center gap-2 px-4 py-2.5 text-sm font-medium rounded-lg transition-colors ${
                loading || !query.trim()
                  ? 'bg-gray-300 text-text-tertiary cursor-not-allowed'
                  : 'bg-brand-600 text-white hover:bg-brand-700'
              }`}
            >
              {loading ? (
                <>
                  <svg className="animate-spin h-4 w-4" viewBox="0 0 24 24">
                    <circle
                      className="opacity-25"
                      cx="12"
                      cy="12"
                      r="10"
                      stroke="currentColor"
                      strokeWidth="4"
                      fill="none"
                    />
                    <path
                      className="opacity-75"
                      fill="currentColor"
                      d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                    />
                  </svg>
                  Executing...
                </>
              ) : (
                <>
                  <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                  Execute Query
                </>
              )}
            </button>
          </div>

          {/* Right Panel - Results */}
          <div className="space-y-2">
            <div className="flex border-b border-border-default">
              <span className="px-4 py-2 text-sm font-medium border-b-2 border-brand-500 text-brand-600">
                Response
              </span>
            </div>

            <div className="relative">
              {error && (
                <div className="mb-2 p-3 bg-accent-danger/10 border border-accent-danger/30 rounded-lg">
                  <p className="text-sm text-accent-danger">{error}</p>
                </div>
              )}

              <pre className="w-full h-96 p-4 font-mono text-sm bg-gray-900 text-gray-100 rounded-lg border border-gray-700 overflow-auto whitespace-pre-wrap">
                {result
                  ? JSON.stringify(result, null, 2)
                  : loading
                  ? 'Executing query...'
                  : 'Results will appear here after executing a query.'}
              </pre>

              {result && (
                <button
                  onClick={() => {
                    navigator.clipboard.writeText(JSON.stringify(result, null, 2));
                  }}
                  className="absolute top-10 right-3 px-2 py-1 text-xs bg-gray-700 text-gray-300 rounded hover:bg-gray-600 transition-colors"
                  title="Copy to clipboard"
                >
                  Copy
                </button>
              )}
            </div>

            {/* Response metadata */}
            {result && (
              <div className="flex gap-4 text-xs text-text-tertiary">
                {result.errors && result.errors.length > 0 && (
                  <span className="text-accent-danger">
                    {result.errors.length} error{result.errors.length !== 1 ? 's' : ''}
                  </span>
                )}
                {result.data && (
                  <span className="text-accent-success">
                    {Object.keys(result.data).length} field{Object.keys(result.data).length !== 1 ? 's' : ''} returned
                  </span>
                )}
              </div>
            )}
          </div>
        </div>

        {/* Schema Reference */}
        <details className="bg-surface-0 rounded-lg border border-border-default p-4">
          <summary className="text-sm font-medium text-text-secondary cursor-pointer hover:text-text-primary">
            Schema Reference
          </summary>
          <div className="mt-3 grid grid-cols-1 md:grid-cols-2 gap-4 text-sm">
            <div>
              <h3 className="font-semibold text-text-primary mb-2">Root Queries</h3>
              <div className="space-y-2 font-mono text-xs bg-surface-1 p-3 rounded">
                <p><span className="text-brand-600">course</span>(id: ID): Course</p>
                <p><span className="text-brand-600">allCourses</span>(page: Int, perPage: Int): [Course]</p>
                <p><span className="text-brand-600">assignment</span>(id: ID): Assignment</p>
                <p><span className="text-brand-600">self</span>: User</p>
                <p><span className="text-brand-600">user</span>(id: ID): User</p>
              </div>
            </div>
            <div>
              <h3 className="font-semibold text-text-primary mb-2">Types</h3>
              <div className="space-y-2 font-mono text-xs bg-surface-1 p-3 rounded">
                <p className="font-semibold text-purple-600">Course</p>
                <p className="ml-2">id, name, course_code, workflow_state, account_id, start_at, end_at, default_view, is_public, created_at, updated_at</p>
                <p className="ml-2 text-accent-success">assignments, enrollments, modules (nested)</p>
                <p className="font-semibold text-purple-600 mt-2">Assignment</p>
                <p className="ml-2">id, name, description, points_possible, due_at, unlock_at, lock_at, course_id, grading_type, submission_types, workflow_state, published, position, created_at</p>
                <p className="font-semibold text-purple-600 mt-2">User</p>
                <p className="ml-2">id, name, sortable_name, short_name, email, login_id, avatar_url, locale, time_zone, created_at</p>
                <p className="font-semibold text-purple-600 mt-2">Enrollment</p>
                <p className="ml-2">id, user_id, course_id, type, role, workflow_state, created_at</p>
                <p className="font-semibold text-purple-600 mt-2">Module</p>
                <p className="ml-2">id, name, position, course_id, unlock_at, require_sequential_progress, workflow_state, created_at</p>
              </div>
            </div>
          </div>
        </details>
      </div>
    </Layout>
  );
};

export default GraphiQLPage;
