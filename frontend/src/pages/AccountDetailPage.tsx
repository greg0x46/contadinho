import { PageContainer } from "@ant-design/pro-layout";
import { Alert, Button, Flex } from "antd";
import { Link, useParams } from "react-router-dom";

import { isUuid } from "../api/contracts";
import { LoadingState, UnavailableState } from "../components/AsyncState";
import { AccountBillsTable } from "../components/accounts/AccountBillsTable";
import { AccountCardsTable } from "../components/accounts/AccountCardsTable";
import { AccountHeaderCard } from "../components/accounts/AccountHeaderCard";
import { AccountTransactionsTable } from "../components/accounts/AccountTransactionsTable";
import { useAccountDetail } from "../hooks/useAccountDetail";

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

  return (
    <PageContainer title="Detalhes da conta" extra={backLink}>
      <Flex vertical gap="large">
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

            <AccountHeaderCard
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

            <AccountTransactionsTable
              transactions={account.transactions}
              isLoading={account.transactionsLoading}
              error={account.transactionsError}
              accountId={id}
            />
          </>
        )}
      </Flex>
    </PageContainer>
  );
}

export function AccountDetailPage() {
  const { id = "" } = useParams();
  return isUuid(id) ? <ValidAccountDetail id={id} /> : <InvalidAccount />;
}
