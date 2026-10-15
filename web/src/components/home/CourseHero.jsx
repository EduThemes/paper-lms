import { useMemo } from 'react';
import PropTypes from 'prop-types';
import { Card } from '../ui/card';
import { Avatar, AvatarFallback, AvatarImage } from '../ui/avatar';
import { cn } from '@/lib/utils';
import { normalizeLook, lookCssVars } from '../../utils/courseLook';

// The course home header. With a course look (courses.look, set from the
// Page Designer) it shows the look's banner art behind a scrim in the look's
// primary colour, with its on-primary text colour, light or dark colourway
// following Paper's theme (.course-look in index.css). Without one it is
// Paper's generated gradient, hued from the course id.

const hashHue = (id) => {
  const s = String(id ?? '0');
  let h = 0;
  for (let i = 0; i < s.length; i++) {
    h = (h * 31 + s.charCodeAt(i)) >>> 0;
  }
  return h % 360;
};

const initialsOf = (name) => {
  if (!name) return '?';
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((p) => p[0]?.toUpperCase() || '')
    .join('') || '?';
};

// ProgressRing draws in currentColor, so it reads on any header colour.
const ProgressRing = ({ value = 0, size = 92, stroke = 8, label = 'Course progress' }) => {
  const pct = Math.max(0, Math.min(100, value));
  const r = (size - stroke) / 2;
  const c = 2 * Math.PI * r;
  const offset = c - (pct / 100) * c;
  return (
    <div
      role="img"
      aria-label={`${label}: ${Math.round(pct)} percent`}
      className="relative inline-flex items-center justify-center"
      style={{ width: size, height: size }}
    >
      <svg width={size} height={size} className="-rotate-90">
        <circle
          cx={size / 2}
          cy={size / 2}
          r={r}
          stroke="currentColor"
          strokeOpacity={0.25}
          strokeWidth={stroke}
          fill="none"
        />
        <circle
          cx={size / 2}
          cy={size / 2}
          r={r}
          stroke="currentColor"
          strokeWidth={stroke}
          strokeLinecap="round"
          fill="none"
          strokeDasharray={c}
          strokeDashoffset={offset}
          style={{ transition: 'stroke-dashoffset 600ms cubic-bezier(.2,.8,.2,1)' }}
        />
      </svg>
      <div className="absolute inset-0 flex flex-col items-center justify-center">
        <span className="text-xl font-semibold leading-none">{Math.round(pct)}%</span>
        <span className="text-[10px] uppercase tracking-wider opacity-80 mt-1">Complete</span>
      </div>
    </div>
  );
};
ProgressRing.propTypes = {
  value: PropTypes.number, size: PropTypes.number, stroke: PropTypes.number, label: PropTypes.string,
};

const DotGrid = () => (
  <svg
    aria-hidden="true"
    className="absolute inset-0 h-full w-full opacity-20 pointer-events-none"
    xmlns="http://www.w3.org/2000/svg"
  >
    <defs>
      <pattern id="dotgrid" width="20" height="20" patternUnits="userSpaceOnUse">
        <circle cx="2" cy="2" r="1.2" fill="white" />
      </pattern>
    </defs>
    <rect width="100%" height="100%" fill="url(#dotgrid)" />
  </svg>
);

export default function CourseHero({ course, courseId, instructors, progress }) {
  const hue = useMemo(() => hashHue(courseId), [courseId]);
  const look = useMemo(() => normalizeLook(course?.look), [course?.look]);
  const gradient = `linear-gradient(135deg, hsl(${hue} 70% 42%) 0%, hsl(${(hue + 40) % 360} 70% 32%) 60%, hsl(${(hue + 80) % 360} 65% 28%) 100%)`;
  const term = course?.term?.name || course?.enrollment_term_id ? (course?.term?.name || `Term ${course?.enrollment_term_id ?? ''}`).trim() : null;
  const teachers = instructors?.length ? instructors : [];
  const teacherNames = teachers.slice(0, 2).map((t) => t.user?.name || t.name).filter(Boolean).join(', ');
  const extraTeachers = Math.max(0, teachers.length - 2);

  return (
    <Card
      className={cn(
        'relative h-48 overflow-hidden border-0 rounded-card shadow-md',
        look ? 'course-look' : 'text-white',
      )}
      style={lookCssVars(look)}
      data-testid="course-hero"
      data-look={look ? (look.themeId || 'custom') : undefined}
    >
      {look ? (
        <>
          {look.banner && (
            <img
              src={look.banner.src}
              srcSet={look.banner.srcSet}
              alt={look.banner.alt}
              aria-hidden={look.banner.alt ? undefined : 'true'}
              decoding="async"
              className="absolute inset-0 h-full w-full object-cover"
              data-testid="course-hero-banner"
            />
          )}
          <div className="course-look-scrim absolute inset-0" aria-hidden="true" />
        </>
      ) : (
        <>
          <div className="absolute inset-0" style={{ background: gradient }} data-testid="course-hero-gradient" />
          <DotGrid />
        </>
      )}
      <div className={cn('relative z-10 flex h-full items-center justify-between gap-6 p-6', look && 'course-look-text')}>
        <div className="min-w-0 flex-1 sm:max-w-[70%]">
          <div className="flex items-center gap-2 mb-2">
            {course?.course_code && (
              <span className="inline-flex items-center rounded-pill bg-surface-0/20 px-2.5 py-0.5 text-xs font-medium backdrop-blur-sm">
                {course.course_code}
              </span>
            )}
            {term && (
              <span className="inline-flex items-center rounded-pill bg-surface-0/15 px-2.5 py-0.5 text-xs font-medium backdrop-blur-sm">
                {term}
              </span>
            )}
          </div>
          <h1 className="font-display text-3xl md:text-4xl font-bold leading-tight tracking-tight truncate">
            {course?.name || 'Course'}
          </h1>
          {teachers.length > 0 && (
            <div className="mt-3 flex items-center gap-3">
              <div className="flex -space-x-2">
                {teachers.slice(0, 3).map((t, i) => {
                  const name = t.user?.name || t.name || 'Instructor';
                  const avatar = t.user?.avatar_url || t.avatar_url;
                  return (
                    <Avatar key={t.id ?? i} className="h-7 w-7 ring-2 ring-white/70">
                      {avatar && <AvatarImage src={avatar} alt={name} />}
                      <AvatarFallback className="bg-surface-0/30 text-white text-[10px] font-semibold">
                        {initialsOf(name)}
                      </AvatarFallback>
                    </Avatar>
                  );
                })}
              </div>
              <p className="text-sm opacity-90 truncate">
                Taught by {teacherNames || 'your instructor'}
                {extraTeachers > 0 && ` +${extraTeachers} more`}
              </p>
            </div>
          )}
        </div>
        <div className="hidden sm:block shrink-0">
          <ProgressRing value={progress} />
        </div>
      </div>
    </Card>
  );
}
CourseHero.propTypes = {
  course: PropTypes.object,
  courseId: PropTypes.oneOfType([PropTypes.string, PropTypes.number]),
  instructors: PropTypes.array,
  progress: PropTypes.number,
};
