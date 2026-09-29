import { ApiOutlined, BulbOutlined, ControlOutlined, LockOutlined, RiseOutlined, TagsOutlined, ThunderboltOutlined } from "@ant-design/icons";
import { useNavigate } from "react-router-dom";

import { DataCard, Page } from "../components/layout";
import { PanelNavRow, PanelSection } from "../components/shared/PanelStack";

const sections = [
  { path: "geral", title: "Geral", description: "Escolha como as compras no cartão entram no mês.", icon: <ControlOutlined /> },
  { path: "seguranca", title: "Segurança", description: "Altere sua senha e gerencie o acesso à sua conta.", icon: <LockOutlined /> },
  { path: "cenarios", title: "Cenários", description: "Crie e organize cenários para seu planejamento financeiro.", icon: <BulbOutlined /> },
  { path: "automacoes", title: "Automações", description: "Configure regras para organizar suas transações.", icon: <ThunderboltOutlined /> },
  { path: "categorias", title: "Categorias", description: "Organize as categorias das suas receitas e despesas.", icon: <TagsOutlined /> },
  { path: "open-banking", title: "Open Banking", description: "Gerencie conexões, sincronizações e credenciais da Pluggy.", icon: <ApiOutlined /> },
  { path: "ativos-de-investimento", title: "Ativos de investimento", description: "Cadastre e edite os ativos dos seus investimentos.", icon: <RiseOutlined /> },
];

export function SettingsPage() {
  const navigate = useNavigate();

  return (
    <Page title="Configurações" description="Organize suas preferências e os recursos da sua conta" compactMobileHeader>
      <DataCard>
        <PanelSection className="panel-section-rows">
          {sections.map((section) => (
            <PanelNavRow
              key={section.path}
              icon={section.icon}
              label={section.title}
              hint={section.description}
              onClick={() => navigate(`/configuracoes/${section.path}`)}
            />
          ))}
        </PanelSection>
      </DataCard>
    </Page>
  );
}
