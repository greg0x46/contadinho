import { PlusOutlined } from "@ant-design/icons";
import { SettingsPageContainer as PageContainer } from "../components/SettingsPageContainer";
import { Alert, Button, Select } from "antd";
import { useState } from "react";

import type { Scenario, ScenarioKind, ScenarioTransactionWrite } from "../api/contracts";
import { scenarioKindLabel } from "../presentation/scenarioLabels";
import { StandaloneScenarioList } from "../components/scenarios/StandaloneScenarioCard";
import { StandaloneScenarioForm } from "../components/scenarios/StandaloneScenarioForm";
import { DataCard, ListToolbar, SearchField } from "../components/layout";
import { useScenarios } from "../hooks/useScenarios";

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "Não foi possível salvar o cenário.";
}

/** Search happens here: the list is small and already fully loaded (the kind
 *  filter, unlike search, changes what the server returns). */
function arrange(scenarios: Scenario[], search: string): Scenario[] {
  const needle = search.trim().toLocaleLowerCase("pt-BR");
  if (!needle) return scenarios;
  return scenarios.filter((scenario) => scenario.name.toLocaleLowerCase("pt-BR").includes(needle));
}

export function ScenariosPage() {
  const [kindFilter, setKindFilter] = useState<ScenarioKind | "all">("all");
  const scenarios = useScenarios({ kind: kindFilter === "all" ? undefined : kindFilter });
  const [search, setSearch] = useState("");
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

      <ListToolbar
        label="Controles dos cenários"
        start={
          <SearchField
            id="scenarios-search"
            label="Buscar cenários"
            placeholder="Buscar por nome…"
            value={search}
            onChange={setSearch}
          />
        }
        end={
          <Select
            aria-label="Filtrar cenários por tipo"
            value={kindFilter}
            onChange={setKindFilter}
            options={[
              { value: "all", label: "Todos os tipos" },
              ...Object.entries(scenarioKindLabel).map(([value, label]) => ({ value, label })),
            ]}
            popupMatchSelectWidth={false}
          />
        }
      />

      <DataCard flush>
        <StandaloneScenarioList
          scenarios={arrange(scenarios.scenarios, search)}
          isLoading={scenarios.isLoading}
          onDeleteScenario={removeScenario}
          onAddTransaction={addTransaction}
          onDeleteTransaction={deleteTransaction}
          onToggleScenario={toggleScenario}
        />
      </DataCard>
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
