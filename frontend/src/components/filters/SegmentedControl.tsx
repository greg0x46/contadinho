import { Segmented } from "antd";

export interface SegmentedOption<Value extends string> {
  value: Value;
  label: string;
  disabled?: boolean;
}

/**
 * A short list of mutually exclusive choices, read at a glance: "Todas /
 * Entradas / Saídas". Exactly one value is selected at any time, so it never
 * represents "nothing chosen" — give it an explicit "all" option for that.
 *
 * antd's Segmented provides the radio semantics and keyboard handling; the
 * `segmented-control` class flattens its look and lifts the contrast of the
 * selected item so it reads without hunting.
 */
export function SegmentedControl<Value extends string>({
  id,
  label,
  value,
  options,
  onChange,
  disabled = false,
}: {
  id?: string;
  /** Accessible name of the group; the visible label is the caller's. */
  label: string;
  value: Value;
  options: SegmentedOption<Value>[];
  onChange: (value: Value) => void;
  disabled?: boolean;
}) {
  return (
    <Segmented
      id={id}
      block
      aria-label={label}
      className="segmented-control"
      value={value}
      options={options}
      disabled={disabled}
      onChange={(next) => onChange(next as Value)}
    />
  );
}
