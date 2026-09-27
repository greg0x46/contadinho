import type { ReactNode } from "react";
import { Link, useLocation } from "react-router-dom";
import { Typography } from "antd";

import { Page } from "./layout";

interface SettingsPageContainerProps {
  title: string;
  /** One line under the title — mirrors Page's own `description`. */
  subTitle?: string;
  /** A longer explanation of the section, shown above its content. */
  content?: ReactNode;
  /** Page-wide actions (e.g. "Nova categoria"). Rendered in Page's actions slot. */
  extra?: ReactNode;
  children: ReactNode;
}

/**
 * The shell every settings screen composes on top of `Page`: the section's
 * title/description, a way back to the settings hub — or, for a sync run's
 * detail, back to Open Banking specifically — and, when the caller has one,
 * a longer explanatory paragraph above its own content.
 *
 * This keeps the prop surface the callers outside this workstream already
 * use (title/subTitle/content/extra/children) so they need no changes.
 */
export function SettingsPageContainer({ title, subTitle, content, extra, children }: SettingsPageContainerProps) {
  const { pathname } = useLocation();
  const isSyncDetail = pathname.includes("/sync-runs/");
  const back = isSyncDetail ? (
    <Link to="/configuracoes/open-banking">Voltar para Open Banking</Link>
  ) : (
    <Link to="/configuracoes">Voltar para configurações</Link>
  );

  return (
    <Page title={title} description={subTitle} back={back} actions={extra}>
      {content !== undefined && (
        <Typography.Paragraph type="secondary" className="settings-page-content">
          {content}
        </Typography.Paragraph>
      )}
      {children}
    </Page>
  );
}
