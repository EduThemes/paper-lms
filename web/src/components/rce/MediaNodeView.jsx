import React, { useEffect, useRef, useState } from 'react';
import { NodeViewWrapper } from '@tiptap/react';
import { Subtitles } from 'lucide-react';
import { fetchMediaTracks, applyTracks, invalidateMediaTracks } from '../../utils/mediaEnhance';
import MediaCaptionsDialog from './MediaCaptionsDialog';

/**
 * Editor view of a `media` node (G002): the player itself (with its caption
 * tracks) plus a Captions button for media objects, which opens the caption
 * manager.
 */
export default function MediaNodeView({ node, selected, editor }) {
  const { src, mediaId, mediaType, title, width } = node.attrs;
  const playerRef = useRef(null);
  const [captionsOpen, setCaptionsOpen] = useState(false);
  const [tracksVersion, setTracksVersion] = useState(0);

  useEffect(() => {
    if (!mediaId || !playerRef.current) return undefined;
    let cancelled = false;
    fetchMediaTracks(mediaId).then((tracks) => {
      if (cancelled || !playerRef.current) return;
      playerRef.current.querySelectorAll('track').forEach((t) => t.remove());
      applyTracks(playerRef.current, tracks);
    });
    return () => { cancelled = true; };
  }, [mediaId, tracksVersion]);

  const Player = mediaType === 'audio' ? 'audio' : 'video';

  return (
    <NodeViewWrapper
      className={`rce-media my-3 inline-block max-w-full rounded-md ${selected ? 'ring-2 ring-ring' : ''}`}
      data-drag-handle=""
    >
      <Player
        ref={playerRef}
        src={src || undefined}
        controls
        preload="metadata"
        title={title || undefined}
        width={Player === 'video' ? (width || undefined) : undefined}
        style={Player === 'video' ? { maxWidth: '100%' } : { width: '100%', minWidth: '18rem' }}
        contentEditable={false}
      />
      <div className="mt-1 flex items-center gap-2 text-xs text-muted-foreground" contentEditable={false}>
        <span className="truncate">{title || (Player === 'audio' ? 'Audio' : 'Video')}</span>
        {mediaId && editor?.isEditable && (
          <button
            type="button"
            onClick={() => setCaptionsOpen(true)}
            className="inline-flex items-center gap-1 rounded border border-input px-1.5 py-0.5 hover:bg-accent"
          >
            <Subtitles className="size-3.5" aria-hidden="true" /> Captions
          </button>
        )}
      </div>
      {captionsOpen && (
        <MediaCaptionsDialog
          mediaId={mediaId}
          title={title}
          onClose={() => setCaptionsOpen(false)}
          onChanged={() => { invalidateMediaTracks(mediaId); setTracksVersion((v) => v + 1); }}
        />
      )}
    </NodeViewWrapper>
  );
}
