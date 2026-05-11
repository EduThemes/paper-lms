import React from 'react';
import { Volume2, Square } from 'lucide-react';
import { Button } from '../ui/button';
import useTextToSpeech from '../../hooks/useTextToSpeech';

/**
 * ReadAloudButton — large, round, kid-tappable TTS toggle.
 * Renders nothing if the browser lacks speechSynthesis.
 */
const ReadAloudButton = ({ text, className = '' }) => {
  const { speak, stop, speaking, supported } = useTextToSpeech();
  if (!supported) return null;

  const onClick = () => (speaking ? stop() : speak(text));
  const Icon = speaking ? Square : Volume2;

  return (
    <Button
      type="button"
      onClick={onClick}
      aria-label={speaking ? 'Stop reading' : 'Read this aloud'}
      aria-pressed={speaking}
      className={`h-14 w-14 rounded-full p-0 shadow-md ${className}`}
    >
      <Icon className="h-7 w-7" aria-hidden="true" />
    </Button>
  );
};

export default ReadAloudButton;
