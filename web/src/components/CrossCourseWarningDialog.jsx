import React from 'react';
import { AlertTriangle, X } from 'lucide-react';

export default function CrossCourseWarningDialog({ issues, onGoBack, onSaveAnyway }) {
  if (!issues || issues.length === 0) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="bg-surface-0 rounded-lg shadow-xl max-w-lg w-full mx-4">
        <div className="flex items-center gap-3 px-6 py-4 border-b border-border-default">
          <div className="flex items-center justify-center w-10 h-10 bg-accent-warning/20 rounded-full shrink-0">
            <AlertTriangle size={20} className="text-accent-warning" />
          </div>
          <div>
            <h2 className="text-lg font-semibold text-text-primary">Cross-Course References Detected</h2>
            <p className="text-sm text-text-tertiary mt-0.5">
              This content contains links or images that reference other courses. Students enrolled only in this course may not be able to access them.
            </p>
          </div>
          <button type="button" onClick={onGoBack} className="ml-auto text-text-disabled hover:text-text-secondary shrink-0">
            <X size={20} />
          </button>
        </div>

        <div className="px-6 py-4 max-h-60 overflow-y-auto">
          <ul className="space-y-2">
            {issues.map((issue, i) => (
              <li key={i} className="flex items-start gap-2 text-sm">
                <span className="inline-block mt-0.5 w-2 h-2 bg-amber-400 rounded-full shrink-0" />
                <div>
                  <div className="text-text-secondary">
                    <code className="text-xs bg-surface-2 px-1 py-0.5 rounded">{issue.element}</code>
                    {' '}references course <strong className="text-accent-warning">#{issue.referencedCourseId}</strong>
                  </div>
                  <div className="text-xs text-text-disabled mt-0.5 break-all">{issue.url}</div>
                  {issue.text && issue.text !== issue.url && (
                    <div className="text-xs text-text-tertiary mt-0.5 italic truncate">"{issue.text}"</div>
                  )}
                </div>
              </li>
            ))}
          </ul>
        </div>

        <div className="flex justify-end gap-3 px-6 py-4 border-t border-border-default bg-surface-1 rounded-b-lg">
          <button
            type="button"
            onClick={onGoBack}
            className="px-4 py-2 text-sm font-medium text-text-secondary bg-surface-0 border border-border-strong rounded-md hover:bg-surface-1"
          >
            Go Back
          </button>
          <button
            type="button"
            onClick={onSaveAnyway}
            className="px-4 py-2 text-sm font-medium text-white bg-amber-600 rounded-md hover:bg-amber-700"
          >
            Save Anyway
          </button>
        </div>
      </div>
    </div>
  );
}
