import { PlusOutlined } from "@ant-design/icons";
import { SettingsPageContainer as PageContainer } from "../components/SettingsPageContainer";
import { Alert, Button, Empty, Select, Skeleton } from "antd";
import { useState } from "react";

import type { Scenario, ScenarioKind, ScenarioTransactionWrite } from "../api/contracts";
import { scenarioKindLabel } from "../presentation/scenarioLabels";
import { StandaloneScenarioCard } from "../components/scenarios/StandaloneScenarioCard";
import { StandaloneScenarioForm } from "../components/scenarios/StandaloneScenarioForm";
import { useScenarios } from "../hooks/useScenarios";

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "Não foi possível salvar o cenário.";
}

export function ScenariosPage() {
  const [kindFilter, setKindFilter] = useState<ScenarioKind | "all">("all");
  const scenarios = useScenarios({ kind: kindFilter === "all" ? undefined : kindFilter });
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

  const toggleScenario = async (scenario: Scenario, isActive: boolean) => {
    setActionError(null);
    try {
      await scenarios.toggleScenario({ scenarioId: scenario.id, isActive });
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
      <Select
        aria-label="Filtrar cenários por tipo"
        value={kindFilter}
        onChange={setKindFilter}
        options={[
          { value: "all", label: "Todos os tipos" },
          ...Object.entries(scenarioKindLabel).map(([value, label]) => ({ value, label })),
        ]}
        style={{ minWidth: 220, marginBottom: 16 }}
      />
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
          onToggleScenario={toggleScenario}
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
