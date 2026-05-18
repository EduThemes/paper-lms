import React from 'react';
import { AlertTriangle } from 'lucide-react';
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
  DialogClose,
} from '@/components/ui/dialog';

// Warning dialog shown when authored content references another course
// the audience may not be able to reach. Open-state is driven by the
// callers passing a truthy `issues` array; closing the dialog (Esc,
// outside click, X button, or Go Back) all route to `onGoBack`.
export default function CrossCourseWarningDialog({ issues, onGoBack, onSaveAnyway }) {
  const open = Array.isArray(issues) && issues.length > 0;

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) onGoBack();
      }}
    >
      <DialogContent className="max-w-lg p-0 bg-surface-0 rounded-lg border border-border-default">
        <div className="flex items-center gap-3 px-6 py-4 border-b border-border-default">
          <div className="flex items-center justify-center w-10 h-10 bg-accent-warning/20 rounded-full shrink-0">
            <AlertTriangle size={20} className="text-accent-warning" />
          </div>
          <div>
            <DialogTitle className="text-lg font-semibold text-text-primary">
              Cross-Course References Detected
            </DialogTitle>
            <DialogDescription className="text-sm text-text-tertiary mt-0.5">
              This content contains links or images that reference other courses. Students enrolled only in this course may not be able to access them.
            </DialogDescription>
          </div>
        </div>

        <div className="px-6 py-4 max-h-60 overflow-y-auto">
          <ul className="space-y-2">
            {(issues || []).map((issue, i) => (
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
          <DialogClose asChild>
            <button
              type="button"
              onClick={onGoBack}
              className="px-4 py-2 text-sm font-medium text-text-secondary bg-surface-0 border border-border-strong rounded-md hover:bg-surface-1"
            >
              Go Back
            </button>
          </DialogClose>
          <button
            type="button"
            onClick={onSaveAnyway}
            className="px-4 py-2 text-sm font-medium text-white bg-amber-600 rounded-md hover:bg-amber-700"
          >
            Save Anyway
          </button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
