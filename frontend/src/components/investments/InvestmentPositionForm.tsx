import { Alert, Button, DatePicker, Drawer, Flex, Input, InputNumber, Select, Typography } from "antd";
import dayjs, { type Dayjs } from "dayjs";
import { useEffect, useMemo, useState } from "react";

import type {
  InvestmentAccount,
  InvestmentPortfolio,
  InvestmentPosition,
  InvestmentPositionUpdate,
  InvestmentPositionWrite,
} from "../../api/contracts";

type Draft = {
  accountId: string;
  assetId: string | null;
  portfolioId: string | null;
  name: string;
  ticker: string;
  assetType: string;
  initialQuantity: string | null;
  initialUnitCost: string | null;
  initialValue: string | null;
  occurredOn: Dayjs;
  notes: string;
};

function blankDraft(accounts: InvestmentAccount[]): Draft {
  return {
    accountId: accounts.find((account) => account.kind === "manual" && account.active)?.id ?? "",
    assetId: null,
    portfolioId: null,
    name: "",
    ticker: "",
    assetType: "Ativo",
    initialQuantity: null,
    initialUnitCost: null,
    initialValue: null,
    occurredOn: dayjs(),
    notes: "",
  };
}

function draftFrom(position: InvestmentPosition | null, accounts: InvestmentAccount[]): Draft {
  if (!position) return blankDraft(accounts);
  return {
    accountId: position.account_id,
    assetId: position.asset_id,
    portfolioId: position.portfolio_id,
    name: position.name,
    ticker: position.ticker ?? "",
    assetType: position.asset_type,
    initialQuantity: null,
    initialUnitCost: null,
    initialValue: null,
    occurredOn: dayjs(),
    notes: position.notes ?? "",
  };
}

function positive(value: string | null): boolean {
  return value !== null && Number(value) > 0;
}

export function InvestmentPositionForm({
  open,
  position,
  accounts,
  positions,
  portfolios,
  submitting,
  submitError,
  onCreate,
  onUpdate,
  onCancel,
}: {
  open: boolean;
  position: InvestmentPosition | null;
  accounts: InvestmentAccount[];
  positions: InvestmentPosition[];
  portfolios: InvestmentPortfolio[];
  submitting: boolean;
  submitError: string | null;
  onCreate: (write: InvestmentPositionWrite) => void;
  onUpdate: (write: InvestmentPositionUpdate) => void;
  onCancel: () => void;
}) {
  const [draft, setDraft] = useState<Draft>(() => draftFrom(position, accounts));
  const [error, setError] = useState<string | null>(null);
  const isEditing = position !== null;

  useEffect(() => {
    if (open) {
      setDraft(draftFrom(position, accounts));
      setError(null);
    }
  }, [open, position, accounts]);

  const editableAccounts = useMemo(
    () => accounts.filter((account) => account.kind === "manual" && account.active),
    [accounts],
  );
  const accountOptions = editableAccounts.map((account) => ({ value: account.id, label: account.name }));
  const portfolioOptions = portfolios.map((portfolio) => ({ value: portfolio.id, label: portfolio.name }));
  const assetOptions = Array.from(new Map(
    positions.filter((item) => item.asset_id !== null).map((item) => [item.asset_id!, item]),
  ).values()).map((item) => ({ value: item.asset_id!, label: [item.ticker, item.name].filter(Boolean).join(" · ") }));

  const submit = () => {
    if (draft.name.trim() === "") {
      setError("Informe o nome da posição.");
      return;
    }
    if (draft.assetType.trim() === "") {
      setError("Informe o tipo do ativo.");
      return;
    }
    if (!isEditing && draft.accountId === "") {
      setError("Selecione uma conta manual de custódia.");
      return;
    }
    if ((draft.initialQuantity !== null && !positive(draft.initialQuantity)) || (draft.initialUnitCost !== null && !positive(draft.initialUnitCost))) {
      setError("Quantidade e custo unitário devem ser maiores que zero.");
      return;
    }
    if (!isEditing && ((draft.initialQuantity !== null || draft.initialUnitCost !== null) && !positive(draft.initialValue) && !(positive(draft.initialQuantity) && positive(draft.initialUnitCost)))) {
      setError("Para informar um saldo inicial, preencha quantidade e custo unitário, ou o valor inicial.");
      return;
    }
    setError(null);
    if (isEditing) {
      onUpdate({
        name: draft.name.trim(),
        ticker: draft.ticker.trim() || null,
        asset_type: draft.assetType.trim(),
        portfolio_id: draft.portfolioId,
        notes: draft.notes.trim() || null,
      });
      return;
    }
    onCreate({
      account_id: draft.accountId,
      asset_id: draft.assetId,
      name: draft.name.trim(),
      ticker: draft.ticker.trim() || null,
      asset_type: draft.assetType.trim(),
      portfolio_id: draft.portfolioId,
      initial_quantity: draft.initialQuantity ?? undefined,
      initial_unit_cost: draft.initialUnitCost ?? undefined,
      initial_value: draft.initialValue ?? undefined,
      occurred_on: draft.occurredOn.format("YYYY-MM-DD"),
      notes: draft.notes.trim() || null,
    });
  };

  const setDecimal = (field: "initialQuantity" | "initialUnitCost" | "initialValue", value: string | number | null) =>
    setDraft((current) => ({ ...current, [field]: value === null ? null : String(value) }));

  return (
    <Drawer
      title={isEditing ? "Editar posição manual" : "Nova posição manual"}
      open={open}
      onClose={onCancel}
      width={500}
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
        {!isEditing && (
          <Alert
            type="info"
            showIcon
            message="Posição manual em BRL"
            description="A cotação só muda quando você registrar uma cotação manual. Sem cotação, o valor exibido é o custo acumulado."
          />
        )}
        <div className="filter-field">
          <label htmlFor="investment-position-account">Conta de custódia</label>
          <Select
            id="investment-position-account"
            value={draft.accountId || undefined}
            options={accountOptions}
            disabled={isEditing}
            placeholder="Selecione uma conta manual"
            onChange={(value: string) => setDraft((current) => ({ ...current, accountId: value }))}
          />
          {isEditing && <Typography.Text type="secondary">A conta é definida na criação.</Typography.Text>}
        </div>
        {!isEditing && assetOptions.length > 0 && (
          <div className="filter-field">
            <label htmlFor="investment-position-asset">Ativo existente (opcional)</label>
            <Select
              id="investment-position-asset"
              value={draft.assetId ?? undefined}
              options={assetOptions}
              allowClear
              showSearch
              optionFilterProp="label"
              placeholder="Cadastre um novo ativo abaixo"
              onChange={(value: string | undefined) => {
                const selected = positions.find((item) => item.asset_id === value);
                setDraft((current) => ({ ...current, assetId: value ?? null,
                  name: selected?.name ?? current.name, ticker: selected?.ticker ?? current.ticker,
                  assetType: selected?.asset_type ?? current.assetType }));
              }}
            />
            <Typography.Text type="secondary">O mesmo ativo pode ser mantido em várias contas.</Typography.Text>
          </div>
        )}
        <div className="filter-field">
          <label htmlFor="investment-position-name">Nome</label>
          <Input
            id="investment-position-name"
            value={draft.name}
            autoFocus
            onChange={(event) => setDraft((current) => ({ ...current, name: event.target.value }))}
            placeholder="Ex.: Tesouro Selic 2029"
          />
        </div>
        <Flex gap="middle" wrap>
          <div className="filter-field" style={{ flex: 1, minWidth: 180 }}>
            <label htmlFor="investment-position-ticker">Código (opcional)</label>
            <Input
              id="investment-position-ticker"
              value={draft.ticker}
              onChange={(event) => setDraft((current) => ({ ...current, ticker: event.target.value }))}
              placeholder="Ex.: BOVA11"
            />
          </div>
          <div className="filter-field" style={{ flex: 1, minWidth: 180 }}>
            <label htmlFor="investment-position-type">Tipo do ativo</label>
            <Input
              id="investment-position-type"
              value={draft.assetType}
              onChange={(event) => setDraft((current) => ({ ...current, assetType: event.target.value }))}
              placeholder="Ex.: ETF, CDB, Fundo"
            />
          </div>
        </Flex>
        <div className="filter-field">
          <label htmlFor="investment-position-goal">Objetivo (opcional)</label>
          <Select
            id="investment-position-goal"
            value={draft.portfolioId ?? undefined}
            options={portfolioOptions}
            allowClear
            placeholder="Sem objetivo"
            showSearch
            optionFilterProp="label"
            onChange={(value: string | undefined) => setDraft((current) => ({ ...current, portfolioId: value ?? null }))}
          />
        </div>
        {!isEditing && (
          <>
            <Flex gap="middle" wrap>
              <div className="filter-field" style={{ flex: 1, minWidth: 140 }}>
                <label htmlFor="investment-position-quantity">Quantidade inicial</label>
                <InputNumber
                  id="investment-position-quantity"
                  style={{ width: "100%" }}
                  min="0.00000001"
                  stringMode
                  decimalSeparator=","
                  value={draft.initialQuantity}
                  onChange={(value) => setDecimal("initialQuantity", value)}
                  placeholder="Opcional se informar o valor"
                />
              </div>
              <div className="filter-field" style={{ flex: 1, minWidth: 140 }}>
                <label htmlFor="investment-position-unit-cost">Custo unitário</label>
                <InputNumber
                  id="investment-position-unit-cost"
                  style={{ width: "100%" }}
                  min="0.00000001"
                  stringMode
                  decimalSeparator=","
                  value={draft.initialUnitCost}
                  onChange={(value) => setDecimal("initialUnitCost", value)}
                  placeholder="Opcional se informar o valor"
                />
              </div>
            </Flex>
            <div className="filter-field">
              <label htmlFor="investment-position-value">Valor inicial</label>
              <InputNumber
                id="investment-position-value"
                style={{ width: "100%" }}
                min="0.01"
                step="0.01"
                stringMode
                decimalSeparator=","
                value={draft.initialValue}
                onChange={(value) => setDecimal("initialValue", value)}
                placeholder="Informe este valor ou quantidade e custo unitário"
              />
            </div>
            <div className="filter-field">
              <label htmlFor="investment-position-date">Data do saldo inicial</label>
              <DatePicker
                id="investment-position-date"
                style={{ width: "100%" }}
                value={draft.occurredOn}
                allowClear={false}
                format="DD/MM/YYYY"
                onChange={(value) => value && setDraft((current) => ({ ...current, occurredOn: value }))}
              />
            </div>
          </>
        )}
        <div className="filter-field">
          <label htmlFor="investment-position-notes">Observações (opcional)</label>
          <Input.TextArea
            id="investment-position-notes"
            value={draft.notes}
            rows={3}
            onChange={(event) => setDraft((current) => ({ ...current, notes: event.target.value }))}
          />
        </div>
      </Flex>
    </Drawer>
  );
}
