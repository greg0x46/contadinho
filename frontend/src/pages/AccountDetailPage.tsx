import { PageContainer } from "@ant-design/pro-layout";
import { Alert, Button } from "antd";
import { useState } from "react";
import { Link, useParams } from "react-router-dom";

import { isUuid, type ManualTransactionWrite } from "../api/contracts";
import { LoadingState, UnavailableState } from "../components/AsyncState";
import { AccountBillsTable } from "../components/accounts/AccountBillsTable";
import { AccountCardsTable } from "../components/accounts/AccountCardsTable";
import { AccountHeader } from "../components/accounts/AccountHeader";
import { AccountRecentTransactions } from "../components/accounts/AccountRecentTransactions";
import { TransactionPanel } from "../components/transactions/TransactionPanel";
import { useAccountDetail } from "../hooks/useAccountDetail";
import { useAccounts } from "../hooks/useAccounts";
import { useCategories } from "../hooks/useCategories";
import { useManualTransaction } from "../hooks/useManualTransaction";
import { useTransactionCategory } from "../hooks/useTransactionCategory";
import { useTransactionInclusion } from "../hooks/useTransactionInclusion";
import { manualTransactionErrorMessage } from "../presentation/manualTransactionErrors";

const backLink = <Link to="/contas-e-cartoes">Voltar para contas e cartões</Link>;

function InvalidAccount() {
  return (
    <Alert
      type="error"
      showIcon
      message={<h1>Endereço de conta inválido</h1>}
      description="O identificador informado não possui o formato esperado."
      action={backLink}
    />
  );
}

function ValidAccountDetail({ id }: { id: string }) {
  const account = useAccountDetail(id);
  const snapshot = account.state.snapshot;
  const isCredit = snapshot?.account_type === "CREDIT";

  // The recent-transactions panel reuses the exact same interaction the
  // Transações page offers — open a row, change its category/inclusion, edit
  // or delete a manual entry — through the same shared hooks, so a decision
  // made here and one made there never disagree.
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const inclusion = useTransactionInclusion();
  const category = useTransactionCategory();
  const categories = useCategories();
  const accounts = useAccounts();
  const manualTransaction = useManualTransaction();
  const [manualDeleteError, setManualDeleteError] = useState<string | null>(null);
  const selected = account.transactions.find((item) => item.id === selectedId) ?? null;

  const selectTransaction = (transactionId: string | null) => {
    setManualDeleteError(null);
    setSelectedId(transactionId);
  };
  const updateManualTransaction = async (transactionId: string, write: ManualTransactionWrite) => {
    try {
      await manualTransaction.update({ transactionId, write });
    } catch (error) {
      throw new Error(manualTransactionErrorMessage(error, "save"));
    }
  };
  const deleteManualTransactionAndClose = async (transactionId: string) => {
    setManualDeleteError(null);
    try {
      await manualTransaction.remove(transactionId);
      setSelectedId(null);
    } catch (error) {
      setManualDeleteError(manualTransactionErrorMessage(error, "delete"));
    }
  };

  return (
    <PageContainer className="accounts-page" title="Detalhes da conta" extra={backLink}>
      <div className="accounts-page-sections">
        {account.state.freshness === "loading" && <LoadingState>Carregando conta…</LoadingState>}
        {account.state.freshness === "not_found" && (
          <Alert
            type="error"
            showIcon
            message="Conta não encontrada"
            description="Não existe uma conta com este identificador."
            action={backLink}
          />
        )}
        {account.state.freshness === "unavailable" && (
          <UnavailableState onRetry={account.retry}>
            Não foi possível consultar esta conta agora.
          </UnavailableState>
        )}
        {snapshot !== null && (
          <>
            {account.state.freshness === "stale" && (
              <Alert
                type="warning"
                showIcon
                message="As informações podem estar desatualizadas."
                action={
                  <Button loading={account.state.retrying} onClick={account.retry}>
                    Tentar novamente
                  </Button>
                }
              />
            )}

            <AccountHeader
              account={snapshot}
              onSaveClosingDay={account.setClosingDay}
              savingClosingDay={account.savingClosingDay}
            />

            {isCredit && (
              <>
                <AccountCardsTable
                  cards={account.cards}
                  isLoading={account.cardsLoading}
                  error={account.cardsError}
                />
                <AccountBillsTable
                  bills={account.bills}
                  isLoading={account.billsLoading}
                  error={account.billsError}
                />
              </>
            )}

            <AccountRecentTransactions
              transactions={account.transactions}
              isLoading={account.transactionsLoading}
              error={account.transactionsError}
              accountId={id}
              selectedId={selectedId}
              onSelect={selectTransaction}
              onInclusion={(transactionId, target) =>
                inclusion.setInclusion({ transactionId, state: target })
              }
              pendingTransactionId={inclusion.pendingTarget?.transactionId}
            />
          </>
        )}
      </div>
      <TransactionPanel
        item={selected}
        categories={categories.categories}
        accounts={accounts.accounts}
        onClose={() => selectTransaction(null)}
        onInclusion={(transactionId, target) =>
          inclusion.setInclusion({ transactionId, state: target })
        }
        inclusionPending={inclusion.pendingTarget?.transactionId === selected?.id}
        onCategory={(transactionId, categoryId) => category.setCategory({ transactionId, categoryId })}
        categoryPending={category.pendingTarget?.transactionId === selected?.id}
        onSaveManual={updateManualTransaction}
        saveManualPending={manualTransaction.isUpdating}
        onDeleteManual={deleteManualTransactionAndClose}
        deleteManualPending={manualTransaction.isRemoving}
        deleteManualError={manualDeleteError}
      />
    </PageContainer>
  );
}

export function AccountDetailPage() {
  const { id = "" } = useParams();
  return isUuid(id) ? <ValidAccountDetail id={id} /> : <InvalidAccount />;
}
