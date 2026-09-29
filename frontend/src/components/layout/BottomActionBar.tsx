import type { ReactNode } from "react";

interface BottomActionBarProps {
  /**
   * Page-level actions only ("Nova pendência"), never a record's own actions
   * (editar/excluir) — those stay with the record, not the page chrome.
   */
  children: ReactNode;
}

/**
 * A fixed, compact command bar for a page's actions on a phone — the mobile
 * home for what a `Page` with `compactMobileHeader` no longer shows in its
 * header. It renders whatever it is given at their natural width and lets
 * them share the row; a single child stretches to fill it so a lone action
 * still reads as the row's whole point, not one item lost in a bar sized for
 * many. Built to hold a few equally-weighted actions this way without a
 * rewrite — grouping extras into a "•••" overflow is a later addition, not
 * something this shape forecloses.
 */
export function BottomActionBar({ children }: BottomActionBarProps) {
  return (
    <div className="bottom-action-bar">
      <div className="bottom-action-bar-row">{children}</div>
    </div>
  );
}
