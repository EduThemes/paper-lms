import React, { useCallback, useEffect, useRef, useState } from 'react';
import { Circle, Square, Mic, Video, RotateCcw } from 'lucide-react';
import { Button } from '../ui/button';
import { MAX_RECORDING_SECONDS, pickRecorderMimeType, formatDuration } from './mediaHelpers';

/**
 * In-browser audio/video recorder (Canvas's "Record media", G002). Uses
 * getUserMedia + MediaRecorder; recordings stop automatically after
 * MAX_RECORDING_SECONDS. Hands the finished Blob to onRecorded.
 *
 * The app's Permissions-Policy allows camera/microphone for its own origin
 * (nginx.conf, middleware/security.go); the browser still asks the user.
 */
export default function MediaRecorderPanel({ onRecorded, disabled }) {
  const supported = typeof navigator !== 'undefined'
    && !!navigator.mediaDevices?.getUserMedia
    && typeof window !== 'undefined' && typeof window.MediaRecorder !== 'undefined';

  const [kind, setKind] = useState('video');
  const [status, setStatus] = useState('idle'); // idle | preview | recording | done
  const [error, setError] = useState(null);
  const [elapsed, setElapsed] = useState(0);
  const [result, setResult] = useState(null); // { blob, url, mimeType }

  const streamRef = useRef(null);
  const recorderRef = useRef(null);
  const chunksRef = useRef([]);
  const timerRef = useRef(null);
  const liveRef = useRef(null);

  const stopStream = useCallback(() => {
    streamRef.current?.getTracks().forEach((t) => t.stop());
    streamRef.current = null;
    if (timerRef.current) { clearInterval(timerRef.current); timerRef.current = null; }
  }, []);

  useEffect(() => () => {
    stopStream();
    if (result?.url) URL.revokeObjectURL(result.url);
  }, [stopStream, result]);

  const startPreview = async () => {
    setError(null);
    try {
      const stream = await navigator.mediaDevices.getUserMedia(
        kind === 'video' ? { video: true, audio: true } : { audio: true },
      );
      streamRef.current = stream;
      setStatus('preview');
      requestAnimationFrame(() => {
        if (liveRef.current && kind === 'video') {
          liveRef.current.srcObject = stream;
          liveRef.current.play?.().catch(() => {});
        }
      });
    } catch (e) {
      setError(e?.name === 'NotAllowedError'
        ? 'Camera or microphone access was blocked. Allow it in your browser settings and try again.'
        : 'No camera or microphone was found.');
    }
  };

  const stopRecording = useCallback(() => {
    if (recorderRef.current && recorderRef.current.state !== 'inactive') recorderRef.current.stop();
  }, []);

  const startRecording = () => {
    const stream = streamRef.current;
    if (!stream) return;
    const mimeType = pickRecorderMimeType(kind);
    let recorder;
    try {
      recorder = mimeType ? new window.MediaRecorder(stream, { mimeType }) : new window.MediaRecorder(stream);
    } catch {
      setError('This browser cannot record media.');
      return;
    }
    chunksRef.current = [];
    recorder.ondataavailable = (e) => { if (e.data?.size) chunksRef.current.push(e.data); };
    recorder.onstop = () => {
      const type = recorder.mimeType || mimeType || (kind === 'audio' ? 'audio/webm' : 'video/webm');
      const blob = new Blob(chunksRef.current, { type });
      stopStream();
      setResult({ blob, url: URL.createObjectURL(blob), mimeType: type });
      setStatus('done');
    };
    recorderRef.current = recorder;
    recorder.start(1000);
    setElapsed(0);
    setStatus('recording');
    const started = Date.now();
    timerRef.current = setInterval(() => {
      const secs = (Date.now() - started) / 1000;
      setElapsed(secs);
      if (secs >= MAX_RECORDING_SECONDS) stopRecording();
    }, 250);
  };

  const reset = () => {
    stopStream();
    if (result?.url) URL.revokeObjectURL(result.url);
    setResult(null);
    setElapsed(0);
    setStatus('idle');
  };

  if (!supported) {
    return <p className="text-sm text-muted-foreground">Recording is not supported in this browser. Upload a file instead.</p>;
  }

  return (
    <div className="space-y-3">
      {status === 'idle' && (
        <div className="flex flex-wrap items-center gap-2" role="radiogroup" aria-label="What to record">
          <Button type="button" variant={kind === 'video' ? 'default' : 'outline'} size="sm" role="radio" aria-checked={kind === 'video'} onClick={() => setKind('video')}>
            <Video className="size-4" /> Video
          </Button>
          <Button type="button" variant={kind === 'audio' ? 'default' : 'outline'} size="sm" role="radio" aria-checked={kind === 'audio'} onClick={() => setKind('audio')}>
            <Mic className="size-4" /> Audio only
          </Button>
          <Button type="button" size="sm" onClick={startPreview} disabled={disabled}>Start camera / microphone</Button>
        </div>
      )}

      {(status === 'preview' || status === 'recording') && (
        <div className="space-y-2">
          {kind === 'video'
            ? <video ref={liveRef} muted playsInline className="w-full max-h-64 rounded-md bg-black" aria-label="Camera preview" />
            : <p className="text-sm text-muted-foreground">Microphone ready.</p>}
          <div className="flex items-center gap-3">
            {status === 'preview' ? (
              <Button type="button" size="sm" onClick={startRecording}>
                <Circle className="size-4 fill-red-600 text-red-600" /> Record
              </Button>
            ) : (
              <Button type="button" size="sm" variant="destructive" onClick={stopRecording}>
                <Square className="size-4" /> Stop
              </Button>
            )}
            <span className="text-sm tabular-nums" aria-live="off">
              {formatDuration(elapsed)} / {formatDuration(MAX_RECORDING_SECONDS)}
            </span>
            <Button type="button" size="sm" variant="ghost" onClick={reset}>Cancel</Button>
          </div>
        </div>
      )}

      {status === 'done' && result && (
        <div className="space-y-2">
          {result.mimeType.startsWith('audio/')
            ? <audio controls src={result.url} className="w-full" />
            : <video controls src={result.url} className="w-full max-h-64 rounded-md bg-black" />}
          <div className="flex items-center gap-2">
            <Button type="button" size="sm" onClick={() => onRecorded(result.blob, result.mimeType, kind)} disabled={disabled}>
              Use this recording
            </Button>
            <Button type="button" size="sm" variant="outline" onClick={reset}>
              <RotateCcw className="size-4" /> Record again
            </Button>
          </div>
        </div>
      )}

      {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
    </div>
  );
}
