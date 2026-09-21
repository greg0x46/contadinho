import type { ReactNode } from "react";

interface ListToolbarProps {
  label: string;
  /** Narrowing controls: search and filters. */
  start?: ReactNode;
  /** Shaping controls: grouping and sorting. */
  end?: ReactNode;
  /** Active-filter chips, shown as a second row when there are any. */
  chips?: ReactNode;
}

/**
 * The collection controls, immediately above the list they act on: what
 * narrows the data (search, filters) on the left, what shapes it (group by,
 * sort) on the right. Nothing here creates data or moves the page's period —
 * those are page actions and page context, and live in the Page header.
 *
 * On a phone the search takes a whole row and the remaining controls share
 * one compact row, so nothing scrolls sideways.
 */
export function ListToolbar({ label, start, end, chips }: ListToolbarProps) {
  return (
    <section className="list-toolbar" aria-label={label}>
      <div className="list-toolbar-row">
        {start !== undefined && <div className="list-toolbar-start">{start}</div>}
        {end !== undefined && <div className="list-toolbar-end">{end}</div>}
      </div>
      {chips}
    </section>
  );
}
