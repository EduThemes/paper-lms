import React from 'react';
import { Upload, AlertCircle } from 'lucide-react';

/**
 * File-upload editor: no answer config needed beyond points.
 * Students upload a file; instructor grades manually.
 */
const FileUploadEditor = () => (
  <div className="border border-dashed border-border-strong rounded-lg p-4 bg-surface-1">
    <div className="flex items-start gap-3">
      <Upload className="w-5 h-5 text-text-tertiary flex-shrink-0 mt-0.5" />
      <div>
        <div className="text-sm font-medium text-text-primary mb-1">File upload question</div>
        <p className="text-xs text-text-secondary">
          Students will see a file picker and upload a single file. There is no answer key — you'll grade these manually
          from the submissions view.
        </p>
        <div className="mt-2 inline-flex items-center gap-1 text-xs text-accent-warning bg-accent-warning/10 border border-accent-warning/30 rounded px-2 py-1">
          <AlertCircle className="w-3.5 h-3.5" /> Manual grading required
        </div>
      </div>
    </div>
  </div>
);

export default FileUploadEditor;
