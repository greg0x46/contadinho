import { PageContainer } from "@ant-design/pro-layout";
import type { ReactNode } from "react";

interface PageProps {
  title: string;
  /** One line under the title; the only explanatory text a page header carries. */
  description?: string;
  /**
   * Page context: a control that changes the whole page at once (the period
   * navigator moves cards, totals, charts and lists together). It is not a
   * filter of any one list, so it lives in the header, not in a toolbar.
   */
  context?: ReactNode;
  /** Page actions: creation and other page-wide commands ("Novo lançamento"). */
  actions?: ReactNode;
  /** Top-level views of the page, rendered right under the header. */
  tabs?: ReactNode;
  className?: string;
  children: ReactNode;
}

/**
 * The shell every screen composes: title + description on the left, page
 * context and page actions on the right, optional tabs, then the content.
 * Collection controls (search, filters, grouping, sorting) never go here —
 * they belong to a ListToolbar sitting right above the list they change.
 *
 * On a phone the header stacks: title, then the context on a full-width
 * touch target, then the actions at full width.
 */
export function Page({ title, description, context, actions, tabs, className, children }: PageProps) {
  const hasSlots = context !== undefined || actions !== undefined;
  return (
    <PageContainer
      // PageHeader wraps its title in a span; the page's name is the one
      // heading assistive tech should land on, so it is a real h1.
      title={<h1 className="page-title">{title}</h1>}
      subTitle={description}
      className={["app-page", className].filter(Boolean).join(" ")}
      extra={
        hasSlots ? (
          <div className="page-header-slots">
            {context !== undefined && <div className="page-context">{context}</div>}
            {actions !== undefined && <div className="page-actions">{actions}</div>}
          </div>
        ) : undefined
      }
    >
      {tabs !== undefined && <div className="page-tabs">{tabs}</div>}
      {children}
    </PageContainer>
  );
}
