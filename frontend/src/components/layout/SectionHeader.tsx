import type { ReactNode } from "react";

/**
 * The header row of an `.accounts-section`-style block: a title and, on the
 * same line, whatever trailing bit belongs with it — a count, a "ver
 * todas" link. Title and trailing content always share one header instead
 * of stacking on separate lines.
 */
export function SectionHeader({ title, trailing }: { title: string; trailing?: ReactNode }) {
  return (
    <header className="accounts-section-header">
      <h2>{title}</h2>
      {trailing}
    </header>
  );
}
