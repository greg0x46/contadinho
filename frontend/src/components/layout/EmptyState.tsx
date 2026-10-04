import type { ReactNode } from "react";

interface EmptyStateProps {
  /** What is missing, in one line: "Nenhuma conta ainda" or "Nada encontrado para essa busca". */
  title: string;
  /** What to do about it, or why — one quiet line. */
  hint?: ReactNode;
  /**
   * The way out ("Importar extrato"): one default (outlined) Button — the
   * variant is fixed, a `primary` one is drawn as default here, because the
   * page's own title-row action is the only filled button on a page. Omit it
   * when it would repeat that action (a page whose title row already says
   * "Nova pendência" does not repeat it in the empty list).
   */
  action?: ReactNode;
  /** Optional mark; most empty states read fine without one. */
  icon?: ReactNode;
  className?: string;
}

/**
 * The shared "there is nothing here" block. Copy should tell "nothing yet" apart from "no results
 * for this search" — the title says which, the hint what to do next.
 */
export function EmptyState({ title, hint, action, icon, className }: EmptyStateProps) {
  return (
    <div className={["empty-state", className].filter(Boolean).join(" ")}>
      {icon !== undefined && (
        <span className="empty-state-icon" aria-hidden="true">
          {icon}
        </span>
      )}
      <p className="empty-state-title">{title}</p>
      {hint !== undefined && <p className="empty-state-hint">{hint}</p>}
      {action !== undefined && <div className="empty-state-action">{action}</div>}
    </div>
  );
}
