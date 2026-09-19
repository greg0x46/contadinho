import { Alert, Button, Drawer, Flex, Input, Select, Switch } from "antd";
import { useEffect, useState } from "react";

import type { Account, InvestmentAccount, InvestmentAccountUpdate } from "../../api/contracts";

/**
 * InvestmentAccountForm only creates. Editing a custody account is a much
 * smaller decision — the name and whether it is still in use — so it gets its
 * own short form instead of a create form with half its fields disabled.
 */
export function InvestmentAccountEditForm({
  open,
  account,
  financialAccounts,
  submitting,
  submitError,
  onSubmit,
  onCancel,
}: {
  open: boolean;
  account: InvestmentAccount | null;
  financialAccounts: Account[];
  submitting: boolean;
  submitError: string | null;
  onSubmit: (write: InvestmentAccountUpdate) => void;
  onCancel: () => void;
}) {
  const [name, setName] = useState("");
  const [financialAccountId, setFinancialAccountId] = useState<string | null>(null);
  const [active, setActive] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (open && account) {
      setName(account.name);
      setActive(account.active);
      setFinancialAccountId(account.financial_account_id);
      setError(null);
    }
  }, [open, account]);

  const submit = () => {
    if (name.trim() === "") {
      setError("Informe o nome da conta de custódia.");
      return;
    }
    setError(null);
    onSubmit({ name: name.trim(), active, financial_account_id: financialAccountId });
  };

  return (
    <Drawer
      title="Editar conta de custódia"
      open={open}
      onClose={onCancel}
      width={440}
      destroyOnHidden
      footer={
        <Flex justify="end" gap="small">
          <Button onClick={onCancel}>Cancelar</Button>
          <Button type="primary" loading={submitting} onClick={submit}>
            Salvar
          </Button>
        </Flex>
      }
    >
      <Flex vertical gap="middle">
        {(error ?? submitError) && <Alert type="error" showIcon message={error ?? submitError} />}
        <div className="filter-field">
          <label htmlFor="investment-account-edit-name">Nome</label>
          <Input
            id="investment-account-edit-name"
            disabled={account?.kind === "integrated"}
            value={name}
            autoFocus
            onChange={(event) => setName(event.target.value)}
          />
        </div>
        <div className="filter-field">
          <label htmlFor="investment-account-edit-financial">Caixa da corretora já importado</label>
          <Select id="investment-account-edit-financial" allowClear value={financialAccountId ?? undefined}
            options={financialAccounts.filter((item) => item.currency_code === "BRL" && item.account_type !== "CREDIT").map((item) => ({ value: item.id, label: item.name ?? item.id }))}
            onChange={(value: string | undefined) => setFinancialAccountId(value ?? null)} placeholder="Nenhuma conta vinculada" />
          {/* The Select's clear icon only shows on hover, which hides the fact
              that a link can be undone; the button makes it a visible action. */}
          {financialAccountId !== null && (
            <Button type="link" size="small" style={{ justifySelf: "start", paddingInline: 0 }}
              onClick={() => setFinancialAccountId(null)}>
              Desvincular conta
            </Button>
          )}
        </div>
        <div className="filter-field">
          <label htmlFor="investment-account-edit-active">Conta em uso</label>
          <Switch
            id="investment-account-edit-active"
            disabled={account?.kind === "integrated"}
            checked={active}
            onChange={setActive}
            checkedChildren="Sim"
            unCheckedChildren="Não"
          />
        </div>
      </Flex>
    </Drawer>
  );
}
