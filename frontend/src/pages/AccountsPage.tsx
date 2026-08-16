import { PageContainer } from "@ant-design/pro-layout";
import { Alert, Button, Flex } from "antd";
import { useNavigate } from "react-router-dom";

import type { Account } from "../api/contracts";
import { AccountsSummary } from "../components/accounts/AccountsSummary";
import { BankAccountList } from "../components/accounts/BankAccountList";
import { CreditCardList } from "../components/accounts/CreditCardList";
import { useAccounts } from "../hooks/useAccounts";

export function AccountsPage() {
  const accounts = useAccounts();
  const navigate = useNavigate();

  const openDetail = (account: Account) => navigate(`/contas-e-cartoes/${account.id}`);
  // Accounts with no type at all are bank accounts for display purposes:
  // only "CREDIT" gets the card treatment, since the limit and invoice
  // columns are meaningless without it.
  const bank = accounts.accounts.filter((account) => account.account_type !== "CREDIT");
  const credit = accounts.accounts.filter((account) => account.account_type === "CREDIT");

  return (
    <PageContainer
      title="Contas e cartões"
      subTitle="Veja saldo, limite e detalhes de cada conta sincronizada"
      content="As contas e os cartões são importados automaticamente da sua instituição financeira."
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
      {!accounts.isLoading && accounts.accounts.length > 0 && (
        <AccountsSummary accounts={accounts.accounts} />
      )}
      <Flex vertical gap="large">
        <BankAccountList accounts={bank} isLoading={accounts.isLoading} onOpen={openDetail} />
        <CreditCardList accounts={credit} isLoading={accounts.isLoading} onOpen={openDetail} />
      </Flex>
    </PageContainer>
  );
}
