import type { ReactNode } from "react";

/**
 * The header row of a section-like block: a title and, on the same line,
 * whatever trailing bit belongs with it — a count, a "ver todas" link. Title
 * and trailing content always share one header instead of stacking on
 * separate lines. It styles itself (`.section-header` in layout.css), so it
 * works inside any bordered block, not only the accounts sections; `Section`
 * is the same header plus the surface around it.
 */
export function SectionHeader({ title, trailing }: { title: string; trailing?: ReactNode }) {
  return (
    <header className="section-header">
      <h2>{title}</h2>
      {trailing}
    </header>
  );
}
