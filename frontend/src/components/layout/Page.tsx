import { LeftOutlined } from "@ant-design/icons";
import { PageContainer } from "@ant-design/pro-layout";
import type { ReactNode } from "react";
import { Link } from "react-router-dom";

import { useCompactScreen } from "../shared/useCompactScreen";

interface PageProps {
  title: string;
  /** A link back to where this page was reached from, e.g. a detail page's
   *  way back to its list. Rendered right under the title, not as a page
   *  action — it navigates away rather than acting on this page. */
  back?: ReactNode;
  /**
   * Same destination as `back`, as a route: when `compactMobileHeader`
   * collapses the header, this turns the collapsed title itself into the
   * back control (a real link, with a leading chevron) instead of dropping
   * navigation entirely. Without it, the collapsed title stays static text.
   */
  backTo?: string;
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
  /**
   * On a phone, drop the hero header — description, back link and actions —
   * for a single quiet title line, so the list starts almost immediately.
   * `context` survives the collapse (it's page-wide state, not chrome, so a
   * page that has one still needs it reachable). From `md` up this is
   * ignored and the header renders as usual; a page opting in still owns
   * getting its actions in front of the user some other way (see
   * BottomActionBar).
   */
  compactMobileHeader?: boolean;
  /**
   * Set when a BottomActionBar is mounted on a phone, so the page reserves
   * room at the bottom of its scroll area for it (shared rule in layout.css)
   * instead of every page copying its own padding-bottom hack.
   */
  hasBottomActionBar?: boolean;
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
 * touch target, then the actions at full width — unless `compactMobileHeader`
 * collapses it instead.
 */
export function Page({
  title,
  back,
  backTo,
  description,
  context,
  actions,
  tabs,
  compactMobileHeader,
  hasBottomActionBar,
  className,
  children,
}: PageProps) {
  const compact = useCompactScreen();
  const collapsed = compactMobileHeader === true && compact;
  const hasContext = context !== undefined;
  const hasVisibleActions = !collapsed && actions !== undefined;

  return (
    <PageContainer
      // PageHeader wraps its title in a span; the page's name is the one
      // heading assistive tech should land on, so it is a real h1 either way.
      title={
        <div className={["page-heading", collapsed ? "page-heading-collapsed" : ""].filter(Boolean).join(" ")}>
          <h1 className={["page-title", collapsed ? "page-title-collapsed" : ""].filter(Boolean).join(" ")}>
            {collapsed && backTo !== undefined ? (
              <Link to={backTo} className="page-title-collapsed-link">
                <LeftOutlined aria-hidden="true" />
                {title}
              </Link>
            ) : (
              title
            )}
          </h1>
          {!collapsed && back !== undefined && <div className="page-back">{back}</div>}
        </div>
      }
      subTitle={collapsed ? undefined : description}
      className={[
        "app-page",
        collapsed ? "app-page-collapsed-header" : "",
        compact && hasBottomActionBar ? "app-page-has-bottom-bar" : "",
        className,
      ]
        .filter(Boolean)
        .join(" ")}
      extra={
        hasContext || hasVisibleActions ? (
          <div className="page-header-slots">
            {hasContext && <div className="page-context">{context}</div>}
            {hasVisibleActions && <div className="page-actions">{actions}</div>}
          </div>
        ) : undefined
      }
    >
      {tabs !== undefined && <div className="page-tabs">{tabs}</div>}
      {children}
    </PageContainer>
  );
}
