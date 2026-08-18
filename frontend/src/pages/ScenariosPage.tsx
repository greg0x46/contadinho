import { PlusOutlined } from "@ant-design/icons";
import { PageContainer } from "@ant-design/pro-layout";
import { Alert, Button, Empty, Skeleton } from "antd";
import { useState } from "react";

import type { Scenario, ScenarioTransactionWrite } from "../api/contracts";
import { StandaloneScenarioCard } from "../components/scenarios/StandaloneScenarioCard";
import { StandaloneScenarioForm } from "../components/scenarios/StandaloneScenarioForm";
import { useScenarios } from "../hooks/useScenarios";

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "Não foi possível salvar o cenário.";
}

export function ScenariosPage() {
  const scenarios = useScenarios({ kind: "standalone" });
  const [formOpen, setFormOpen] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const submit = async (name: string) => {
    setSaveError(null);
    try {
      await scenarios.createScenario({ name });
      setFormOpen(false);
    } catch (error) {
      setSaveError(errorMessage(error));
    }
  };

  const removeScenario = async (scenario: Scenario) => {
    setActionError(null);
    try {
      await scenarios.deleteScenario(scenario.id);
    } catch (error) {
      setActionError(errorMessage(error));
    }
  };

  const addTransaction = (scenarioId: string, write: ScenarioTransactionWrite) =>
    scenarios.createTransaction({ scenarioId, write });

  const deleteTransaction = (scenarioId: string, transactionId: string) =>
    scenarios.deleteTransaction({ scenarioId, transactionId });

  return (
    <PageContainer
      title="Cenários"
      subTitle="Simule decisões hipotéticas"
      content="Crie cenários como uma viagem ou uma troca de emprego, com transações hipotéticas, sem vincular a nenhuma dívida ou conta a receber."
      extra={[
        <Button key="new-scenario" type="primary" icon={<PlusOutlined aria-hidden="true" />} onClick={() => setFormOpen(true)}>
          Novo cenário
        </Button>,
      ]}
    >
      {actionError && (
        <Alert
          type="error"
          showIcon
          closable
          onClose={() => setActionError(null)}
          message={actionError}
          style={{ marginBottom: 16 }}
        />
      )}
      {scenarios.error && (
        <Alert
          type="error"
          showIcon
          message="Não foi possível carregar os cenários"
          description={<Button onClick={() => scenarios.refetch()}>Tentar novamente</Button>}
          style={{ marginBottom: 16 }}
        />
      )}
      {scenarios.isLoading && <Skeleton active paragraph={{ rows: 4 }} />}
      {!scenarios.isLoading && scenarios.scenarios.length === 0 && (
        <Empty description="Nenhum cenário criado ainda." />
      )}
      {scenarios.scenarios.map((scenario) => (
        <StandaloneScenarioCard
          key={scenario.id}
          scenario={scenario}
          onDeleteScenario={removeScenario}
          onAddTransaction={addTransaction}
          onDeleteTransaction={deleteTransaction}
        />
      ))}
      <StandaloneScenarioForm
        open={formOpen}
        submitting={scenarios.isCreating}
        submitError={saveError}
        onSubmit={submit}
        onCancel={() => setFormOpen(false)}
      />
    </PageContainer>
  );
}
