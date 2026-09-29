import { useState } from "react";
import { Link, useParams } from "react-router-dom";

import { isUuid, type ManualTransactionWrite } from "../api/contracts";
import { AccountBillsTable } from "../components/accounts/AccountBillsTable";
import { AccountCardsTable } from "../components/accounts/AccountCardsTable";
import { AccountSummary } from "../components/accounts/AccountSummary";
import { AccountRecentTransactions } from "../components/accounts/AccountRecentTransactions";
import { DetailPage, InvalidDetailPage } from "../components/layout";
import { TransactionPanel } from "../components/transactions/TransactionPanel";
import { useAccountDetail } from "../hooks/useAccountDetail";
import { useAccounts } from "../hooks/useAccounts";
import { useCategories } from "../hooks/useCategories";
import { useManualTransaction } from "../hooks/useManualTransaction";
import { useTransactionCategory } from "../hooks/useTransactionCategory";
import { useTransactionInclusion } from "../hooks/useTransactionInclusion";
import { manualTransactionErrorMessage } from "../presentation/manualTransactionErrors";

const pageTitle = "Detalhes da conta";
const backLink = <Link to="/contas-e-cartoes">Voltar para contas e cartões</Link>;

function ValidAccountDetail({ id }: { id: string }) {
  const account = useAccountDetail(id);

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
    <DetailPage
      className="accounts-page"
      title={pageTitle}
      back={backLink}
      state={account.state}
      retry={account.retry}
      loadingLabel="Carregando conta…"
      notFoundMessage="Conta não encontrada"
      notFoundDescription="Não existe uma conta com este identificador."
      unavailableMessage="Não foi possível consultar esta conta agora."
    >
      {(snapshot) => (
        <>
          <AccountSummary
            account={snapshot}
            onSaveClosingDay={account.setClosingDay}
            savingClosingDay={account.savingClosingDay}
          />

          {snapshot.account_type === "CREDIT" && (
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
        </>
      )}
    </DetailPage>
  );
}

export function AccountDetailPage() {
  const { id = "" } = useParams();
  return isUuid(id) ? (
    <ValidAccountDetail id={id} />
  ) : (
    <InvalidDetailPage
      className="accounts-page"
      title={pageTitle}
      back={backLink}
      invalidTitle="Endereço de conta inválido"
    />
  );
}
