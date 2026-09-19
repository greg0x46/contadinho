import { PageContainer } from "@ant-design/pro-layout";
import type { ComponentProps } from "react";
import { Link, useLocation } from "react-router-dom";

export function SettingsPageContainer(props: ComponentProps<typeof PageContainer>) {
  const { pathname } = useLocation();
  const isSyncDetail = pathname.includes("/sync-runs/");
  return (
    <PageContainer
      {...props}
      breadcrumb={{ items: [
        { title: <Link to="/configuracoes">Configurações</Link> },
        ...(isSyncDetail ? [{ title: <Link to="/configuracoes/open-banking">Open Banking</Link> }] : []),
        { title: props.title },
      ] }}
      extra={<>{props.extra}<Link to="/configuracoes">Voltar para configurações</Link></>}
    />
  );
}
