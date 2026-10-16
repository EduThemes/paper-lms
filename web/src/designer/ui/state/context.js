import { createContext, useContext } from 'react';

/** Shared by DesignerProvider (state/DesignerContext.jsx) and every designer component. */
export const DesignerContext = createContext(null);

export function useDesigner() {
  const ctx = useContext(DesignerContext);
  if (!ctx) throw new Error('useDesigner must be used inside <DesignerProvider>');
  return ctx;
}
