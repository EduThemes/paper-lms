import { ChevronLeft, ChevronRight } from 'lucide-react';

/**
 * Previous / next pager for lists backed by the API's Link-header
 * pagination (`{ data, pagination }` from services/api.js). Renders
 * nothing when the list fits on one page.
 */
const ListPager = ({ page, pagination, onPageChange, label = 'Pagination' }) => {
  const hasPrev = page > 1 || !!pagination?.prev;
  const hasNext = !!pagination?.next;
  if (!hasPrev && !hasNext) return null;

  return (
    <nav aria-label={label} className="flex items-center justify-between mt-4 text-sm">
      <button
        type="button"
        onClick={() => onPageChange(page - 1)}
        disabled={!hasPrev}
        className="inline-flex items-center gap-1 px-3 py-1.5 rounded-md border border-border-strong text-text-secondary hover:bg-surface-1 disabled:opacity-40 disabled:cursor-not-allowed"
      >
        <ChevronLeft className="w-4 h-4" aria-hidden="true" />
        Previous
      </button>
      <span className="text-text-tertiary" aria-live="polite">Page {page}</span>
      <button
        type="button"
        onClick={() => onPageChange(page + 1)}
        disabled={!hasNext}
        className="inline-flex items-center gap-1 px-3 py-1.5 rounded-md border border-border-strong text-text-secondary hover:bg-surface-1 disabled:opacity-40 disabled:cursor-not-allowed"
      >
        Next
        <ChevronRight className="w-4 h-4" aria-hidden="true" />
      </button>
    </nav>
  );
};

export default ListPager;
