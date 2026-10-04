import type { ReactNode } from "react";
import { useId } from "react";

interface SectionProps {
  title: string;
  /**
   * Something quiet that belongs with the title, on the same line and at
   * its right edge — a "ver todas" link, a figure. Never a count used as a
   * subtitle.
   */
  trailing?: ReactNode;
  /** Drops the body's side padding so a list or table can run edge to edge. */
  flush?: boolean;
  className?: string;
  id?: string;
  children: ReactNode;
}

/**
 * A titled block of a page: a small heading with an optional quiet trailing
 * bit on the same line, a hairline, then the content. It is a white surface
 * like every other (an edge-to-edge sheet on a phone, a hairline-bordered
 * block from `md` up) — not a card, so it must not be nested inside a
 * `DataCard` or another `Section`.
 */
export function Section({ title, trailing, flush, className, id, children }: SectionProps) {
  const headingId = useId();
  return (
    <section
      id={id}
      className={["section", flush ? "section-flush" : "", className].filter(Boolean).join(" ")}
      aria-labelledby={headingId}
    >
      <header className="section-header">
        <h2 id={headingId}>{title}</h2>
        {trailing !== undefined && <div className="section-trailing">{trailing}</div>}
      </header>
      <div className="section-body">{children}</div>
    </section>
  );
}
