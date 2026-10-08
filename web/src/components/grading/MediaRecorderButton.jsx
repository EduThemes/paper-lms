import React, { useEffect, useRef, useState } from 'react';
import { Mic, Video, Square, X } from 'lucide-react';

// Records audio or video in the browser (MediaRecorder) and hands the
// result to onRecorded as a File, ready to upload as a media submission
// or a comment attachment. Shows nothing when the browser cannot record.

const pickMimeType = (kind) => {
  if (typeof window === 'undefined' || typeof window.MediaRecorder === 'undefined') return '';
  const candidates = kind === 'video'
    ? ['video/webm;codecs=vp9,opus', 'video/webm', 'video/mp4']
    : ['audio/webm;codecs=opus', 'audio/webm', 'audio/mp4', 'audio/ogg'];
  return candidates.find((t) => window.MediaRecorder.isTypeSupported?.(t)) || '';
};

export const canRecordMedia = () =>
  typeof window !== 'undefined' &&
  typeof window.MediaRecorder !== 'undefined' &&
  !!navigator?.mediaDevices?.getUserMedia;

const extensionFor = (mime) => {
  if (mime.includes('mp4')) return 'mp4';
  if (mime.includes('ogg')) return 'ogg';
  return 'webm';
};

const MediaRecorderButton = ({ onRecorded, disabled = false, compact = false }) => {
  const [kind, setKind] = useState(null); // 'audio' | 'video' while recording
  const [error, setError] = useState(null);
  const [seconds, setSeconds] = useState(0);
  const recorderRef = useRef(null);
  const chunksRef = useRef([]);
  const streamRef = useRef(null);
  const previewRef = useRef(null);
  const cancelledRef = useRef(false);

  const stopStream = () => {
    streamRef.current?.getTracks().forEach((t) => t.stop());
    streamRef.current = null;
  };

  useEffect(() => () => stopStream(), []);

  useEffect(() => {
    if (!kind) return undefined;
    setSeconds(0);
    const timer = setInterval(() => setSeconds((s) => s + 1), 1000);
    return () => clearInterval(timer);
  }, [kind]);

  useEffect(() => {
    if (kind === 'video' && previewRef.current && streamRef.current) {
      previewRef.current.srcObject = streamRef.current;
    }
  }, [kind]);

  if (!canRecordMedia()) return null;

  const start = async (k) => {
    setError(null);
    try {
      const stream = await navigator.mediaDevices.getUserMedia(k === 'video' ? { audio: true, video: true } : { audio: true });
      streamRef.current = stream;
      const mimeType = pickMimeType(k);
      const recorder = new window.MediaRecorder(stream, mimeType ? { mimeType } : undefined);
      chunksRef.current = [];
      cancelledRef.current = false;
      recorder.ondataavailable = (e) => { if (e.data && e.data.size > 0) chunksRef.current.push(e.data); };
      recorder.onstop = () => {
        stopStream();
        setKind(null);
        if (cancelledRef.current) return;
        const type = (recorder.mimeType || mimeType || (k === 'video' ? 'video/webm' : 'audio/webm')).split(';')[0];
        const blob = new Blob(chunksRef.current, { type });
        const stamp = new Date().toISOString().replace(/[:.]/g, '-');
        const file = new File([blob], `${k}-recording-${stamp}.${extensionFor(type)}`, { type });
        onRecorded(file);
      };
      recorderRef.current = recorder;
      recorder.start();
      setKind(k);
    } catch (err) {
      stopStream();
      setError(err?.name === 'NotAllowedError' ? 'Permission to use the microphone or camera was denied.' : 'Recording is not available.');
    }
  };

  const stop = () => recorderRef.current?.state === 'recording' && recorderRef.current.stop();
  const cancel = () => { cancelledRef.current = true; stop(); };

  const btn = 'inline-flex items-center gap-1 px-2 py-1 rounded border text-xs transition-colors disabled:opacity-50';

  return (
    <div className="inline-flex flex-col gap-1">
      {kind ? (
        <div className="flex items-center gap-2">
          <span className="inline-flex items-center gap-1 text-xs text-accent-danger font-medium" role="status">
            <span className="h-2 w-2 rounded-full bg-accent-danger animate-pulse" aria-hidden="true" />
            Recording {kind} {Math.floor(seconds / 60)}:{String(seconds % 60).padStart(2, '0')}
          </span>
          <button type="button" onClick={stop} className={`${btn} border-accent-danger/40 text-accent-danger hover:bg-accent-danger/10`}>
            <Square className="w-3 h-3" /> Stop
          </button>
          <button type="button" onClick={cancel} className={`${btn} border-border-default text-text-secondary hover:bg-surface-2`} aria-label="Discard recording">
            <X className="w-3 h-3" />
          </button>
        </div>
      ) : (
        <div className="flex items-center gap-2">
          <button type="button" disabled={disabled} onClick={() => start('audio')} className={`${btn} border-border-default text-text-secondary hover:bg-surface-2`}>
            <Mic className="w-3 h-3" /> {compact ? '' : 'Record audio'}
            {compact && <span className="sr-only">Record audio</span>}
          </button>
          <button type="button" disabled={disabled} onClick={() => start('video')} className={`${btn} border-border-default text-text-secondary hover:bg-surface-2`}>
            <Video className="w-3 h-3" /> {compact ? '' : 'Record video'}
            {compact && <span className="sr-only">Record video</span>}
          </button>
        </div>
      )}
      {kind === 'video' && (
        <video ref={previewRef} autoPlay muted playsInline className="w-48 rounded border border-border-default" />
      )}
      {error && <p className="text-xs text-accent-danger">{error}</p>}
    </div>
  );
};

export default MediaRecorderButton;
