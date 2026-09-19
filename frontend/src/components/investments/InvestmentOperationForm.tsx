import { Alert, Button, DatePicker, Drawer, Flex, Input, InputNumber, Select, Checkbox, Typography } from "antd";
import dayjs, { type Dayjs } from "dayjs";
import { useEffect, useMemo, useRef, useState } from "react";

import type {
  InvestmentAccount,
  InvestmentOperation,
  InvestmentOperationKind,
  InvestmentOperationWrite,
  InvestmentPosition,
} from "../../api/contracts";
import { isZeroDecimal, multiplyDecimals, subtractDecimals, sumDecimals } from "../../presentation/decimal";
import { investmentOperationKindLabel } from "../../presentation/investmentWorkspaceLabels";

type Draft = {
  accountId: string;
  positionId: string | null;
  kind: InvestmentOperationKind;
  occurredOn: Dayjs;
  amount: string | null;
  quantity: string | null;
  unitPrice: string | null;
  fees: string | null;
  taxes: string | null;
  notes: string;
};

function blankDraft(accounts: InvestmentAccount[], initial?: Partial<InvestmentOperationWrite>): Draft {
  return {
    accountId: initial?.account_id ?? accounts.find((account) => account.active)?.id ?? "",
    positionId: initial?.position_id ?? null,
    kind: initial?.kind ?? "deposit",
    occurredOn: initial?.occurred_on ? dayjs(initial.occurred_on) : dayjs(),
    amount: initial?.amount ?? null,
    quantity: initial?.quantity ?? null,
    unitPrice: initial?.unit_price ?? null,
    fees: initial?.fees ?? null,
    taxes: initial?.taxes ?? null,
    notes: initial?.notes ?? "",
  };
}

function draftFrom(
  operation: InvestmentOperation | null,
  accounts: InvestmentAccount[],
  initial?: Partial<InvestmentOperationWrite>,
): Draft {
  if (!operation) return blankDraft(accounts, initial);
  return {
    accountId: operation.account_id,
    positionId: operation.position_id,
    kind: operation.kind,
    occurredOn: dayjs(operation.occurred_on),
    amount: isDerivedAmount(operation) ? null : operation.amount,
    quantity: operation.quantity,
    unitPrice: operation.unit_price,
    fees: operation.fees,
    taxes: operation.taxes,
    notes: operation.notes ?? "",
  };
}

/**
 * A trade whose amount is exactly quantity × unit price was computed by the
 * server, not typed. Loading it back as empty keeps it out of the PUT, so a
 * corrected quantity or price is priced again instead of inheriting the old
 * total (20 cotas por R$ 990).
 */
function isDerivedAmount(operation: InvestmentOperation): boolean {
  if (operation.quantity === null || operation.unit_price === null) return false;
  return isZeroDecimal(subtractDecimals(operation.amount, multiplyDecimals(operation.quantity, operation.unit_price)));
}

const kindsNeedingPosition: InvestmentOperationKind[] = ["buy", "sell", "valuation"];
const tradeKinds: InvestmentOperationKind[] = ["buy", "sell"];
const kindOptions = (Object.keys(investmentOperationKindLabel) as InvestmentOperationKind[])
  .filter((value) => value !== "transfer_out" && value !== "transfer_in")
  .map((value) => ({
  value,
  label: investmentOperationKindLabel[value],
}));

function greaterThanZero(value: string | null): boolean {
  return value !== null && Number(value) > 0;
}

function isNegative(value: string | null): boolean {
  return value !== null && Number(value) < 0;
}

export function InvestmentOperationForm({
  open,
  operation,
  initial,
  accounts,
  positions,
  submitting,
  submitError,
  onSubmit,
  onCancel,
  onSubmitCompound,
  allowedKinds,
}: {
  open: boolean;
  operation: InvestmentOperation | null;
  initial?: Partial<InvestmentOperationWrite> | null;
  accounts: InvestmentAccount[];
  positions: InvestmentPosition[];
  submitting: boolean;
  submitError: string | null;
  onSubmit: (write: InvestmentOperationWrite) => void;
  onCancel: () => void;
  onSubmitCompound?: (writes: InvestmentOperationWrite[]) => void;
  allowedKinds?: InvestmentOperationKind[];
}) {
  const [draft, setDraft] = useState<Draft>(() => draftFrom(operation, accounts, initial ?? undefined));
  const [error, setError] = useState<string | null>(null);
  const [funding, setFunding] = useState<"deposit" | "income" | null>(null);
  // Whether the person typed into "Valor bruto" since the drawer opened. Until
  // then a trade's amount follows quantity × price, so changing either clears it.
  const [amountTouched, setAmountTouched] = useState(false);
  const integrated = accounts.find((account) => account.id === draft.accountId)?.kind === "integrated";
  const isEditing = operation !== null;
  const isTrade = tradeKinds.includes(draft.kind) || (draft.kind === "initial_balance" && draft.positionId !== null);
  const needsPosition = kindsNeedingPosition.includes(draft.kind) || (draft.kind === "initial_balance" && draft.positionId !== null);

  // Callers pass `initial` as a fresh literal on every render and `accounts`
  // changes identity on every refetch. Neither should wipe what the person
  // typed (a 409 on save re-renders the parent), so the draft only resets when
  // the drawer opens or switches operation; the latest props are read from a ref.
  const latest = useRef({ initial, accounts });
  useEffect(() => {
    latest.current = { initial, accounts };
  });
  useEffect(() => {
    if (open) {
      setDraft(draftFrom(operation, latest.current.accounts, latest.current.initial ?? undefined));
      setError(null);
      setFunding(null);
      setAmountTouched(false);
    }
  }, [open, operation]);
  // Accounts may still be loading when the drawer opens; fill the account in
  // once they arrive instead of leaving the select empty.
  useEffect(() => {
    if (!open || accounts.length === 0) return;
    setDraft((current) => {
      if (current.accountId !== "") return current;
      const accountId = latest.current.initial?.account_id ?? accounts.find((account) => account.active)?.id ?? "";
      return accountId === "" ? current : { ...current, accountId };
    });
  }, [open, accounts]);

  const writableAccounts = useMemo(
    () => accounts.filter((account) => account.active),
    [accounts],
  );
  const accountOptions = (isEditing ? accounts : writableAccounts).map((account) => ({
    value: account.id,
    label: account.name,
  }));
  const positionOptions = positions
    .filter((position) => position.account_id === draft.accountId && position.source === "manual")
    .map((position) => ({
      value: position.id,
      label: [position.ticker, position.name].filter(Boolean).join(" · "),
    }));

  const setDecimal = (
    field: "amount" | "quantity" | "unitPrice" | "fees" | "taxes",
    value: string | number | null,
  ) => {
    const next = value === null ? null : String(value);
    if (field === "amount") setAmountTouched(true);
    const clearsAmount = (field === "quantity" || field === "unitPrice") && !amountTouched;
    setDraft((current) => ({ ...current, [field]: next, ...(clearsAmount ? { amount: null } : {}) }));
  };

  const submit = () => {
    if (draft.accountId === "") {
      setError("Selecione uma conta de custódia.");
      return;
    }
    if (needsPosition && draft.positionId === null) {
      setError("Selecione uma posição para esta movimentação.");
      return;
    }
    if (isTrade && (!greaterThanZero(draft.quantity) || !greaterThanZero(draft.unitPrice))) {
      setError("Informe quantidade e preço unitário maiores que zero.");
      return;
    }
    if (!isTrade && draft.kind !== "valuation" && !greaterThanZero(draft.amount)) {
      setError("Informe um valor maior que zero.");
      return;
    }
    if (draft.kind === "valuation" && (draft.amount === null || Number(draft.amount) < 0)) {
      setError("Informe o valor total atual da posição.");
      return;
    }
    if (isNegative(draft.fees) || isNegative(draft.taxes)) {
      setError("Taxas e impostos não podem ser negativos.");
      return;
    }
    setError(null);
    const write: InvestmentOperationWrite = {
      account_id: draft.accountId,
      position_id: needsPosition ? draft.positionId : null,
      kind: draft.kind,
      occurred_on: draft.occurredOn.format("YYYY-MM-DD"),
      amount: draft.amount ?? undefined,
      quantity: isTrade ? draft.quantity : null,
      unit_price: isTrade ? draft.unitPrice : null,
      fees: isTrade ? draft.fees : null,
      taxes: isTrade ? draft.taxes : null,
      notes: draft.notes.trim() || null,
    };
    if (funding && isTrade && draft.kind === "buy" && onSubmitCompound && !isEditing) {
      const amount = sumDecimals([draft.amount ?? multiplyDecimals(draft.quantity!, draft.unitPrice!), draft.fees ?? "0", draft.taxes ?? "0"]);
      onSubmitCompound([{ account_id: draft.accountId, position_id: null, kind: funding, occurred_on: write.occurred_on, amount }, write]);
    } else {
      onSubmit(write);
    }
  };

  return (
    <Drawer
      title={isEditing ? "Editar movimentação manual" : "Registrar movimentação"}
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
        {draft.kind === "valuation" && (
          <Alert
            type="info"
            showIcon
            message="Cotação manual"
            description="Ela marca o valor atual desta posição nesta data. A rentabilidade permanece desconhecida se não houver base suficiente."
          />
        )}
        <div className="filter-field">
          <label htmlFor="investment-operation-account">Conta de custódia</label>
          <Select
            id="investment-operation-account"
            value={draft.accountId || undefined}
            options={accountOptions}
            disabled={isEditing}
            placeholder="Selecione uma conta de custódia"
            onChange={(value: string) =>
              setDraft((current) => ({ ...current, accountId: value, positionId: null, kind: initial?.kind ?? "deposit" }))
            }
          />
        </div>
        <div className="filter-field">
          <label htmlFor="investment-operation-kind">Tipo</label>
          <Select
            id="investment-operation-kind"
            value={draft.kind}
            options={kindOptions.filter(({ value }) => (!allowedKinds || allowedKinds.includes(value)) && (!integrated || ["deposit", "withdrawal", "income", "fee", "tax"].includes(value)))}
            disabled={isEditing}
            onChange={(value: InvestmentOperationKind) =>
              setDraft((current) => ({ ...current, kind: value }))
            }
          />
        </div>
        {(needsPosition || draft.kind === "initial_balance") && (
        <div className="filter-field">
          <label htmlFor="investment-operation-position">
            Posição {needsPosition ? "" : "(opcional)"}
          </label>
          <Select
            id="investment-operation-position"
            value={draft.positionId ?? undefined}
            options={positionOptions}
            allowClear={!needsPosition}
            placeholder={needsPosition ? "Selecione uma posição" : "Movimentação só de caixa"}
            showSearch
            optionFilterProp="label"
            onChange={(value: string | undefined) =>
              setDraft((current) => ({ ...current, positionId: value ?? null }))
            }
          />
        </div>
        )}
        <div className="filter-field">
          <label htmlFor="investment-operation-date">Data</label>
          <DatePicker
            id="investment-operation-date"
            style={{ width: "100%" }}
            value={draft.occurredOn}
            allowClear={false}
            format="DD/MM/YYYY"
            onChange={(value) => value && setDraft((current) => ({ ...current, occurredOn: value }))}
          />
        </div>
        {isTrade ? (
          <>
            <Flex gap="middle" wrap>
              <div className="filter-field" style={{ flex: 1, minWidth: 150 }}>
                <label htmlFor="investment-operation-quantity">Quantidade</label>
                <InputNumber
                  id="investment-operation-quantity"
                  style={{ width: "100%" }}
                  min="0.00000001"
                  stringMode
                  decimalSeparator=","
                  value={draft.quantity}
                  onChange={(value) => setDecimal("quantity", value)}
                />
              </div>
              <div className="filter-field" style={{ flex: 1, minWidth: 150 }}>
                <label htmlFor="investment-operation-unit-price">Preço unitário</label>
                <InputNumber
                  id="investment-operation-unit-price"
                  style={{ width: "100%" }}
                  min="0.00000001"
                  stringMode
                  decimalSeparator=","
                  value={draft.unitPrice}
                  onChange={(value) => setDecimal("unitPrice", value)}
                />
              </div>
            </Flex>
            <div className="filter-field">
              <label htmlFor="investment-operation-amount">Valor bruto (opcional)</label>
              <InputNumber
                id="investment-operation-amount"
                style={{ width: "100%" }}
                min="0"
                step="0.01"
                stringMode
                decimalSeparator=","
                value={draft.amount}
                onChange={(value) => setDecimal("amount", value)}
                placeholder="Calculado por quantidade × preço se vazio"
              />
            </div>
            <Flex gap="middle" wrap>
              <div className="filter-field" style={{ flex: 1, minWidth: 150 }}>
                <label htmlFor="investment-operation-fees">Taxas (opcional)</label>
                <InputNumber
                  id="investment-operation-fees"
                  style={{ width: "100%" }}
                  min="0"
                  step="0.01"
                  stringMode
                  decimalSeparator=","
                  value={draft.fees}
                  onChange={(value) => setDecimal("fees", value)}
                />
              </div>
              <div className="filter-field" style={{ flex: 1, minWidth: 150 }}>
                <label htmlFor="investment-operation-taxes">Impostos (opcional)</label>
                <InputNumber
                  id="investment-operation-taxes"
                  style={{ width: "100%" }}
                  min="0"
                  step="0.01"
                  stringMode
                  decimalSeparator=","
                  value={draft.taxes}
                  onChange={(value) => setDecimal("taxes", value)}
                />
              </div>
            </Flex>
          </>
        ) : (
          <div className="filter-field">
            <label htmlFor="investment-operation-amount">
              {draft.kind === "valuation" ? "Valor total atual" : "Valor"}
            </label>
            <InputNumber
              id="investment-operation-amount"
              style={{ width: "100%" }}
              min={draft.kind === "valuation" ? "0" : "0.01"}
              step="0.01"
              stringMode
              decimalSeparator=","
              value={draft.amount}
              onChange={(value) => setDecimal("amount", value)}
              placeholder="0,00"
            />
          </div>
        )}
        {isTrade && draft.kind === "buy" && !isEditing && onSubmitCompound && (
          <Flex vertical gap="small">
            <Checkbox checked={funding !== null} onChange={(event) => setFunding(event.target.checked ? "deposit" : null)}>
              Registrar entrada de dinheiro junto com a compra
            </Checkbox>
            {funding && <Select aria-label="Origem do dinheiro" value={funding} onChange={setFunding}
              options={[{ value: "deposit", label: "Aporte e compra" }, { value: "income", label: "Rendimento reinvestido" }]} />}
          </Flex>
        )}
        {integrated && <Alert type="info" message="Os saldos continuam sendo informados pela instituição. Esta movimentação registra o evento para os relatórios e a conciliação." />}
        <div className="filter-field">
          <label htmlFor="investment-operation-notes">Observações (opcional)</label>
          <Input.TextArea
            id="investment-operation-notes"
            rows={3}
            value={draft.notes}
            onChange={(event) => setDraft((current) => ({ ...current, notes: event.target.value }))}
          />
        </div>
        {isEditing && <Typography.Text type="secondary">Ao salvar, as quantidades, os custos e o caixa são recalculados.</Typography.Text>}
      </Flex>
    </Drawer>
  );
}
