import { useQueries } from "@tanstack/react-query";
import { Alert, Button, Descriptions, Drawer, Flex, InputNumber, List, Select, Space, Tag, Typography } from "antd";
import dayjs from "dayjs";
import { useState } from "react";

import { listInvestmentTransactions } from "../../api/investments";
import type {
  InvestmentOperation,
  InvestmentOperationKind,
  InvestmentOperationWrite,
  InvestmentPosition,
  InvestmentTransaction,
  TransactionItem,
} from "../../api/contracts";
import { useInvestmentWorkspace } from "../../hooks/useInvestmentWorkspace";
import { formatBRL } from "../../presentation/money";
import { sumDecimals, subtractDecimals, isPositiveDecimal, isZeroDecimal } from "../../presentation/decimal";
import { investmentOperationKindLabel } from "../../presentation/investmentWorkspaceLabels";
import { InvestmentOperationForm } from "./InvestmentOperationForm";

function absolute(value: string): string { return value.replace(/^-/, ""); }
const positive = isPositiveDecimal;

function operationDate(transaction: TransactionItem): string {
  return transaction.occurred_at ? dayjs(transaction.occurred_at).format("YYYY-MM-DD") : dayjs().format("YYYY-MM-DD");
}

function joinLabel(parts: Array<string | null | undefined>): string {
  return parts.filter(Boolean).join(" · ");
}

function operationLabel(operation: InvestmentOperation, accountName: string, positionName: string | null): string {
  const figures = `${formatBRL(operation.amount)} · ${dayjs(operation.occurred_on).format("DD/MM/YYYY")}`;
  if (operation.source === "synced") {
    return joinLabel([operation.notes ?? "Movimento importado", accountName, figures]);
  }
  return joinLabel([investmentOperationKindLabel[operation.kind], accountName, positionName, figures]);
}

const movementTypeLabel: Record<string, string> = { BUY: "Aplicação", SELL: "Resgate", INTEREST: "Rendimento" };

function movementDate(movement: InvestmentTransaction): string | null {
  return (movement.trade_date ?? movement.occurred_at)?.slice(0, 10) ?? null;
}

/**
 * One row of the destination select: either a local operation or a provider
 * movement, which the backend turns into its own read-only pivot on link.
 */
type Candidate = {
  key: string;
  label: string;
  operationId: string | null;
  movementId: string | null;
  available: string;
  date: string | null;
};

/**
 * Bank transactions are never moved automatically. This panel only offers
 * likely destinations — local aportes/resgates and the provider's own
 * movements on integrated custodies; the person reviewing the bank line
 * chooses the destination and the amount, including a partial split.
 */
export function InvestmentReconciliationSection({ transaction }: { transaction: TransactionItem }) {
  const [open, setOpen] = useState(false);
  // The panel sits in every BRL transaction drawer. The ledger is only
  // fetched when the person opens the link drawer, or when the transaction
  // already has links to list (the item itself says how much is allocated).
  const workspace = useInvestmentWorkspace({ enabled: open || isPositiveDecimal(transaction.investment_transfer_amount) });
  const [selectedKey, setSelectedKey] = useState<string | null>(null);
  const [amount, setAmount] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [newOperationOpen, setNewOperationOpen] = useState(false);

  const syncedPositions = workspace.positions.filter((position) => position.source === "synced" && position.currency_code === "BRL");
  const importedQueries = useQueries({
    queries: syncedPositions.map((position) => ({
      queryKey: ["investmentTransactions", position.id],
      queryFn: ({ signal }: { signal?: AbortSignal }) => listInvestmentTransactions(position.id, signal),
      enabled: open,
    })),
  });
  const importedLoading = importedQueries.some((query) => query.isLoading);
  const importedError = importedQueries.some((query) => query.isError);

  const links = workspace.reconciliations.filter((link) => link.financial_transaction_id === transaction.id);
  const linkedAmount = sumDecimals(links.map((link) => link.amount));
  const fullValue = transaction.effective_money?.currency_code === "BRL" ? absolute(transaction.effective_money.value) : null;
  const remaining = fullValue === null ? "0.00" : subtractDecimals(fullValue, linkedAmount);
  const available = positive(remaining) ? remaining : "0.00";
  const direction = transaction.classification === "outflow" ? "deposit" : transaction.classification === "inflow" ? "withdrawal" : null;
  const compatibleKinds: InvestmentOperationKind[] = direction === "deposit" ? ["deposit", "fee", "tax"] : direction === "withdrawal" ? ["withdrawal", "income"] : [];
  const accountById = new Map(workspace.accounts.map((account) => [account.id, account]));
  const positionById = new Map(workspace.positions.map((position) => [position.id, position]));
  const linkedToOperation = (operationId: string) =>
    sumDecimals(workspace.reconciliations.filter((link) => link.operation_id === operationId).map((link) => link.amount));
  const pivotFor = (movementId: string): InvestmentOperation | null => {
    const link = workspace.reconciliations.find((item) => item.financial_investment_transaction_id === movementId);
    return link ? workspace.operations.find((operation) => operation.id === link.operation_id && operation.source === "synced") ?? null : null;
  };

  const operationCandidates: Candidate[] = workspace.operations
    .filter((operation) => operation.source === "manual" && compatibleKinds.includes(operation.kind))
    .filter((operation) => accountById.get(operation.account_id)?.currency_code === "BRL")
    .map((operation) => ({
      key: `operation:${operation.id}`,
      label: operationLabel(operation, accountById.get(operation.account_id)?.name ?? "Conta não encontrada",
        operation.position_id ? positionById.get(operation.position_id)?.name ?? null : null),
      operationId: operation.id,
      movementId: null,
      available: subtractDecimals(operation.amount, linkedToOperation(operation.id)),
      date: operation.occurred_on,
    }));
  const importedCandidates: Candidate[] = syncedPositions.flatMap((position: InvestmentPosition, index) =>
    (importedQueries[index]?.data ?? [])
      .filter((movement) => movement.amount !== null && movement.direction === (direction === "deposit" ? "inflow" : "outflow"))
      .map((movement) => {
        const pivot = pivotFor(movement.id);
        const total = absolute(movement.amount!);
        const date = movementDate(movement);
        return {
          key: `movement:${movement.id}`,
          label: joinLabel([
            movementTypeLabel[movement.movement_type ?? ""] ?? movement.movement_type ?? "Movimento",
            position.name,
            `${formatBRL(total)} · ${date ? dayjs(date).format("DD/MM/YYYY") : "Sem data"}`,
          ]),
          operationId: pivot?.id ?? null,
          movementId: movement.id,
          available: pivot ? subtractDecimals(pivot.amount, linkedToOperation(pivot.id)) : total,
          date,
        };
      }),
  );
  const alreadyLinked = (candidate: Candidate) =>
    links.some((link) =>
      (candidate.operationId !== null && link.operation_id === candidate.operationId) ||
      (candidate.movementId !== null && link.financial_investment_transaction_id === candidate.movementId));
  const dateDistance = (candidate: Candidate) =>
    candidate.date ? Math.abs(dayjs(candidate.date).diff(dayjs(operationDate(transaction)), "day")) : Number.MAX_SAFE_INTEGER;
  const suggested = (candidate: Candidate) => dateDistance(candidate) <= 5 && isZeroDecimal(subtractDecimals(candidate.available, available));
  const candidates = [...importedCandidates, ...operationCandidates]
    .filter((candidate) => positive(candidate.available) && !alreadyLinked(candidate))
    .sort((a, b) => Number(suggested(b)) - Number(suggested(a)) || dateDistance(a) - dateDistance(b));
  const selected = candidates.find((candidate) => candidate.key === selectedKey) ?? null;

  const openDrawer = () => {
    setError(null);
    setSelectedKey(null);
    setAmount(positive(available) ? available : null);
    setOpen(true);
  };

  const link = async () => {
    if (!selected) {
      setError("Selecione uma movimentação de destino.");
      return;
    }
    const value = amount ?? "0";
    if (!positive(value)) {
      setError("Informe um valor maior que zero.");
      return;
    }
    if (positive(subtractDecimals(value, available))) {
      setError("O valor vinculado não pode ser maior que o saldo disponível desta transação.");
      return;
    }
    setError(null);
    try {
      await workspace.createReconciliation({
        operation_id: selected.movementId ? null : selected.operationId,
        financial_transaction_id: transaction.id,
        financial_investment_transaction_id: selected.movementId,
        amount: value,
      });
      setOpen(false);
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Não foi possível vincular a movimentação.");
    }
  };

  const createAndLink = async (write: InvestmentOperationWrite) => {
    const value = amount ?? "0";
    if (!positive(value) || positive(subtractDecimals(value, available))) {
      setError("Revise o valor a vincular antes de criar a movimentação.");
      return;
    }
    setError(null);
    try {
      await workspace.createOperations({ operations: [write], reconciliation: {
        financial_transaction_id: transaction.id,
        financial_investment_transaction_id: null, amount: value,
      } });
      setNewOperationOpen(false);
      setOpen(false);
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Não foi possível criar e vincular a movimentação.");
    }
  };

  const unlink = async (reconciliationId: string) => {
    setError(null);
    try {
      await workspace.deleteReconciliation(reconciliationId);
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Não foi possível desfazer o vínculo.");
    }
  };

  const unsupportedReason =
    fullValue === null
      ? "O valor desta transação não está disponível em reais para vincular."
      : direction === null
        ? "Só entradas e saídas classificadas podem ser vinculadas a um investimento."
        : null;
  const linkedText = fullValue === null
    ? null
    : `${formatBRL(linkedAmount)} de ${formatBRL(fullValue)} vinculados`;

  return (
    <>
    <Descriptions title="Investimentos" column={1} size="small" colon={false}>
      <Descriptions.Item label="Vínculo com investimento">
        {linkedText && (
          <div className="transaction-category-origin">
            {linkedText}
            {positive(available) ? " · ainda disponível para dividir" : " · valor integralmente vinculado"}
          </div>
        )}
        {links.length > 0 && (
          <List
            size="small"
            dataSource={links}
            renderItem={(link) => {
              const operation = workspace.operations.find((item) => item.id === link.operation_id);
              const account = operation ? accountById.get(operation.account_id) : undefined;
              const position = operation?.position_id ? positionById.get(operation.position_id) : undefined;
              return (
                <List.Item
                  actions={[
                    <Button key="unlink" type="link" danger size="small" loading={workspace.isDeleting} onClick={() => void unlink(link.id)}>
                      Desvincular
                    </Button>,
                  ]}
                >
                  <Space direction="vertical" size={0}>
                    <span>
                      {operation
                        ? operationLabel(operation, account?.name ?? "Conta não encontrada", position?.name ?? null)
                        : "Movimentação não encontrada"}
                    </span>
                    <Typography.Text type="secondary">{formatBRL(link.amount)} vinculado</Typography.Text>
                  </Space>
                </List.Item>
              );
            }}
          />
        )}
        {error && !open && <Alert type="error" message={error} showIcon />}
        {unsupportedReason ? (
          <Typography.Text type="secondary">{unsupportedReason}</Typography.Text>
        ) : (
          <Button disabled={!positive(available)} onClick={openDrawer}>
            Vincular a investimento
          </Button>
        )}
      </Descriptions.Item>
    </Descriptions>

      <Drawer
        title="Vincular a investimento"
        open={open}
        onClose={() => setOpen(false)}
        width={520}
        destroyOnHidden
        footer={
          <Flex justify="space-between" gap="small">
            <Button onClick={() => setNewOperationOpen(true)} disabled={direction === null || !positive(available)}>
              Nova movimentação
            </Button>
            <Space>
              <Button onClick={() => setOpen(false)}>Cancelar</Button>
              <Button type="primary" loading={workspace.isSaving} onClick={() => void link()}>
                Vincular
              </Button>
            </Space>
          </Flex>
        }
      >
        <Flex vertical gap="middle">
          {error && <Alert type="error" showIcon message={error} />}
          <Alert
            type="info"
            showIcon
            message="Confirme antes de vincular"
            description="As sugestões usam direção, moeda, valor disponível e datas próximas. Movimentos importados da instituição aparecem junto das movimentações manuais. Você decide o destino e pode dividir esta transação em mais de uma movimentação."
          />
          {importedError && <Alert type="warning" showIcon message="Não foi possível carregar o histórico importado de todos os investimentos." />}
          <div className="filter-field">
            <label htmlFor="investment-reconciliation-operation">Movimentação de destino</label>
            <Select
              id="investment-reconciliation-operation"
              value={selectedKey ?? undefined}
              loading={importedLoading}
              options={candidates.map((candidate) => ({ value: candidate.key, label: candidate.label, exact: suggested(candidate) }))}
              optionRender={(option) => (
                <Flex justify="space-between" gap="small">
                  <span>{option.label}</span>
                  {option.data.exact && <Tag color="blue">Sugestão</Tag>}
                </Flex>
              )}
              placeholder={
                direction === "deposit"
                  ? "Selecione um aporte ou aplicação"
                  : "Selecione um resgate ou rendimento"
              }
              showSearch
              optionFilterProp="label"
              notFoundContent={importedLoading ? "Carregando histórico importado…" : "Nenhuma movimentação compatível. Crie uma nova."}
              onChange={(value: string) => setSelectedKey(value)}
            />
          </div>
          <div className="filter-field">
            <label htmlFor="investment-reconciliation-amount">Valor a vincular</label>
            <InputNumber
              id="investment-reconciliation-amount"
              style={{ width: "100%" }}
              min="0.01"
              max={available}
              step="0.01"
              stringMode
              decimalSeparator=","
              value={amount}
              onChange={(value) => setAmount(value === null ? null : String(value))}
            />
            <Typography.Text type="secondary">
              Disponível: {formatBRL(available)}. Você pode vincular o restante depois.
            </Typography.Text>
          </div>
        </Flex>
      </Drawer>

      <InvestmentOperationForm
        open={newOperationOpen}
        operation={null}
        initial={{
          kind: direction ?? "deposit",
          occurred_on: operationDate(transaction),
          amount: amount ?? undefined,
        }}
        allowedKinds={compatibleKinds}
        accounts={workspace.accounts}
        positions={workspace.positions}
        submitting={workspace.isSaving}
        submitError={error}
        onSubmit={(write) => void createAndLink(write)}
        onCancel={() => setNewOperationOpen(false)}
      />
    </>
  );
}
