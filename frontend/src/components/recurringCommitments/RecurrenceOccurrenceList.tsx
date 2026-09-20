import { Alert, Button, Flex, Modal, Popconfirm, Select, Space, Table, Tag, Typography } from "antd";
import type { ColumnsType } from "antd/es/table";
import { useState } from "react";

import type { EligibleTransaction, RecurrenceOccurrence, RecurringCommitment } from "../../api/contracts";
import { formatDay, formatOptionalLocalDay } from "../../presentation/dates";
import { formatBRL, formatMoney } from "../../presentation/money";
import {
  reconciliationLabel,
  reconciliationStatusColor,
} from "../../presentation/reconciliationLabels";
import {
  useReconciliationCandidates,
  useRecurrenceOccurrences,
} from "../../hooks/useRecurrenceOccurrences";

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

function candidateLabel(candidate: EligibleTransaction): string {
  const date = formatOptionalLocalDay(candidate.occurred_at);
  const description = candidate.description ?? "Sem descrição";
  return `${date} · ${description} · ${formatBRL(candidate.effective_money.value)}`;
}

/**
 * The picker for "Conciliar com…": a server-side searchable Select over the
 * transactions eligible for one occurrence. Same shape as the payables link
 * dialog, because it is the same problem — choosing a real transaction for a
 * planned thing.
 */
function ReconcileDialog({
  commitmentId,
  occurrence,
  onCancel,
  onConfirm,
}: {
  commitmentId: string;
  occurrence: RecurrenceOccurrence | null;
  onCancel: () => void;
  onConfirm: (transactionId: string) => Promise<unknown>;
}) {
  const { search, setSearch, candidates, isSearching } = useReconciliationCandidates(
    commitmentId,
    occurrence?.date ?? null,
  );
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const close = () => {
    setSelectedId(null);
    setError(null);
    onCancel();
  };

  const submit = async () => {
    if (selectedId === null) return;
    setError(null);
    setSubmitting(true);
    try {
      await onConfirm(selectedId);
      setSelectedId(null);
      onCancel();
    } catch (caught) {
      setError(errorMessage(caught, "Não foi possível conciliar a transação."));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title={occurrence === null ? "Conciliar ocorrência" : `Conciliar ocorrência de ${formatDay(occurrence.date)}`}
      open={occurrence !== null}
      onCancel={close}
      onOk={submit}
      okText="Conciliar"
      cancelText="Cancelar"
      okButtonProps={{ disabled: selectedId === null, loading: submitting }}
      destroyOnHidden
    >
      <Flex vertical gap="middle">
        {error && <Alert type="error" showIcon message={error} />}
        {occurrence && (
          <Typography.Text type="secondary">
            Valor esperado: {formatBRL(occurrence.expected_amount)}
          </Typography.Text>
        )}
        <Select
          aria-label="Buscar transação para conciliar"
          showSearch
          allowClear
          style={{ width: "100%" }}
          placeholder="Buscar por descrição"
          value={selectedId}
          searchValue={search}
          onSearch={setSearch}
          filterOption={false}
          loading={isSearching}
          notFoundContent={isSearching ? "Buscando…" : "Nenhuma transação elegível por perto"}
          options={candidates.map((candidate) => ({
            value: candidate.id,
            label: candidateLabel(candidate),
          }))}
          onChange={(value: string | null) => setSelectedId(value)}
        />
      </Flex>
    </Modal>
  );
}

/**
 * The expanded body of a commitment row: its recent occurrences and what
 * settled each one.
 *
 * The three actions map to the three states an occurrence can be in.
 * "Desconciliar" always leaves the occurrence unreconciled, whether an
 * automation rule or a manual pick was behind it — deleting a manual link
 * alone would let the rule re-claim the occurrence on the next read, and the
 * button would look broken. "Voltar ao automático" is the separate, explicit
 * undo that hands it back to the rule.
 */
export function RecurrenceOccurrenceList({ commitment }: { commitment: RecurringCommitment }) {
  const occurrences = useRecurrenceOccurrences(commitment.id);
  const [picking, setPicking] = useState<RecurrenceOccurrence | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [pendingDate, setPendingDate] = useState<string | null>(null);

  const runAction = async (date: string, action: () => Promise<unknown>, fallback: string) => {
    setActionError(null);
    setPendingDate(date);
    try {
      await action();
    } catch (error) {
      setActionError(errorMessage(error, fallback));
    } finally {
      setPendingDate(null);
    }
  };

  const columns: ColumnsType<RecurrenceOccurrence> = [
    {
      title: "Ocorrência",
      dataIndex: "date",
      render: (_: unknown, occurrence) => formatDay(occurrence.date),
    },
    {
      title: "Valor esperado",
      dataIndex: "expected_amount",
      render: (_: unknown, occurrence) => formatMoney(occurrence.expected_amount, "BRL"),
    },
    {
      title: "Situação",
      dataIndex: "status",
      render: (_: unknown, occurrence) => (
        <Tag color={reconciliationStatusColor[occurrence.status]}>
          {reconciliationLabel(occurrence.status, occurrence.origin)}
        </Tag>
      ),
    },
    {
      title: "Transação",
      dataIndex: "transaction",
      render: (_: unknown, occurrence) => {
        if (occurrence.transaction === null) return "—";
        const { occurred_at, description, effective_money } = occurrence.transaction;
        return (
          <span>
            {formatOptionalLocalDay(occurred_at)} · {description ?? "Sem descrição"} ·{" "}
            {formatBRL(effective_money.value)}
          </span>
        );
      },
    },
    {
      title: "Ação",
      render: (_: unknown, occurrence) => {
        const pending = pendingDate === occurrence.date;
        return (
          <Space>
            {occurrence.status === "reconciled" ? (
              <Popconfirm
                title="Desconciliar ocorrência"
                description="A ocorrência volta a ser projetada na Home. A transação permanece inalterada."
                onConfirm={() =>
                  runAction(
                    occurrence.date,
                    () => occurrences.detach(occurrence.date),
                    "Não foi possível desconciliar a ocorrência.",
                  )
                }
                okText="Desconciliar"
                cancelText="Cancelar"
              >
                <Button type="link" danger loading={pending}>
                  Desconciliar
                </Button>
              </Popconfirm>
            ) : (
              <Button type="link" onClick={() => setPicking(occurrence)}>
                Conciliar com…
              </Button>
            )}
            {occurrence.status === "detached" && (
              <Button
                type="link"
                loading={pending}
                onClick={() =>
                  runAction(
                    occurrence.date,
                    () => occurrences.restoreAutomatic(occurrence.date),
                    "Não foi possível voltar ao automático.",
                  )
                }
              >
                Voltar ao automático
              </Button>
            )}
          </Space>
        );
      },
    },
  ];

  return (
    <Flex vertical gap="small">
      {actionError && (
        <Alert
          type="error"
          showIcon
          closable
          onClose={() => setActionError(null)}
          message={actionError}
        />
      )}
      {occurrences.error !== null && (
        <Alert
          type="warning"
          showIcon
          message="Não foi possível carregar as ocorrências."
          action={<Button onClick={occurrences.retry}>Tentar novamente</Button>}
        />
      )}
      {/*
        A plain antd Table, not the ProTable the top-level lists use: every
        ProTable feature here would be switched off anyway (search, toolbar,
        pagination), and its ProCard wrapper carries a `margin: 0 -8px` that
        cancels the expanded cell's padding — the very thing antd's own
        nested-table rule (`margin-left: 40px; margin-right: -8px`) already
        does. Applied twice, the table's right edge landed 8px past the outer
        table's scroll container, so expanding a row always produced a
        horizontal scrollbar even with room to spare.
      */}
      <Table<RecurrenceOccurrence>
        aria-label={`Ocorrências de ${commitment.name}`}
        columns={columns}
        dataSource={occurrences.occurrences}
        loading={occurrences.isLoading}
        rowKey="date"
        pagination={false}
        size="small"
        locale={{
          emptyText: "Nenhuma ocorrência neste período.",
        }}
      />
      <ReconcileDialog
        commitmentId={commitment.id}
        occurrence={picking}
        onCancel={() => setPicking(null)}
        onConfirm={(transactionId) => occurrences.reconcile(picking!.date, transactionId)}
      />
    </Flex>
  );
}
