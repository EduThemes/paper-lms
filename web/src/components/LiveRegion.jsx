import React, { useState, useCallback, createContext, useContext } from 'react';

const LiveRegionContext = createContext(null);

export const useLiveRegion = () => {
  const context = useContext(LiveRegionContext);
  if (!context) {
    throw new Error('useLiveRegion must be used within a LiveRegionProvider');
  }
  return context;
};

export const LiveRegionProvider = ({ children }) => {
  const [message, setMessage] = useState('');
  const [politeness, setPoliteness] = useState('polite');

  const announce = useCallback((text, level = 'polite') => {
    setPoliteness(level);
    // Clear and re-set to ensure screen readers pick up repeated messages
    setMessage('');
    requestAnimationFrame(() => {
      setMessage(text);
    });
  }, []);

  return (
    <LiveRegionContext.Provider value={{ announce }}>
      {children}
      <div
        aria-live={politeness}
        aria-atomic="true"
        role={politeness === 'assertive' ? 'alert' : 'status'}
        className="sr-only"
      >
        {message}
      </div>
    </LiveRegionContext.Provider>
  );
};

export default LiveRegionProvider;
