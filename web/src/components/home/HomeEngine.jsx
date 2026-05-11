import React, { useState, useEffect } from 'react';
import { useParams } from 'react-router-dom';
import { api } from '../../services/api';
import { useCourseUI } from '../../contexts/CourseUIContext';
import HomeButton from './HomeButton';

const HomeEngine = () => {
  const { courseId } = useParams();
  const { uiMode, isK2, is35 } = useCourseUI();
  const [homeData, setHomeData] = useState(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.getCourseHomeData(courseId)
      .then(data => { setHomeData(data); setLoading(false); })
      .catch(() => setLoading(false));
  }, [courseId]);

  if (loading) return <div className="text-center py-12 text-text-tertiary">Loading...</div>;
  if (!homeData) return <div className="text-center py-12 text-text-tertiary">No home data available.</div>;

  // Grid layout varies by mode
  const gridClass = isK2
    ? 'grid grid-cols-2 gap-4'
    : is35
    ? 'grid grid-cols-2 md:grid-cols-3 gap-4'
    : 'grid grid-cols-2 md:grid-cols-4 gap-4';

  return (
    <div className={gridClass}>
      {homeData.buttons?.map(button => (
        <HomeButton
          key={button.id}
          button={button}
          uiMode={uiMode}
          todaysLesson={homeData.todays_lesson}
          continueData={homeData.continue_url}
          courseId={courseId}
        />
      ))}
    </div>
  );
};

export default HomeEngine;
