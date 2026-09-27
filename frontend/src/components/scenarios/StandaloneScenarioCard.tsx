import { DeleteOutlined, PlusOutlined } from "@ant-design/icons";
import { Alert, Button, DatePicker, Flex, Input, InputNumber, Popconfirm, Select, Skeleton, Space, Switch, Tag } from "antd";
import type { ProColumns } from "@ant-design/pro-table";
import ProTable from "@ant-design/pro-table";
import dayjs, { type Dayjs } from "dayjs";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";

import type { Scenario, ScenarioTransaction, ScenarioTransactionWrite } from "../../api/contracts";
import { getScenario, listScenarioPlannedTransactions } from "../../api/scenarios";
import { formatBRL } from "../../presentation/money";
import { scenarioKindLabel } from "../../presentation/scenarioLabels";

const dateFormat = "YYYY-MM-DD";

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

function NewTransactionRow({
  onAdd,
  submitting,
}: {
  onAdd: (write: ScenarioTransactionWrite) => Promise<void>;
  submitting: boolean;
}) {
  const [description, setDescription] = useState("");
  const [kind, setKind] = useState<"income" | "expense">("expense");
  const [amount, setAmount] = useState<number | null>(null);
  const [projectedAt, setProjectedAt] = useState<Dayjs>(dayjs());
  const [error, setError] = useState<string | null>(null);

  const submit = async () => {
    if (description.trim() === "") {
      setError("Informe uma descrição.");
      return;
    }
    if (amount === null || amount <= 0) {
      setError("Informe um valor maior que zero.");
      return;
    }
    setError(null);
    await onAdd({
      description: description.trim(),
      amount: kind === "expense" ? -amount : amount,
      projected_at: projectedAt.format(dateFormat),
    });
    setDescription("");
    setAmount(null);
    setProjectedAt(dayjs());
  };

  return (
    <Flex vertical gap="small" className="scenario-panel-new-transaction">
      {error && <Alert type="error" showIcon message={error} closable onClose={() => setError(null)} />}
      <Flex gap="small" wrap>
        <Input
          aria-label="Descrição"
          placeholder="Descrição"
          value={description}
          onChange={(event) => setDescription(event.target.value)}
          style={{ maxWidth: 220 }}
        />
        <Select
          aria-label="Tipo"
          value={kind}
          options={[
            { value: "expense", label: "Despesa" },
            { value: "income", label: "Receita" },
          ]}
          onChange={setKind}
          style={{ width: 120 }}
        />
        <InputNumber
          aria-label="Valor"
          placeholder="0,00"
          min={0.01}
          step={0.01}
          decimalSeparator=","
          value={amount}
          onChange={setAmount}
        />
        <DatePicker
          aria-label="Data prevista"
          format="DD/MM/YYYY"
          value={projectedAt}
          allowClear={false}
          onChange={(value) => value && setProjectedAt(value)}
        />
        <Button icon={<PlusOutlined aria-hidden="true" />} loading={submitting} onClick={submit}>
          Adicionar transação hipotética
        </Button>
      </Flex>
    </Flex>
  );
}

/**
 * The expanded body of a scenario row: its hypothetical transactions, the
 * realized/pending counts of the projected occurrences, and — for
 * standalone scenarios only — the form to add another transaction.
 *
 * Detail and planned-transaction data load here, per expanded scenario,
 * rather than eagerly for the whole list: the same choice
 * RecurrenceOccurrenceList makes for recurring commitments, for the same
 * reason (most scenarios are never opened in a given visit).
 */
function ScenarioTransactionsPanel({
  scenario,
  onAddTransaction,
  onDeleteTransaction,
}: {
  scenario: Scenario;
  onAddTransaction: (scenarioId: string, write: ScenarioTransactionWrite) => Promise<unknown>;
  onDeleteTransaction: (scenarioId: string, transactionId: string) => Promise<void>;
}) {
  const detailQuery = useQuery({
    queryKey: ["scenarios", scenario.id, "detail"],
    queryFn: ({ signal }) => getScenario(scenario.id, signal),
  });
  const plannedQuery = useQuery({
    queryKey: ["scenarios", scenario.id, "planned-transactions"],
    queryFn: async ({ signal }) =>
      (await listScenarioPlannedTransactions(scenario.id, undefined, undefined, signal)) ?? [],
  });
  const [addError, setAddError] = useState<string | null>(null);
  const [adding, setAdding] = useState(false);

  const add = async (write: ScenarioTransactionWrite) => {
    setAddError(null);
    setAdding(true);
    try {
      await onAddTransaction(scenario.id, write);
      await detailQuery.refetch();
    } catch (error) {
      setAddError(errorMessage(error, "Não foi possível adicionar a transação."));
    } finally {
      setAdding(false);
    }
  };

  const remove = async (transactionId: string) => {
    await onDeleteTransaction(scenario.id, transactionId);
    await detailQuery.refetch();
  };

  const columns: ProColumns<ScenarioTransaction>[] = [
    { title: "Descrição", dataIndex: "description" },
    {
      title: "Valor",
      dataIndex: "amount",
      render: (_, transaction) => formatBRL(transaction.amount),
    },
    {
      title: "Data prevista",
      dataIndex: "projected_at",
      render: (_, transaction) => {
        const [year, month, day] = transaction.projected_at.split("-");
        return `${day}/${month}/${year}`;
      },
    },
    {
      title: "Ação",
      valueType: "option",
      render: (_, transaction) => (
        <Popconfirm
          title="Excluir transação hipotética"
          onConfirm={() => remove(transaction.id)}
          okText="Excluir"
          cancelText="Cancelar"
        >
          <Button type="link" danger icon={<DeleteOutlined aria-hidden="true" />} />
        </Popconfirm>
      ),
    },
  ];

  return (
    <Flex vertical gap="small" className="scenario-panel">
      {addError && <Alert type="error" showIcon message={addError} closable onClose={() => setAddError(null)} />}
      <ProTable<ScenarioTransaction>
        aria-label={`Transações hipotéticas de ${scenario.name}`}
        columns={columns}
        dataSource={detailQuery.data?.transactions ?? []}
        loading={detailQuery.isLoading}
        rowKey="id"
        search={false}
        options={false}
        pagination={false}
        size="small"
        locale={{ emptyText: "Nenhuma transação hipotética ainda." }}
      />
      <div className="scenario-panel-counts">
        Eventos realizados: {plannedQuery.data?.filter((event) => event.realized).length ?? 0}
        {" · "}
        Eventos pendentes: {plannedQuery.data?.filter((event) => !event.realized).length ?? 0}
      </div>
      {scenario.kind === "standalone" && <NewTransactionRow onAdd={add} submitting={adding} />}
    </Flex>
  );
}

const emptyText = <span>Nenhum cenário criado ainda.</span>;

/**
 * Scenarios as flat rows — name, type, source and status up front, with the
 * hypothetical transactions of each one tucked behind its row's expander
 * rather than always-open per-scenario cards (see RecurringCommitmentList
 * for the same shape applied to recurring commitments).
 */
export function StandaloneScenarioList({
  scenarios,
  isLoading,
  onDeleteScenario,
  onAddTransaction,
  onDeleteTransaction,
  onToggleScenario,
}: {
  scenarios: Scenario[];
  isLoading: boolean;
  onDeleteScenario: (scenario: Scenario) => void;
  onAddTransaction: (scenarioId: string, write: ScenarioTransactionWrite) => Promise<unknown>;
  onDeleteTransaction: (scenarioId: string, transactionId: string) => Promise<void>;
  onToggleScenario?: (scenario: Scenario, isActive: boolean) => Promise<unknown>;
}) {
  if (isLoading) {
    return <Skeleton active paragraph={{ rows: 4 }} />;
  }

  const columns: ProColumns<Scenario>[] = [
    { title: "Nome", dataIndex: "name" },
    {
      title: "Tipo",
      dataIndex: "kind",
      render: (_, scenario) => <Tag>{scenarioKindLabel[scenario.kind]}</Tag>,
    },
    {
      title: "Origem",
      dataIndex: "is_accounting_source",
      render: (_, scenario) => (
        <Tag color={scenario.is_accounting_source ? "blue" : undefined}>
          {scenario.is_accounting_source ? "Contábil" : "Simulação"}
        </Tag>
      ),
    },
    {
      title: "Ativo",
      dataIndex: "is_active",
      render: (_, scenario) =>
        onToggleScenario ? (
          <Switch
            aria-label={`${scenario.is_active === false ? "Ativar" : "Desativar"} cenário ${scenario.name}`}
            checked={scenario.is_active !== false}
            onChange={(checked) => void onToggleScenario(scenario, checked)}
          />
        ) : null,
    },
    {
      title: "Ação",
      valueType: "option",
      render: (_, scenario) => (
        <Space onClick={(event) => event.stopPropagation()}>
          <Popconfirm
            title="Excluir cenário"
            description="As projeções deste cenário também serão excluídas."
            onConfirm={() => onDeleteScenario(scenario)}
            okText="Excluir"
            cancelText="Cancelar"
          >
            <Button type="link" danger>
              Excluir
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <ProTable<Scenario>
      aria-label="Cenários"
      columns={columns}
      dataSource={scenarios}
      rowKey="id"
      search={false}
      options={false}
      pagination={false}
      cardBordered
      scroll={{ x: "max-content" }}
      expandable={{
        expandedRowRender: (scenario) => (
          <ScenarioTransactionsPanel
            scenario={scenario}
            onAddTransaction={onAddTransaction}
            onDeleteTransaction={onDeleteTransaction}
          />
        ),
      }}
      locale={{ emptyText: <div className="debt-list-empty">{emptyText}</div> }}
    />
  );
}
