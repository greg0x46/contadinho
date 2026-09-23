import { Alert, Button, Drawer, Flex, Input, Select } from "antd";
import { useEffect, useState } from "react";

import type { Account, InvestmentAccountWrite } from "../../api/contracts";

type Draft = { name: string; financialAccountId: string | null };

const blankDraft: Draft = { name: "", financialAccountId: null };

export function InvestmentAccountForm({
  open,
  financialAccounts,
  submitting,
  submitError,
  onSubmit,
  onCancel,
}: {
  open: boolean;
  financialAccounts: Account[];
  submitting: boolean;
  submitError: string | null;
  onSubmit: (write: InvestmentAccountWrite) => void;
  onCancel: () => void;
}) {
  const [draft, setDraft] = useState<Draft>(blankDraft);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (open) {
      setDraft(blankDraft);
      setError(null);
    }
  }, [open]);

  const submit = () => {
    if (draft.name.trim() === "") {
      setError("Informe o nome da conta de custódia.");
      return;
    }
    setError(null);
    onSubmit({
      name: draft.name.trim(),
      currency_code: "BRL",
      financial_account_id: draft.financialAccountId,
    });
  };

  const accountOptions = financialAccounts.filter((account) => account.currency_code === "BRL" && account.account_type !== "CREDIT").map((account) => ({
    value: account.id,
    label: [account.name, account.institution_name].filter(Boolean).join(" · ") || account.id,
  }));

  return (
    <Drawer
      title="Nova conta de custódia"
      open={open}
      onClose={onCancel}
      width={440}
      destroyOnHidden
      footer={
        <Flex justify="end" gap="small">
          <Button onClick={onCancel}>Cancelar</Button>
          <Button type="primary" loading={submitting} onClick={submit}>
            Criar conta
          </Button>
        </Flex>
      }
    >
      <Flex vertical gap="middle">
        {(error ?? submitError) && <Alert type="error" showIcon message={error ?? submitError} />}
        <Alert
          type="info"
          showIcon
          message="Conta manual em reais"
          description="Use uma conta de custódia para controlar posições e caixa. A integração bancária não poderá ser alterada por aqui."
        />
        <div className="filter-field">
          <label htmlFor="investment-account-name">Nome</label>
          <Input
            id="investment-account-name"
            value={draft.name}
            onChange={(event) => setDraft((current) => ({ ...current, name: event.target.value }))}
            placeholder="Ex.: Corretora XP"
            autoFocus
          />
        </div>
        <div className="filter-field">
          <label htmlFor="investment-financial-account">Conta bancária associada (opcional)</label>
          <Select
            id="investment-financial-account"
            value={draft.financialAccountId ?? undefined}
            options={accountOptions}
            placeholder="Selecione se o caixa vier de uma conta importada"
            allowClear
            showSearch
            optionFilterProp="label"
            onChange={(value: string | undefined) =>
              setDraft((current) => ({ ...current, financialAccountId: value ?? null }))
            }
          />
        </div>
      </Flex>
    </Drawer>
  );
}
