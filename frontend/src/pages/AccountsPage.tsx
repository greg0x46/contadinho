import { Alert, Button, Skeleton } from "antd";
import { useNavigate } from "react-router-dom";

import type { Account } from "../api/contracts";
import { AccountList } from "../components/accounts/AccountList";
import { CreditCardList } from "../components/accounts/CreditCardList";
import { FinancialSummary } from "../components/accounts/FinancialSummary";
import { DataCard, Page } from "../components/layout";
import { useAccounts } from "../hooks/useAccounts";

export function AccountsPage() {
  const accounts = useAccounts();
  const navigate = useNavigate();

  const openDetail = (account: Account) => navigate(`/contas-e-cartoes/${account.id}`);
  // Accounts with no type at all are bank accounts for display purposes:
  // only "CREDIT" gets the card treatment, since the limit and invoice
  // fields are meaningless without it.
  const bank = accounts.accounts.filter((account) => account.account_type !== "CREDIT");
  const credit = accounts.accounts.filter((account) => account.account_type === "CREDIT");
  const hasAccounts = accounts.accounts.length > 0;

  return (
    <Page
      title="Contas e cartões"
      description="Veja saldos, limites e detalhes das suas contas sincronizadas."
      className="accounts-page"
    >
      {accounts.error && (
        <Alert
          type="error"
          showIcon
          message="Não foi possível carregar as contas"
          action={<Button onClick={() => accounts.refetch()}>Tentar novamente</Button>}
          style={{ marginBottom: 16 }}
        />
      )}

      <DataCard
        className="accounts-card"
        summary={
          hasAccounts ? (
            <FinancialSummary accounts={accounts.accounts} busy={accounts.isLoading} />
          ) : accounts.isLoading ? (
            <section className="data-card-summary" aria-label="Carregando resumo financeiro" aria-busy="true">
              <Skeleton active paragraph={{ rows: 1 }} title={false} />
            </section>
          ) : undefined
        }
      >
        <section className="accounts-section" aria-labelledby="accounts-section-bank">
          <h2 id="accounts-section-bank" className="accounts-section-title">
            Contas bancárias
          </h2>
          <AccountList accounts={bank} isLoading={accounts.isLoading} onOpen={openDetail} />
        </section>

        <section className="accounts-section" aria-labelledby="accounts-section-credit">
          <h2 id="accounts-section-credit" className="accounts-section-title">
            Cartões de crédito
          </h2>
          <CreditCardList accounts={credit} isLoading={accounts.isLoading} onOpen={openDetail} />
        </section>
      </DataCard>
    </Page>
  );
}
