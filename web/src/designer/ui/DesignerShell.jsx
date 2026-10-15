/* eslint-disable react/prop-types -- Paper JSX components don't declare propTypes */
/** The designer inside a DesignerProvider: header with steps, any notices, the current step, toasts. */
import { useEffect, useRef } from 'react';
import { StepHeader } from './components/StepHeader';
import { DesignerToast } from './components/Toast';
import { ChooseStep } from './steps/ChooseStep';
import { CustomizeStep } from './steps/CustomizeStep';
import { FinishStep } from './steps/FinishStep';
import { useDesigner } from './state/context';

export function DesignerShell({ initialKind = 'home', notices = null }) {
  const { state, doc } = useDesigner();
  const step = !doc ? 'choose' : state.step;
  const main = useRef(null);
  const first = useRef(true);
  // Moving between steps swaps the page's purpose: put keyboard and screen-reader users at its top.
  useEffect(() => {
    if (first.current) {
      first.current = false;
      return;
    }
    main.current?.focus();
  }, [step]);

  return (
    <div className="flex min-h-full flex-col bg-surface-0">
      <StepHeader />
      {notices && <div className="space-y-2 px-4 pt-4 sm:px-6">{notices}</div>}
      <div ref={main} tabIndex={-1} className="flex-1 outline-none" data-step={step}>
        {step === 'choose' && <ChooseStep initialKind={initialKind} />}
        {step === 'customize' && <CustomizeStep />}
        {step === 'finish' && <FinishStep />}
      </div>
      <DesignerToast />
    </div>
  );
}
