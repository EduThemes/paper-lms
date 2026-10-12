import React from 'react';
import { X, PlayCircle, Trash2 } from 'lucide-react';
import {
  useConferenceRecordings,
  useDeleteConferenceRecording,
  recordingLabel,
  safeHref,
} from '../../services/api_groups_conferences';

// ConferenceRecordings lists a conference's provider recordings (G098):
// playback links for everyone, delete for instructors.
export default function ConferenceRecordings({ conference, isTeacher, onClose }) {
  const query = useConferenceRecordings(conference.id);
  const del = useDeleteConferenceRecording(conference.id);
  // Provider links only: never render a non-http(s) href.
  const recordings = (query.data || []).map((rec) => ({
    ...rec,
    playback_url: safeHref(rec.playback_url),
    formats: (rec.formats || []).filter((f) => safeHref(f.url)),
  }));

  const handleDelete = (rec) => {
    if (!window.confirm('Delete this recording? This cannot be undone.')) return;
    del.mutate(rec.id);
  };

  return (
    <div className="mt-6 bg-surface-0 rounded-lg shadow p-6">
      <div className="flex items-center justify-between mb-4">
        <h3 className="font-semibold">Recordings - {conference.title}</h3>
        <button onClick={onClose} className="text-text-disabled hover:text-text-secondary" aria-label="Close recordings">
          <X className="w-5 h-5" />
        </button>
      </div>
      {query.isLoading && <p className="text-sm text-text-tertiary">Loading recordings…</p>}
      {query.isError && (
        <p role="alert" className="text-sm text-accent-danger">{query.error?.message || 'Could not load recordings.'}</p>
      )}
      {del.isError && (
        <p role="alert" className="text-sm text-accent-danger">{del.error?.message || 'Could not delete the recording.'}</p>
      )}
      {!query.isLoading && !query.isError && recordings.length === 0 && (
        <p className="text-sm text-text-tertiary">No recordings available for this conference.</p>
      )}
      {recordings.length > 0 && (
        <ul className="divide-y">
          {recordings.map((rec) => (
            <li key={rec.id} className="py-2 flex items-center justify-between gap-3">
              <div className="min-w-0">
                <p className="text-sm text-text-primary truncate">{rec.title || conference.title}</p>
                <p className="text-xs text-text-tertiary">{recordingLabel(rec)}</p>
                {rec.formats?.length > 1 && (
                  <p className="text-xs text-text-tertiary">
                    {rec.formats.map((f, i) => (
                      <React.Fragment key={`${f.type}-${f.url}`}>
                        {i > 0 && ' · '}
                        <a href={f.url} target="_blank" rel="noopener noreferrer" className="text-brand-600 hover:underline">{f.type}</a>
                      </React.Fragment>
                    ))}
                  </p>
                )}
              </div>
              <div className="flex items-center gap-2 flex-shrink-0">
                {rec.playback_url && (
                  <a
                    href={rec.playback_url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center gap-1 text-sm text-brand-600 hover:underline"
                  >
                    <PlayCircle className="w-4 h-4" />
                    Watch
                  </a>
                )}
                {isTeacher && (
                  <button
                    onClick={() => handleDelete(rec)}
                    disabled={del.isPending}
                    className="p-1 text-text-disabled hover:text-accent-danger disabled:opacity-50"
                    title="Delete recording"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                )}
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
