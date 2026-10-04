import type { ReactNode } from "react";

interface FormFieldProps {
  label: ReactNode;
  /** The id of the control, so the label is clickable and announced with it. */
  htmlFor?: string;
  /** For a group of controls (a radiogroup) that has no single input to label. */
  labelId?: string;
  /** A quiet one-line explanation under the control. */
  hint?: ReactNode;
  /** An inline validation message under the control; takes the hint's place. */
  error?: ReactNode;
  children: ReactNode;
}

/**
 * A labelled form field: label above, control full-width, then the hint or
 * the error. Replaces the ad-hoc `.filter-field` + `<label>` pattern in forms.
 * The hint and error get ids derived from `htmlFor` (`<id>-hint`,
 * `<id>-error`) so a control can point `aria-describedby` at them.
 */
export function FormField({ label, htmlFor, labelId, hint, error, children }: FormFieldProps) {
  return (
    <div className="form-field">
      {htmlFor ? (
        <label className="form-field-label" htmlFor={htmlFor}>
          {label}
        </label>
      ) : (
        <span className="form-field-label" id={labelId}>
          {label}
        </span>
      )}
      {children}
      {error ? (
        <div className="form-field-error" id={htmlFor ? `${htmlFor}-error` : undefined} role="alert">
          {error}
        </div>
      ) : hint ? (
        <div className="form-field-hint" id={htmlFor ? `${htmlFor}-hint` : undefined}>
          {hint}
        </div>
      ) : null}
    </div>
  );
}
