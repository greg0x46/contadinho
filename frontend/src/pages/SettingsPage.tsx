import { ApiOutlined, BulbOutlined, ControlOutlined, LockOutlined, RiseOutlined, TagsOutlined, ThunderboltOutlined } from "@ant-design/icons";
import { PageContainer } from "@ant-design/pro-layout";
import { Card, Col, Row, Typography } from "antd";
import { Link } from "react-router-dom";

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
  return (
    <PageContainer title="Configurações" subTitle="Organize suas preferências e os recursos da sua conta">
      <Row gutter={[16, 16]}>
        {sections.map((section) => (
          <Col key={section.path} xs={24} md={12} xl={8}>
            <Link className="settings-section-link" to={`/configuracoes/${section.path}`} aria-label={section.title}>
              <Card hoverable style={{ height: "100%" }}>
                <Typography.Title level={2} style={{ fontSize: 18, marginTop: 0 }}>
                  <span aria-hidden="true">{section.icon}</span> {section.title}
                </Typography.Title>
                <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>{section.description}</Typography.Paragraph>
              </Card>
            </Link>
          </Col>
        ))}
      </Row>
    </PageContainer>
  );
}
