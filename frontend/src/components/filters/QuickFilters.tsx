export interface QuickFilter {
  key: string;
  label: string;
  /** Whether the filter this toggle stands for is currently on. */
  pressed: boolean;
  onToggle: () => void;
}

/**
 * One-tap shortcuts to the filters people reach for most ("Sem categoria",
 * "Entradas"), as small pill toggles under the toolbar row. They set the
 * same values as the filter panel — `pressed` is read from them — so they
 * are a faster way in, never a second state. Quiet on purpose: text-only,
 * and only the pressed one takes the primary colour.
 */
export function QuickFilters({ label, items }: { label: string; items: QuickFilter[] }) {
  return (
    <div className="filter-quick" role="group" aria-label={label}>
      {items.map((item) => (
        <button
          key={item.key}
          type="button"
          className="filter-quick-toggle"
          aria-pressed={item.pressed}
          onClick={item.onToggle}
        >
          {item.label}
        </button>
      ))}
    </div>
  );
}
