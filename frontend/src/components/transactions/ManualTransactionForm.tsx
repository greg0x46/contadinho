import { Alert, Button, DatePicker, Drawer, Flex, Input, InputNumber, Segmented, Select } from "antd";
import dayjs, { type Dayjs } from "dayjs";
import { useState } from "react";

import type { Account, Category, ManualTransactionWrite, TransactionItem } from "../../api/contracts";
import { categoryKindLabel, renderCategoryIcon } from "../../presentation/categoryLabels";

const dateFormat = "YYYY-MM-DD";

type Direction = "inflow" | "outflow";

type Draft = {
  accountId: string;
  description: string;
  direction: Direction;
  amount: number | null;
  date: Dayjs;
  categoryId: string | null;
};

function blankDraft(accountId: string): Draft {
  return { accountId, description: "", direction: "outflow", amount: null, date: dayjs(), categoryId: null };
}

function draftFrom(transaction: TransactionItem | null, defaultAccountId: string): Draft {
  if (!transaction) return blankDraft(defaultAccountId);
  return {
    accountId: transaction.account.id,
    description: transaction.description ?? "",
    direction: transaction.classification === "inflow" ? "inflow" : "outflow",
    amount: transaction.amount === null ? null : Math.abs(Number(transaction.amount)),
    date: transaction.occurred_at ? dayjs(transaction.occurred_at) : dayjs(),
    categoryId: transaction.internal_category?.id ?? null,
  };
}

function matchesDirection(kind: Category["kind"], direction: Direction): boolean {
  return direction === "inflow" ? kind === "income" || kind === "transfer" : kind === "expense" || kind === "transfer";
}

function categoryOptions(categories: Category[], direction: Direction) {
  const kindOrder = ["expense", "income", "transfer"] as const;
  return categories
    .filter((category) => category.is_active && matchesDirection(category.kind, direction))
    .sort((a, b) => {
      if (a.kind !== b.kind) return kindOrder.indexOf(a.kind) - kindOrder.indexOf(b.kind);
      return a.name.localeCompare(b.name, "pt-BR");
    })
    .map((category) => ({
      value: category.id,
      label: `${categoryKindLabel[category.kind]}: ${category.name}`,
      icon: category.icon,
      color: category.color,
    }));
}

/**
 * Creates or edits a lançamento manual — a transaction the user authors by
 * hand instead of it arriving through a Pluggy sync, on an account that
 * already exists (see .specs/lancamentos-manuais.md). transaction === null
 * means "create"; otherwise this edits that row's core fields (account,
 * description, signed amount, date). Category keeps going through the
 * existing category picker (TransactionDetailDrawer) once the lançamento
 * exists, but can also be set up front here.
 */
type FieldsProps = {
  /** The `<form>` id, so a submit button can live outside the fields (e.g. a sticky footer). */
  formId: string;
  transaction: TransactionItem | null;
  accounts: Account[];
  categories: Category[];
  submitError: string | null;
  onSubmit: (write: ManualTransactionWrite) => void;
};

/**
 * The fields of a lançamento manual, without a container. The draft is
 * seeded once on mount (remount with a `key` to start over) and rendered as
 * a `<form>` so the submit control can live in a drawer footer or a panel's
 * sticky bar via `form={formId}`.
 */
export function ManualTransactionFields({
  formId,
  transaction,
  accounts,
  categories,
  submitError,
  onSubmit,
}: FieldsProps) {
  const isEditing = transaction !== null;
  const [draft, setDraft] = useState<Draft>(() => draftFrom(transaction, accounts[0]?.id ?? ""));
  const [error, setError] = useState<string | null>(null);

  const accountOptions = accounts.map((account) => ({
    value: account.id,
    label: [account.name, account.institution_name].filter(Boolean).join(" · ") || account.id,
  }));

  const submit = () => {
    if (draft.accountId === "") {
      setError("Selecione uma conta.");
      return;
    }
    if (draft.description.trim() === "") {
      setError("Informe uma descrição.");
      return;
    }
    if (draft.amount === null || draft.amount <= 0) {
      setError("Informe um valor maior que zero.");
      return;
    }
    setError(null);
    const signed = draft.direction === "outflow" ? -draft.amount : draft.amount;
    onSubmit({
      account_id: draft.accountId,
      description: draft.description.trim(),
      amount: signed.toFixed(2),
      occurred_at: draft.date.format(dateFormat),
      category_id: draft.categoryId,
    });
  };

  return (
    <form
      id={formId}
      onSubmit={(event) => {
        event.preventDefault();
        submit();
      }}
    >
      <Flex vertical gap="middle">
        {(error ?? submitError) && <Alert type="error" showIcon message={error ?? submitError} />}

        <Alert
          type="info"
          showIcon
          message="O saldo da conta não muda"
          description="Ele sempre vem do banco. Isso só entra no extrato e nos totais."
        />

        <div className="filter-field">
          <label htmlFor="manual-transaction-account">Conta</label>
          <Select
            id="manual-transaction-account"
            value={draft.accountId || undefined}
            options={accountOptions}
            showSearch
            optionFilterProp="label"
            placeholder="Selecione uma conta"
            onChange={(value: string) => setDraft((current) => ({ ...current, accountId: value }))}
          />
        </div>

        <div className="filter-field">
          <label htmlFor="manual-transaction-description">Descrição</label>
          <Input
            id="manual-transaction-description"
            value={draft.description}
            onChange={(event) => setDraft((current) => ({ ...current, description: event.target.value }))}
            placeholder="Ex.: Almoço em dinheiro"
          />
        </div>

        <div className="filter-field">
          <label htmlFor="manual-transaction-direction">Direção</label>
          <Segmented
            id="manual-transaction-direction"
            value={draft.direction}
            options={[
              { value: "outflow", label: "Saída" },
              { value: "inflow", label: "Entrada" },
            ]}
            onChange={(value) => {
              const direction = value as Direction;
              setDraft((current) => {
                const selected = categories.find((category) => category.id === current.categoryId);
                const categoryId = selected && !matchesDirection(selected.kind, direction) ? null : current.categoryId;
                return { ...current, direction, categoryId };
              });
            }}
          />
        </div>

        <div className="filter-field">
          <label htmlFor="manual-transaction-amount">Valor</label>
          <InputNumber
            id="manual-transaction-amount"
            style={{ width: "100%" }}
            min={0.01}
            step={0.01}
            decimalSeparator=","
            value={draft.amount}
            onChange={(value) => setDraft((current) => ({ ...current, amount: value }))}
            placeholder="0,00"
          />
        </div>

        <div className="filter-field">
          <label htmlFor="manual-transaction-date">Data</label>
          <DatePicker
            id="manual-transaction-date"
            style={{ width: "100%" }}
            format="DD/MM/YYYY"
            value={draft.date}
            onChange={(value) => value && setDraft((current) => ({ ...current, date: value }))}
            allowClear={false}
          />
        </div>

        <div className="filter-field">
          <label htmlFor="manual-transaction-category">Categoria (opcional)</label>
          <Select
            id="manual-transaction-category"
            value={draft.categoryId ?? undefined}
            options={categoryOptions(categories, draft.direction)}
            allowClear={!isEditing}
            showSearch
            optionFilterProp="label"
            placeholder="Sem categoria"
            optionRender={(option) => (
              <span>
                <span style={{ color: option.data.color }} aria-hidden="true">
                  {renderCategoryIcon(option.data.icon)}
                </span>{" "}
                {option.label}
              </span>
            )}
            onChange={(value: string | undefined) =>
              setDraft((current) => ({ ...current, categoryId: value ?? null }))
            }
          />
        </div>
      </Flex>
    </form>
  );
}

const drawerFormId = "manual-transaction-form";

/** The fields inside a side drawer — how a new lançamento is created from the list. */
export function ManualTransactionForm({
  open,
  submitting,
  onCancel,
  ...fields
}: Omit<FieldsProps, "formId"> & {
  open: boolean;
  submitting: boolean;
  onCancel: () => void;
}) {
  const isEditing = fields.transaction !== null;
  return (
    <Drawer
      title={isEditing ? "Editar lançamento manual" : "Novo lançamento manual"}
      open={open}
      onClose={onCancel}
      width={420}
      destroyOnHidden
      footer={
        <Flex justify="end" gap="small">
          <Button onClick={onCancel}>Cancelar</Button>
          <Button type="primary" htmlType="submit" form={drawerFormId} loading={submitting}>
            Salvar
          </Button>
        </Flex>
      }
    >
      <ManualTransactionFields key={fields.transaction?.id ?? "new"} formId={drawerFormId} {...fields} />
    </Drawer>
  );
}
