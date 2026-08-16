import { DeleteOutlined, PlusOutlined } from "@ant-design/icons";
import { Alert, Button, Card, DatePicker, Flex, Input, InputNumber, Popconfirm, Select } from "antd";
import type { ProColumns } from "@ant-design/pro-table";
import ProTable from "@ant-design/pro-table";
import dayjs, { type Dayjs } from "dayjs";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";

import type { Scenario, ScenarioTransaction, ScenarioTransactionWrite } from "../../api/contracts";
import { getScenario } from "../../api/scenarios";
import { formatBRL } from "../../presentation/money";

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
    <Flex vertical gap="small" style={{ marginTop: 12 }}>
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

export function StandaloneScenarioCard({
  scenario,
  onDeleteScenario,
  onAddTransaction,
  onDeleteTransaction,
}: {
  scenario: Scenario;
  onDeleteScenario: (scenario: Scenario) => void;
  onAddTransaction: (scenarioId: string, write: ScenarioTransactionWrite) => Promise<unknown>;
  onDeleteTransaction: (scenarioId: string, transactionId: string) => Promise<void>;
}) {
  const detailQuery = useQuery({
    queryKey: ["scenarios", scenario.id, "detail"],
    queryFn: ({ signal }) => getScenario(scenario.id, signal),
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
    <Card
      title={scenario.name}
      extra={
        <Popconfirm
          title="Excluir cenário"
          description="As transações hipotéticas deste cenário também serão excluídas."
          onConfirm={() => onDeleteScenario(scenario)}
          okText="Excluir"
          cancelText="Cancelar"
        >
          <Button type="link" danger>
            Excluir cenário
          </Button>
        </Popconfirm>
      }
      style={{ marginBottom: 16 }}
    >
      {addError && <Alert type="error" showIcon message={addError} style={{ marginBottom: 12 }} />}
      <ProTable<ScenarioTransaction>
        aria-label={`Transações hipotéticas de ${scenario.name}`}
        columns={columns}
        dataSource={detailQuery.data?.transactions ?? []}
        loading={detailQuery.isLoading}
        rowKey="id"
        search={false}
        options={false}
        pagination={false}
        locale={{ emptyText: "Nenhuma transação hipotética ainda." }}
      />
      <NewTransactionRow onAdd={add} submitting={adding} />
    </Card>
  );
}
