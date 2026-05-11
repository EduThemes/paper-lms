import { useCallback, useEffect, useRef, useState } from 'react';

/**
 * useTextToSpeech — thin, ergonomic wrapper around window.speechSynthesis.
 * Cancels any in-flight utterance on unmount so navigation never leaves a voice talking.
 */
const pickDefaultVoice = (voices) => {
  if (!voices || voices.length === 0) return null;
  const enUSFemale = voices.find(
    (v) => v.lang === 'en-US' && /female|samantha|victoria|allison|ava|zira/i.test(v.name)
  );
  const enUS = voices.find((v) => v.lang === 'en-US');
  return enUSFemale || enUS || voices[0];
};

const useTextToSpeech = () => {
  const supported = typeof window !== 'undefined' && 'speechSynthesis' in window;
  const synthRef = useRef(supported ? window.speechSynthesis : null);

  const [voices, setVoices] = useState([]);
  const [voice, setVoice] = useState(null);
  const [rate, setRate] = useState(0.95); // a touch slower for K-2
  const [pitch, setPitch] = useState(1);
  const [speaking, setSpeaking] = useState(false);
  const [paused, setPaused] = useState(false);

  // Populate voices (Chrome fires `voiceschanged` async).
  useEffect(() => {
    if (!supported) return undefined;
    const synth = synthRef.current;
    const load = () => {
      const list = synth.getVoices();
      setVoices(list);
      setVoice((current) => current || pickDefaultVoice(list));
    };
    load();
    synth.addEventListener?.('voiceschanged', load);
    return () => synth.removeEventListener?.('voiceschanged', load);
  }, [supported]);

  // Cancel any in-flight utterance on unmount.
  useEffect(() => {
    if (!supported) return undefined;
    const synth = synthRef.current;
    return () => synth.cancel();
  }, [supported]);

  const speak = useCallback(
    (text, opts = {}) => {
      if (!supported || !text) return;
      const synth = synthRef.current;
      synth.cancel(); // flush any prior utterance
      const u = new SpeechSynthesisUtterance(String(text));
      u.voice = opts.voice ?? voice ?? null;
      u.rate = opts.rate ?? rate;
      u.pitch = opts.pitch ?? pitch;
      u.onstart = () => {
        setSpeaking(true);
        setPaused(false);
      };
      u.onend = () => {
        setSpeaking(false);
        setPaused(false);
      };
      u.onerror = () => {
        setSpeaking(false);
        setPaused(false);
      };
      synth.speak(u);
    },
    [supported, voice, rate, pitch]
  );

  const stop = useCallback(() => {
    if (!supported) return;
    synthRef.current.cancel();
    setSpeaking(false);
    setPaused(false);
  }, [supported]);

  const pause = useCallback(() => {
    if (!supported) return;
    synthRef.current.pause();
    setPaused(true);
  }, [supported]);

  const resume = useCallback(() => {
    if (!supported) return;
    synthRef.current.resume();
    setPaused(false);
  }, [supported]);

  return {
    speak,
    stop,
    pause,
    resume,
    speaking,
    paused,
    supported,
    voices,
    voice,
    setVoice,
    rate,
    setRate,
    pitch,
    setPitch,
  };
};

export default useTextToSpeech;
