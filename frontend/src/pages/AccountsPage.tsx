import { UploadOutlined } from "@ant-design/icons";
import { Alert, Button } from "antd";
import { useNavigate } from "react-router-dom";

import type { Account } from "../api/contracts";
import { AccountsSummary } from "../components/accounts/AccountsSummary";
import { BankAccountList } from "../components/accounts/BankAccountList";
import { CreditCardList } from "../components/accounts/CreditCardList";
import { BottomActionBar, Page } from "../components/layout";
import { useCompactScreen } from "../components/shared/useCompactScreen";
import { useAccounts } from "../hooks/useAccounts";

export function AccountsPage() {
  const accounts = useAccounts();
  const navigate = useNavigate();
  const compact = useCompactScreen();

  const openDetail = (account: Account) => navigate(`/contas-e-cartoes/${account.id}`);
  // Accounts with no type at all are bank accounts for display purposes:
  // only "CREDIT" gets the card treatment, since the limit and invoice
  // columns are meaningless without it.
  const bank = accounts.accounts.filter((account) => account.account_type !== "CREDIT");
  const credit = accounts.accounts.filter((account) => account.account_type === "CREDIT");

  // The compact header hides page actions on a phone, so the import action
  // moves to the bottom bar there.
  const importAction = (
    <Button icon={<UploadOutlined />} onClick={() => navigate("/contas-e-cartoes/importar")}>
      Importar extrato
    </Button>
  );

  return (
    <Page
      title="Contas e cartões"
      description="Saldos e detalhes das suas contas conectadas e importadas por arquivo"
      actions={importAction}
      className="accounts-page"
      compactMobileHeader
      hasBottomActionBar
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
      <div className="accounts-page-sections">
        {!accounts.isLoading && accounts.accounts.length > 0 && <AccountsSummary accounts={accounts.accounts} />}
        <BankAccountList accounts={bank} isLoading={accounts.isLoading} onOpen={openDetail} />
        <CreditCardList accounts={credit} isLoading={accounts.isLoading} onOpen={openDetail} />
      </div>
      {compact && <BottomActionBar>{importAction}</BottomActionBar>}
    </Page>
  );
}
