import React from 'react';
import { Star, Download, BookOpen, FileText, ListChecks, Layers, MessageSquare, Library } from 'lucide-react';

const RESOURCE_ICONS = {
  course: Library,
  assignment: FileText,
  page: BookOpen,
  quiz: ListChecks,
  module: Layers,
  discussion_topic: MessageSquare,
};

const RESOURCE_LABELS = {
  course: 'Course',
  assignment: 'Assignment',
  page: 'Page',
  quiz: 'Quiz',
  module: 'Module',
  discussion_topic: 'Discussion',
};

/**
 * CommonsCard renders a single Commons catalog item: thumbnail, title,
 * resource-type badge, subject + grade pills, and download/favorite counts.
 */
const CommonsCard = ({ item, isFavorited, onFavorite, onClick }) => {
  const Icon = RESOURCE_ICONS[item.resource_type] || Library;
  const handleStarClick = (e) => {
    e.stopPropagation();
    onFavorite?.(item);
  };

  return (
    <button
      type="button"
      onClick={() => onClick?.(item)}
      className="group flex flex-col text-left bg-surface-0 border border-slate-200 rounded-lg overflow-hidden shadow-sm hover:shadow-md focus:ring-2 focus:ring-indigo-500 focus:outline-none transition"
    >
      <div className="relative h-32 bg-gradient-to-br from-indigo-100 to-purple-100 flex items-center justify-center">
        {item.thumbnail_url ? (
          <img src={item.thumbnail_url} alt="" className="w-full h-full object-cover" />
        ) : (
          <Icon className="w-12 h-12 text-indigo-400" aria-hidden="true" />
        )}
        <span className="absolute top-2 left-2 inline-flex items-center gap-1 text-[11px] font-medium uppercase tracking-wide bg-surface-0/90 text-slate-700 px-2 py-0.5 rounded">
          <Icon className="w-3 h-3" aria-hidden="true" />
          {RESOURCE_LABELS[item.resource_type] || item.resource_type}
        </span>
        <span
          role="button"
          tabIndex={0}
          aria-label={isFavorited ? 'Unfavorite' : 'Favorite'}
          onClick={handleStarClick}
          onKeyDown={(e) => {
            if (e.key === 'Enter' || e.key === ' ') {
              e.preventDefault();
              handleStarClick(e);
            }
          }}
          className="absolute top-2 right-2 p-1 rounded-full bg-surface-0/90 hover:bg-surface-0"
        >
          <Star
            className={`w-4 h-4 ${isFavorited ? 'fill-yellow-400 text-yellow-400' : 'text-slate-400'}`}
          />
        </span>
      </div>
      <div className="p-3 flex flex-col flex-1">
        <h3 className="text-sm font-semibold text-slate-900 line-clamp-2">{item.title}</h3>
        {item.description && (
          <p className="mt-1 text-xs text-slate-500 line-clamp-2">{item.description}</p>
        )}
        <div className="mt-2 flex flex-wrap gap-1">
          {item.subject && (
            <span className="text-[10px] font-medium bg-slate-100 text-slate-700 px-2 py-0.5 rounded">
              {item.subject}
            </span>
          )}
          {item.grade_level && (
            <span className="text-[10px] font-medium bg-slate-100 text-slate-700 px-2 py-0.5 rounded">
              {item.grade_level}
            </span>
          )}
        </div>
        <div className="mt-3 flex items-center justify-between text-xs text-slate-500">
          <span className="flex items-center gap-1">
            <Download className="w-3.5 h-3.5" aria-hidden="true" />
            {item.download_count || 0}
          </span>
          <span className="flex items-center gap-1">
            <Star className="w-3.5 h-3.5" aria-hidden="true" />
            {item.favorite_count || 0}
          </span>
        </div>
      </div>
    </button>
  );
};

export default CommonsCard;
