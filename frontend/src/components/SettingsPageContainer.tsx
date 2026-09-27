import type { ReactNode } from "react";
import { Link, useLocation } from "react-router-dom";
import { Typography } from "antd";

import { BottomActionBar, Page } from "./layout";
import { useCompactScreen } from "./shared/useCompactScreen";

interface SettingsPageContainerProps {
  title: string;
  /** One line under the title — mirrors Page's own `description`. */
  subTitle?: string;
  /** A longer explanation of the section, shown above its content. */
  content?: ReactNode;
  /** Page-wide actions (e.g. "Nova categoria"). Rendered in Page's actions slot. */
  extra?: ReactNode;
  /** Forwarded to Page — see its own doc comment. */
  compactMobileHeader?: boolean;
  children: ReactNode;
}

/**
 * The shell every settings screen composes on top of `Page`: the section's
 * title/description, a way back to the settings hub — or, for a sync run's
 * detail, back to Open Banking specifically — and, when the caller has one,
 * a longer explanatory paragraph above its own content.
 *
 * This keeps the prop surface the callers outside this workstream already
 * use (title/subTitle/content/extra/children) so they need no changes beyond
 * opting into `compactMobileHeader`.
 */
export function SettingsPageContainer({
  title,
  subTitle,
  content,
  extra,
  compactMobileHeader,
  children,
}: SettingsPageContainerProps) {
  const compact = useCompactScreen();
  const { pathname } = useLocation();
  const isSyncDetail = pathname.includes("/sync-runs/");
  const backTo = isSyncDetail ? "/configuracoes/open-banking" : "/configuracoes";
  const backLabel = isSyncDetail ? "Voltar para Open Banking" : "Voltar para configurações";
  const back = <Link to={backTo}>{backLabel}</Link>;
  // Every consumer that opts into compactMobileHeader also already renders
  // its extra as a page action, so its bottom bar is inferred rather than a
  // separate prop that could drift out of sync with it.
  const hasBottomActionBar = compactMobileHeader === true && extra !== undefined;

  return (
    <Page
      title={title}
      description={subTitle}
      back={back}
      backTo={backTo}
      compactMobileHeader={compactMobileHeader}
      hasBottomActionBar={hasBottomActionBar}
      actions={extra}
    >
      {content !== undefined && (
        <Typography.Paragraph type="secondary" className="settings-page-content">
          {content}
        </Typography.Paragraph>
      )}
      {children}
      {hasBottomActionBar && compact && <BottomActionBar>{extra}</BottomActionBar>}
    </Page>
  );
}
