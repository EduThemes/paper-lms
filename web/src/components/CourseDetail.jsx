import React, { useState, useEffect } from 'react';
import { ChevronRight, ChevronDown, Book, FileText, PenTool } from 'lucide-react';
import { api } from '../services/api';

const CourseDetail = ({ courseId }) => {
  const [course, setCourse] = useState(null);
  const [modules, setModules] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [expandedModules, setExpandedModules] = useState({});

  useEffect(() => {
    const fetchCourseData = async () => {
      try {
        const [courseData, modulesData] = await Promise.all([
          api.getCourse(courseId),
          api.getModules(courseId),
        ]);
        setCourse(courseData.course);
        setModules(modulesData);
      } catch (err) {
        setError(err.message);
      } finally {
        setLoading(false);
      }
    };

    fetchCourseData();
  }, [courseId]);

  const toggleModule = (moduleId) => {
    setExpandedModules(prev => ({
      ...prev,
      [moduleId]: !prev[moduleId]
    }));
  };

  const getItemIcon = (type) => {
    switch (type) {
      case 'page':
        return <FileText className="w-4 h-4" />;
      case 'assignment':
        return <PenTool className="w-4 h-4" />;
      default:
        return <Book className="w-4 h-4" />;
    }
  };

  if (loading) return <div className="p-4">Loading course...</div>;
  if (error) return <div className="p-4 text-accent-danger">{error}</div>;
  if (!course) return <div className="p-4">Course not found</div>;

  return (
    <div className="bg-surface-0 rounded-lg shadow">
      <div className="p-6 border-b">
        <h1 className="text-2xl font-bold">{course.name}</h1>
        <p className="text-text-secondary">{course.code}</p>
        <p className="mt-2">{course.description}</p>
      </div>

      <div className="p-6">
        <h2 className="text-lg font-semibold mb-4">Modules</h2>
        <div className="space-y-2">
          {modules.map((module) => (
            <div key={module.id} className="border rounded-lg">
              <button
                className="w-full px-4 py-3 flex items-center justify-between hover:bg-surface-1"
                onClick={() => toggleModule(module.id)}
              >
                <span className="font-medium">{module.title}</span>
                {expandedModules[module.id] ? (
                  <ChevronDown className="w-5 h-5" />
                ) : (
                  <ChevronRight className="w-5 h-5" />
                )}
              </button>
              
              {expandedModules[module.id] && module.items && (
                <div className="px-4 py-2 border-t">
                  {module.items.map((item) => (
                    <div
                      key={item.id}
                      className="flex items-center space-x-2 py-2 px-4 hover:bg-surface-1 rounded"
                    >
                      {getItemIcon(item.type)}
                      <span>{item.title}</span>
                    </div>
                  ))}
                </div>
              )}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
};

export default CourseDetail;