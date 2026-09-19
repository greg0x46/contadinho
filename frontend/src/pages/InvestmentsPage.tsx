import { PlusOutlined } from "@ant-design/icons";
import { PageContainer } from "@ant-design/pro-layout";
import { Alert, Button, Empty, Segmented, Space, Switch } from "antd";
import { useState } from "react";
import { useNavigate } from "react-router-dom";

import type {
  Investment,
  InvestmentAccount,
  InvestmentAccountUpdate,
  InvestmentAccountWrite,
  InvestmentOperation,
  InvestmentOperationWrite,
  InvestmentPortfolio,
  InvestmentPortfolioWrite,
  InvestmentPosition,
  InvestmentPositionUpdate,
  InvestmentPositionWrite,
} from "../api/contracts";
import { LoadingState, UnavailableState } from "../components/AsyncState";
import { InvestmentAccountCard } from "../components/investments/InvestmentAccountCard";
import { InvestmentAccountEditForm } from "../components/investments/InvestmentAccountEditForm";
import { InvestmentAccountForm } from "../components/investments/InvestmentAccountForm";
import { InvestmentGoalCard } from "../components/investments/InvestmentGoalCard";
import { InvestmentList } from "../components/investments/InvestmentList";
import { InvestmentOperationForm } from "../components/investments/InvestmentOperationForm";
import { InvestmentPortfolioForm } from "../components/investments/InvestmentPortfolioForm";
import { InvestmentPositionForm } from "../components/investments/InvestmentPositionForm";
import { InvestmentWorkspaceSummary } from "../components/investments/InvestmentWorkspaceSummary";
import { InvestmentsSummary } from "../components/investments/InvestmentsSummary";
import { useAccounts } from "../hooks/useAccounts";
import { useInvestmentWorkspace } from "../hooks/useInvestmentWorkspace";
import { useInvestments } from "../hooks/useInvestments";

type View = "accounts" | "goals" | "synced";

const viewOptions = [
  { label: "Por conta", value: "accounts" },
  { label: "Por objetivo", value: "goals" },
  { label: "Sincronizados", value: "synced" },
];

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

export function InvestmentsPage() {
  const workspace = useInvestmentWorkspace();
  const financialAccounts = useAccounts();
  const syncedInvestments = useInvestments();
  const navigate = useNavigate();

  const [view, setView] = useState<View>("accounts");
  const [showClosedPositions, setShowClosedPositions] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);

  const [accountFormOpen, setAccountFormOpen] = useState(false);
  const [editingAccount, setEditingAccount] = useState<InvestmentAccount | null>(null);
  const [portfolioFormOpen, setPortfolioFormOpen] = useState(false);
  const [editingPortfolio, setEditingPortfolio] = useState<InvestmentPortfolio | null>(null);
  const [positionFormOpen, setPositionFormOpen] = useState(false);
  const [editingPosition, setEditingPosition] = useState<InvestmentPosition | null>(null);
  // InvestmentPositionForm picks its account from the list it is given, so a
  // card's "Nova posição" hands it only that account instead of a preselection.
  const [positionAccounts, setPositionAccounts] = useState<InvestmentAccount[] | null>(null);
  const [operationFormOpen, setOperationFormOpen] = useState(false);
  const [editingOperation, setEditingOperation] = useState<InvestmentOperation | null>(null);
  const [operationAccountId, setOperationAccountId] = useState<string | null>(null);

  const manualAccounts = workspace.accounts.filter((account) => account.kind === "manual");
  const accountName = (accountId: string) =>
    workspace.accounts.find((account) => account.id === accountId)?.name ?? "Conta não encontrada";
  const summaryAccount = (accountId: string) =>
    workspace.summary?.accounts.find((entry) => entry.account_id === accountId);
  const summaryPortfolio = (portfolioId: string | null) =>
    workspace.summary?.portfolios.find((entry) => entry.portfolio_id === portfolioId);
  // Closed positions (fully sold or redeemed) are hidden by default so the
  // workspace reads as what is actually invested today; the switch below
  // brings them back for anyone reviewing history.
  const visiblePositions = showClosedPositions
    ? workspace.positions
    : workspace.positions.filter((position) => !position.closed);
  const positionsOfAccount = (accountId: string) =>
    visiblePositions.filter((position) => position.account_id === accountId);
  const positionsOfGoal = (portfolioId: string | null) =>
    visiblePositions.filter((position) => position.portfolio_id === portfolioId);
  // The switch only hides rows of the positions tables. Movement history and
  // totals still belong to a closed position: its buys and sells keep their
  // name in the operations table and keep counting in the goal's figures.
  const allPositionsOfAccount = (accountId: string) =>
    workspace.positions.filter((position) => position.account_id === accountId);
  const allPositionsOfGoal = (portfolioId: string | null) =>
    workspace.positions.filter((position) => position.portfolio_id === portfolioId);
  const operationsOfAccount = (accountId: string) =>
    workspace.operations.filter((operation) => operation.account_id === accountId);
  const operationsOfPositions = (positions: InvestmentPosition[]) => {
    const ids = new Set(positions.map((position) => position.id));
    return workspace.operations.filter((operation) => operation.position_id !== null && ids.has(operation.position_id));
  };

  const run = async (action: () => Promise<unknown>, fallback: string) => {
    setActionError(null);
    try {
      await action();
      return true;
    } catch (error) {
      setActionError(errorMessage(error, fallback));
      return false;
    }
  };

  const save = async (action: () => Promise<unknown>, close: () => void, fallback: string) => {
    setSaveError(null);
    try {
      await action();
      close();
    } catch (error) {
      setSaveError(errorMessage(error, fallback));
    }
  };

  const openAccountCreate = () => {
    setSaveError(null);
    setAccountFormOpen(true);
  };
  const openAccountEdit = (account: InvestmentAccount) => {
    setSaveError(null);
    setEditingAccount(account);
  };
  const openPortfolio = (portfolio: InvestmentPortfolio | null) => {
    setSaveError(null);
    setEditingPortfolio(portfolio);
    setPortfolioFormOpen(true);
  };
  const openPositionCreate = (accounts: InvestmentAccount[] | null) => {
    setSaveError(null);
    setEditingPosition(null);
    setPositionAccounts(accounts);
    setPositionFormOpen(true);
  };
  const openPositionEdit = (position: InvestmentPosition) => {
    setSaveError(null);
    setEditingPosition(position);
    setPositionAccounts(null);
    setPositionFormOpen(true);
  };
  const openOperationCreate = (accountId: string | null) => {
    setSaveError(null);
    setEditingOperation(null);
    setOperationAccountId(accountId);
    setOperationFormOpen(true);
  };
  const openOperationEdit = (operation: InvestmentOperation) => {
    setSaveError(null);
    setEditingOperation(operation);
    setOperationAccountId(operation.account_id);
    setOperationFormOpen(true);
  };

  const submitAccount = (write: InvestmentAccountWrite) =>
    void save(() => workspace.createAccount(write), () => setAccountFormOpen(false), "Não foi possível criar a conta.");
  const submitAccountEdit = (write: InvestmentAccountUpdate) => {
    if (!editingAccount) return;
    void save(
      () => workspace.updateAccount({ accountId: editingAccount.id, write }),
      () => setEditingAccount(null),
      "Não foi possível salvar a conta.",
    );
  };
  const submitPortfolio = (write: InvestmentPortfolioWrite) =>
    void save(
      () =>
        editingPortfolio
          ? workspace.updatePortfolio({ portfolioId: editingPortfolio.id, write })
          : workspace.createPortfolio(write),
      () => setPortfolioFormOpen(false),
      "Não foi possível salvar o objetivo.",
    );
  const submitPositionCreate = (write: InvestmentPositionWrite) =>
    void save(
      () => workspace.createPosition(write),
      () => setPositionFormOpen(false),
      "Não foi possível criar a posição.",
    );
  const submitPositionUpdate = (write: InvestmentPositionUpdate) => {
    if (!editingPosition) return;
    void save(
      () => workspace.updatePosition({ positionId: editingPosition.id, write }),
      () => setPositionFormOpen(false),
      "Não foi possível salvar a posição.",
    );
  };
  const submitOperation = (write: InvestmentOperationWrite) =>
    void save(
      () =>
        editingOperation
          ? workspace.updateOperation({ operationId: editingOperation.id, write })
          : workspace.createOperation(write),
      () => setOperationFormOpen(false),
      "Não foi possível salvar a movimentação.",
    );

  /**
   * Changing a goal rewrites only the grouping. The position keeps its account,
   * its quantity and its value, so nothing about the money moves.
   */
  const assignGoal = (position: InvestmentPosition, portfolioId: string | null) =>
    void run(
      () =>
        workspace.updatePosition({
          positionId: position.id,
          write: {
            name: position.name,
            ticker: position.ticker,
            asset_type: position.asset_type,
            portfolio_id: portfolioId,
            notes: position.notes,
          },
        }),
      "Não foi possível mudar o objetivo desta posição.",
    );

  const busy = workspace.isSaving || workspace.isDeleting;
  const goalCards: { portfolio: InvestmentPortfolio | null }[] = [
    ...workspace.portfolios.map((portfolio) => ({ portfolio })),
    { portfolio: null },
  ];

  return (
    <PageContainer
      title="Investimentos"
      subTitle="Contas de custódia, objetivos e o caixa disponível para investir"
      content="Aportes e resgates transferem patrimônio: eles mudam o seu caixa, aparecem como aporte ou resgate e não contam como gasto nem alteram o total do patrimônio."
      extra={[
        <Space key="actions" wrap>
          <Button onClick={() => navigate("/transacoes?period=all")}>Revisar lançamentos antigos</Button>
          <Button onClick={() => openOperationCreate(null)} disabled={workspace.accounts.length === 0}>
            Registrar movimentação
          </Button>
          <Button onClick={() => openPositionCreate(null)} disabled={manualAccounts.length === 0}>
            Nova posição
          </Button>
          <Button onClick={() => openPortfolio(null)}>Novo objetivo</Button>
          <Button type="primary" icon={<PlusOutlined aria-hidden="true" />} onClick={openAccountCreate}>
            Nova conta de custódia
          </Button>
        </Space>,
      ]}
    >
      {workspace.error && !workspace.isLoading && (
        <UnavailableState onRetry={() => void workspace.refetch()}>
          Não foi possível carregar os investimentos
        </UnavailableState>
      )}
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

      {workspace.isLoading ? (
        <LoadingState>Carregando investimentos…</LoadingState>
      ) : (
        <>
          {workspace.positions.some((position) => position.currency_code !== "BRL") && (
            <Alert type="info" showIcon style={{ marginBottom: 16 }} message="Totais em reais"
              description="Posições em outras moedas são exibidas na moeda original e ficam fora dos totais e metas em reais. Esta versão não faz conversão de câmbio." />
          )}
          <InvestmentWorkspaceSummary summary={workspace.summary} operations={workspace.operations} />

          <Space style={{ marginBottom: 16, display: "flex", justifyContent: "space-between", width: "100%" }} wrap>
            <Segmented
              options={viewOptions}
              value={view}
              onChange={(value) => setView(value as View)}
            />
            {view !== "synced" && (
              <Space>
                <Switch
                  size="small"
                  checked={showClosedPositions}
                  onChange={setShowClosedPositions}
                  aria-label="Mostrar posições fechadas"
                />
                <span>Mostrar posições fechadas</span>
              </Space>
            )}
          </Space>

          {view === "accounts" &&
            (workspace.accounts.length === 0 ? (
              <Empty description="Nenhuma conta de custódia ainda. Crie uma conta manual ou conecte uma instituição." />
            ) : (
              workspace.accounts.map((account) => (
                <InvestmentAccountCard
                  key={account.id}
                  account={account}
                  summary={summaryAccount(account.id)}
                  positions={positionsOfAccount(account.id)}
                  allPositions={allPositionsOfAccount(account.id)}
                  operations={operationsOfAccount(account.id)}
                  portfolios={workspace.portfolios}
                  onRename={openAccountEdit}
                  onRemove={(target) =>
                    void run(() => workspace.deleteAccount(target.id), "Não foi possível remover a conta.")
                  }
                  onNewPosition={(target) => openPositionCreate([target])}
                  onNewOperation={(target) => openOperationCreate(target.id)}
                  onEditPosition={openPositionEdit}
                  onRemovePosition={(position) =>
                    void run(() => workspace.deletePosition(position.id), "Não foi possível remover a posição.")
                  }
                  onEditOperation={openOperationEdit}
                  onRemoveOperation={(operation) =>
                    void run(
                      () => workspace.deleteOperation(operation.id),
                      "Não foi possível excluir a movimentação.",
                    )
                  }
                  onAssignGoal={assignGoal}
                  busy={busy}
                />
              ))
            ))}

          {view === "goals" &&
            goalCards.map(({ portfolio }) => {
              const positions = positionsOfGoal(portfolio?.id ?? null);
              // The "sem objetivo" bucket only earns a card when it has
              // something in it; an empty one would just be noise.
              if (portfolio === null && positions.length === 0) return null;
              return (
                <InvestmentGoalCard
                  key={portfolio?.id ?? "sem-objetivo"}
                  portfolio={portfolio}
                  summary={summaryPortfolio(portfolio?.id ?? null)}
                  positions={positions}
                  operations={operationsOfPositions(allPositionsOfGoal(portfolio?.id ?? null))}
                  portfolios={workspace.portfolios}
                  cashBalance={workspace.summary?.cash_balance ?? "0"}
                  accountNameOf={accountName}
                  onEdit={openPortfolio}
                  onRemove={(target) =>
                    void run(() => workspace.deletePortfolio(target.id), "Não foi possível remover o objetivo.")
                  }
                  onAssignGoal={assignGoal}
                  busy={busy}
                />
              );
            })}

          {view === "synced" && (
            <>
              <Alert
                type="info"
                showIcon
                style={{ marginBottom: 16 }}
                message="Importados da sua instituição"
                description="Estes ativos também aparecem como posições dentro da conta integrada correspondente. Aqui fica o detalhe importado, com o histórico do provedor."
              />
              {syncedInvestments.error && (
                <Alert
                  type="error"
                  showIcon
                  message="Não foi possível carregar os investimentos sincronizados"
                  description={<Button onClick={() => syncedInvestments.refetch()}>Tentar novamente</Button>}
                  style={{ marginBottom: 16 }}
                />
              )}
              {!syncedInvestments.isLoading && syncedInvestments.investments.length > 0 && (
                <InvestmentsSummary investments={syncedInvestments.investments} />
              )}
              <InvestmentList
                investments={syncedInvestments.investments}
                isLoading={syncedInvestments.isLoading}
                onOpen={(investment: Investment) => navigate(`/investimentos/${investment.id}`)}
              />
            </>
          )}
        </>
      )}

      <InvestmentAccountForm
        open={accountFormOpen}
        financialAccounts={financialAccounts.accounts}
        submitting={workspace.isSaving}
        submitError={saveError}
        onSubmit={submitAccount}
        onCancel={() => setAccountFormOpen(false)}
      />
      <InvestmentAccountEditForm
        open={editingAccount !== null}
        account={editingAccount}
        financialAccounts={financialAccounts.accounts}
        submitting={workspace.isSaving}
        submitError={saveError}
        onSubmit={submitAccountEdit}
        onCancel={() => setEditingAccount(null)}
      />
      <InvestmentPortfolioForm
        open={portfolioFormOpen}
        portfolio={editingPortfolio}
        submitting={workspace.isSaving}
        submitError={saveError}
        onSubmit={submitPortfolio}
        onCancel={() => setPortfolioFormOpen(false)}
      />
      <InvestmentPositionForm
        open={positionFormOpen}
        position={editingPosition}
        accounts={positionAccounts ?? workspace.accounts}
        positions={workspace.positions}
        portfolios={workspace.portfolios}
        submitting={workspace.isSaving}
        submitError={saveError}
        onCreate={submitPositionCreate}
        onUpdate={submitPositionUpdate}
        onCancel={() => setPositionFormOpen(false)}
      />
      <InvestmentOperationForm
        open={operationFormOpen}
        operation={editingOperation}
        initial={operationAccountId === null ? null : { account_id: operationAccountId }}
        accounts={workspace.accounts}
        positions={workspace.positions}
        submitting={workspace.isSaving}
        submitError={saveError}
        onSubmit={submitOperation}
        onSubmitCompound={(operations) => void save(() => workspace.createOperations({ operations }), () => setOperationFormOpen(false), "Não foi possível salvar a movimentação.")}
        onCancel={() => setOperationFormOpen(false)}
      />
    </PageContainer>
  );
}
