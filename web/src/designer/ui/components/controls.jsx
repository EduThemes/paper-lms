/* eslint-disable react/prop-types -- Paper JSX components don't declare propTypes */
/**
 * Small form controls for the designer, built on Paper's shadcn primitives and Tailwind tokens. Native
 * elements underneath (select, radio, checkbox), so keyboard and screen-reader behaviour comes for free.
 */
import { forwardRef, useId } from 'react';
import { AlertTriangle, CheckCircle2, Info, XCircle } from 'lucide-react';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { cn } from '@/lib/utils';

export function Spinner({ label, className }) {
  return (
    <span role="status" className={cn('inline-flex items-center gap-2 text-text-tertiary', className)}>
      <svg className="h-5 w-5 animate-spin" viewBox="0 0 24 24" fill="none" aria-hidden="true">
        <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
        <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" />
      </svg>
      <span className="sr-only">{label}</span>
    </span>
  );
}

const ALERT_STYLES = {
  info: 'border-accent-info/30 bg-accent-info/5 text-text-primary',
  success: 'border-accent-success/30 bg-accent-success/5 text-text-primary',
  warning: 'border-accent-warning/40 bg-accent-warning/10 text-text-primary',
  error: 'border-accent-danger/30 bg-accent-danger/5 text-text-primary',
};
const ALERT_ICONS = { info: Info, success: CheckCircle2, warning: AlertTriangle, error: XCircle };
const ALERT_ICON_COLORS = { info: 'text-accent-info', success: 'text-accent-success', warning: 'text-accent-warning', error: 'text-accent-danger' };

/** A boxed message. `role` defaults to "alert" for errors, none otherwise (wrap in a live region if needed). */
export function Notice({ variant = 'info', title, children, className, role, action }) {
  const Icon = ALERT_ICONS[variant] ?? Info;
  return (
    <div role={role ?? (variant === 'error' ? 'alert' : undefined)} className={cn('flex gap-3 rounded-md border p-3 text-sm', ALERT_STYLES[variant], className)}>
      <Icon className={cn('mt-0.5 h-4 w-4 shrink-0', ALERT_ICON_COLORS[variant])} aria-hidden="true" />
      <div className="min-w-0 flex-1 space-y-1">
        {title && <p className="font-semibold">{title}</p>}
        {children && <div>{children}</div>}
        {action && <div className="pt-1">{action}</div>}
      </div>
    </div>
  );
}

/** Label + input + hint/error, wired with aria-describedby. */
export const TextField = forwardRef(function TextField(
  { label, hint, error, value, onChange, required, type = 'text', className, inputClassName, id: idProp, ...rest },
  ref,
) {
  const autoId = useId();
  const id = idProp ?? autoId;
  const hintId = `${id}-hint`;
  const errId = `${id}-err`;
  const describedBy = [error ? errId : null, hint ? hintId : null].filter(Boolean).join(' ') || undefined;
  return (
    <div className={cn('space-y-1.5', className)}>
      <Label htmlFor={id} className="text-text-primary">
        {label}
        {required && <span aria-hidden="true" className="text-accent-danger"> *</span>}
      </Label>
      <Input
        ref={ref}
        id={id}
        type={type}
        value={value ?? ''}
        onChange={(e) => onChange(e.target.value)}
        aria-invalid={error ? true : undefined}
        aria-required={required || undefined}
        aria-describedby={describedBy}
        className={cn('bg-surface-0 text-text-primary', inputClassName)}
        {...rest}
      />
      {error && (
        <p id={errId} className="text-xs text-accent-danger">
          {error}
        </p>
      )}
      {hint && (
        <p id={hintId} className="text-xs text-text-secondary">
          {hint}
        </p>
      )}
    </div>
  );
});

export function TextAreaField({ label, hint, value, onChange, rows = 3, className, labelHidden, id: idProp, ...rest }) {
  const autoId = useId();
  const id = idProp ?? autoId;
  return (
    <div className={cn('space-y-1.5', className)}>
      <Label htmlFor={id} className={cn('text-text-primary', labelHidden && 'sr-only')}>
        {label}
      </Label>
      <textarea
        id={id}
        rows={rows}
        value={value ?? ''}
        onChange={(e) => onChange(e.target.value)}
        aria-describedby={hint ? `${id}-hint` : undefined}
        className="flex w-full rounded-md border border-input bg-surface-0 px-3 py-2 text-sm text-text-primary ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
        {...rest}
      />
      {hint && (
        <p id={`${id}-hint`} className="text-xs text-text-secondary">
          {hint}
        </p>
      )}
    </div>
  );
}

/** A native select. `options`: [{ value, label, disabled? }] or groups [{ group, options }]. */
export function SelectField({ label, hint, value, onChange, options, className, labelHidden, id: idProp, placeholder, ...rest }) {
  const autoId = useId();
  const id = idProp ?? autoId;
  const renderOption = (o) => (
    <option key={o.value} value={o.value} disabled={o.disabled}>
      {o.label}
    </option>
  );
  return (
    <div className={cn('space-y-1.5', className)}>
      <Label htmlFor={id} className={cn('text-text-primary', labelHidden && 'sr-only')}>
        {label}
      </Label>
      <select
        id={id}
        value={value ?? ''}
        onChange={(e) => onChange(e.target.value)}
        aria-describedby={hint ? `${id}-hint` : undefined}
        className="h-10 w-full rounded-md border border-input bg-surface-0 px-3 text-sm text-text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
        {...rest}
      >
        {placeholder !== undefined && (
          <option value="" disabled>
            {placeholder}
          </option>
        )}
        {options.map((o) =>
          o.group ? (
            <optgroup key={o.group} label={o.group}>
              {o.options.map(renderOption)}
            </optgroup>
          ) : (
            renderOption(o)
          ),
        )}
      </select>
      {hint && (
        <p id={`${id}-hint`} className="text-xs text-text-secondary">
          {hint}
        </p>
      )}
    </div>
  );
}

/** A segmented control: a radio group that looks like toggle buttons. `options`: [{ value, label, icon? }]. */
export function Segmented({ legend, legendHidden, name, value, onChange, options, size = 'md', className }) {
  return (
    <fieldset className={cn('min-w-0', className)}>
      <legend className={cn('mb-1.5 text-sm font-medium text-text-primary', legendHidden && 'sr-only')}>{legend}</legend>
      <div className="inline-flex flex-wrap rounded-md border border-border bg-surface-1 p-0.5">
        {options.map((o) => {
          const Icon = o.icon;
          const checked = value === o.value;
          return (
            <label
              key={o.value}
              className={cn(
                'relative inline-flex cursor-pointer items-center gap-1.5 rounded-[5px] font-medium transition-colors',
                size === 'sm' ? 'px-2.5 py-1 text-xs' : 'px-3 py-1.5 text-sm',
                checked ? 'bg-surface-0 text-text-primary shadow-xs' : 'text-text-secondary hover:text-text-primary',
                'has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-ring',
              )}
            >
              <input type="radio" className="sr-only" name={name} value={o.value} checked={checked} onChange={() => onChange(o.value)} />
              {Icon && <Icon className="h-4 w-4" aria-hidden="true" />}
              <span>{o.label}</span>
            </label>
          );
        })}
      </div>
    </fieldset>
  );
}

/** A labelled switch (a checkbox with role="switch"). */
export function Switch({ label, hint, checked, onChange, disabled, className, id: idProp }) {
  const autoId = useId();
  const id = idProp ?? autoId;
  return (
    <div className={cn('flex items-start gap-3', className)}>
      <span className="relative mt-0.5 inline-flex h-5 w-9 shrink-0">
        <input
          id={id}
          type="checkbox"
          role="switch"
          className="peer absolute inset-0 z-10 h-full w-full cursor-pointer opacity-0 disabled:cursor-not-allowed"
          checked={checked}
          disabled={disabled}
          onChange={(e) => onChange(e.target.checked)}
          aria-describedby={hint ? `${id}-hint` : undefined}
        />
        <span
          aria-hidden="true"
          className="pointer-events-none h-5 w-9 rounded-full bg-border-strong transition-colors peer-checked:bg-brand-600 peer-focus-visible:ring-2 peer-focus-visible:ring-ring peer-focus-visible:ring-offset-2 peer-disabled:opacity-50"
        />
        <span
          aria-hidden="true"
          className="pointer-events-none absolute left-0.5 top-0.5 h-4 w-4 rounded-full bg-white shadow transition-transform peer-checked:translate-x-4"
        />
      </span>
      <span className="min-w-0">
        <label htmlFor={id} className="text-sm font-medium text-text-primary">
          {label}
        </label>
        {hint && (
          <span id={`${id}-hint`} className="block text-xs text-text-secondary">
            {hint}
          </span>
        )}
      </span>
    </div>
  );
}

/** A labelled checkbox. */
export function Checkbox({ label, checked, onChange, disabled, className, id: idProp, value }) {
  const autoId = useId();
  const id = idProp ?? autoId;
  return (
    <div className={cn('flex items-start gap-2', className)}>
      <input
        id={id}
        type="checkbox"
        value={value}
        className="mt-0.5 h-4 w-4 rounded border-border-strong text-brand-600 focus-visible:ring-2 focus-visible:ring-ring"
        checked={checked}
        disabled={disabled}
        onChange={(e) => onChange(e.target.checked)}
      />
      <label htmlFor={id} className="text-sm text-text-primary">
        {label}
      </label>
    </div>
  );
}

/** A heading + small description used at the top of panels. */
export function PanelHeading({ as: Tag = 'h2', id, children, description, className }) {
  return (
    <div className={cn('space-y-1', className)}>
      <Tag id={id} className="text-base font-semibold text-text-primary">
        {children}
      </Tag>
      {description && <p className="text-sm text-text-secondary">{description}</p>}
    </div>
  );
}
