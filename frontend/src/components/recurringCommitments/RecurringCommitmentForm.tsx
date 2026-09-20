import { Alert, Button, DatePicker, Divider, Drawer, Flex, Input, InputNumber, Select, Switch, Typography } from "antd";
import dayjs, { type Dayjs } from "dayjs";
import { useState } from "react";

import type {
  Category,
  RecurringCommitment,
  RecurringCommitmentCadence,
  RecurringCommitmentKind,
  RecurringCommitmentWrite,
} from "../../api/contracts";
import {
  recurringCommitmentCadenceLabel,
  recurringCommitmentKindLabel,
} from "../../presentation/recurringCommitmentLabels";

const dateFormat = "YYYY-MM-DD";

const kindOptions = (Object.keys(recurringCommitmentKindLabel) as RecurringCommitmentKind[]).map(
  (value) => ({ value, label: recurringCommitmentKindLabel[value] }),
);
const cadenceOptions = (
  Object.keys(recurringCommitmentCadenceLabel) as RecurringCommitmentCadence[]
).map((value) => ({ value, label: recurringCommitmentCadenceLabel[value] }));
const monthOptions = Array.from({ length: 12 }, (_, index) => ({
  value: index + 1,
  label: new Intl.DateTimeFormat("pt-BR", { month: "long" }).format(new Date(2000, index, 1)),
}));

export type RecurringCommitmentDraft = {
  name: string;
  kind: RecurringCommitmentKind;
  amount: number | null;
  categoryId: string | null;
  accountId: string;
  cadence: RecurringCommitmentCadence;
  dayOfMonth: number | null;
  monthOfYear: number | null;
  startDate: Dayjs;
  endDate: Dayjs | null;
  isActive: boolean;
};

const blankDraft: RecurringCommitmentDraft = {
  name: "",
  kind: "expense",
  amount: null,
  categoryId: null,
  accountId: "",
  cadence: "monthly",
  dayOfMonth: null,
  monthOfYear: null,
  startDate: dayjs(),
  endDate: null,
  isActive: true,
};

function draftFrom(commitment: RecurringCommitment | null): RecurringCommitmentDraft {
  if (!commitment) return blankDraft;
  return {
    name: commitment.name,
    kind: commitment.kind,
    amount: Number(commitment.amount),
    categoryId: commitment.category_id,
    accountId: commitment.account_id ?? "",
    cadence: commitment.cadence,
    dayOfMonth: commitment.day_of_month,
    monthOfYear: commitment.month_of_year,
    startDate: dayjs(commitment.start_date, dateFormat),
    endDate: commitment.end_date ? dayjs(commitment.end_date, dateFormat) : null,
    isActive: commitment.is_active,
  };
}

type FieldsProps = {
  /** The `<form>` id, so a submit button can live outside the fields (e.g. a sticky footer). */
  formId: string;
  commitment: RecurringCommitment | null;
  initialDraft?: Partial<RecurringCommitmentDraft> | null;
  categories: Category[];
  submitError: string | null;
  onSubmit: (write: RecurringCommitmentWrite) => void;
};

/**
 * The fields of a compromisso recorrente, without a container. The draft is
 * seeded once on mount (remount with a `key` to start over) and rendered as
 * a `<form>` so the submit control can live in a drawer footer or a panel's
 * sticky bar via `form={formId}`.
 */
export function RecurringCommitmentFields({
  formId,
  commitment,
  initialDraft,
  categories,
  submitError,
  onSubmit,
}: FieldsProps) {
  const [draft, setDraft] = useState<RecurringCommitmentDraft>(() => ({
    ...draftFrom(commitment),
    ...(commitment ? {} : initialDraft),
  }));
  const [error, setError] = useState<string | null>(null);

  const categoryOptions = categories
    .filter((category) => category.is_active && category.kind !== "transfer")
    .map((category) => ({ value: category.id, label: category.name }));

  const submit = () => {
    if (draft.name.trim() === "") {
      setError("Informe um nome para o compromisso.");
      return;
    }
    if (draft.amount === null || draft.amount <= 0) {
      setError("Informe um valor maior que zero.");
      return;
    }
    if (draft.categoryId === null) {
      setError("Selecione uma categoria.");
      return;
    }
    if (draft.dayOfMonth === null) {
      setError("Informe o dia do mês.");
      return;
    }
    if (draft.cadence === "annual" && draft.monthOfYear === null) {
      setError("Informe o mês do ano para uma recorrência anual.");
      return;
    }
    if (draft.endDate && draft.endDate.isBefore(draft.startDate, "day")) {
      setError("A data de término não pode ser anterior à data de início.");
      return;
    }
    setError(null);
    onSubmit({
      name: draft.name.trim(),
      kind: draft.kind,
      amount: draft.amount.toFixed(2),
      category_id: draft.categoryId,
      account_id: draft.accountId.trim() === "" ? null : draft.accountId.trim(),
      cadence: draft.cadence,
      day_of_month: draft.dayOfMonth,
      month_of_year: draft.cadence === "annual" ? draft.monthOfYear : null,
      start_date: draft.startDate.format(dateFormat),
      end_date: draft.endDate ? draft.endDate.format(dateFormat) : null,
      is_active: draft.isActive,
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

        <Divider orientation="left" plain style={{ margin: 0 }}>
          Dados da recorrência
        </Divider>

        <div className="filter-field">
          <label htmlFor="recurring-commitment-name">Nome</label>
          <Input
            id="recurring-commitment-name"
            value={draft.name}
            onChange={(event) => setDraft((current) => ({ ...current, name: event.target.value }))}
            placeholder="Ex.: Salário, Aluguel, Netflix"
          />
        </div>

        <div className="filter-field">
          <label htmlFor="recurring-commitment-kind">Tipo</label>
          <Select
            id="recurring-commitment-kind"
            value={draft.kind}
            options={kindOptions}
            onChange={(value: RecurringCommitmentKind) =>
              setDraft((current) => ({ ...current, kind: value }))
            }
          />
        </div>

        <div className="filter-field">
          <label htmlFor="recurring-commitment-amount">Valor</label>
          <InputNumber
            id="recurring-commitment-amount"
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
          <label htmlFor="recurring-commitment-category">Categoria</label>
          <Select
            id="recurring-commitment-category"
            value={draft.categoryId ?? undefined}
            options={categoryOptions}
            showSearch
            optionFilterProp="label"
            placeholder="Selecione uma categoria"
            onChange={(value: string) => setDraft((current) => ({ ...current, categoryId: value }))}
          />
        </div>

        <div className="filter-field">
          <label htmlFor="recurring-commitment-account">Conta (opcional)</label>
          <Input
            id="recurring-commitment-account"
            value={draft.accountId}
            onChange={(event) =>
              setDraft((current) => ({ ...current, accountId: event.target.value }))
            }
            placeholder="ID da conta"
          />
        </div>

        <div className="filter-field">
          <label htmlFor="recurring-commitment-cadence">Cadência</label>
          <Select
            id="recurring-commitment-cadence"
            value={draft.cadence}
            options={cadenceOptions}
            onChange={(value: RecurringCommitmentCadence) =>
              setDraft((current) => ({ ...current, cadence: value }))
            }
          />
        </div>

        <div className="filter-field">
          <label htmlFor="recurring-commitment-day">Dia do mês</label>
          <InputNumber
            id="recurring-commitment-day"
            style={{ width: "100%" }}
            min={1}
            max={31}
            value={draft.dayOfMonth}
            onChange={(value) => setDraft((current) => ({ ...current, dayOfMonth: value }))}
          />
          <Typography.Text type="secondary">
            Usado só para agendar a data esperada no calendário.
          </Typography.Text>
        </div>

        {draft.cadence === "annual" && (
          <div className="filter-field">
            <label htmlFor="recurring-commitment-month">Mês do ano</label>
            <Select
              id="recurring-commitment-month"
              value={draft.monthOfYear ?? undefined}
              options={monthOptions}
              placeholder="Selecione um mês"
              onChange={(value: number) =>
                setDraft((current) => ({ ...current, monthOfYear: value }))
              }
            />
          </div>
        )}

        <div className="filter-field">
          <label htmlFor="recurring-commitment-start-date">Data de início</label>
          <DatePicker
            id="recurring-commitment-start-date"
            style={{ width: "100%" }}
            format="DD/MM/YYYY"
            value={draft.startDate}
            allowClear={false}
            onChange={(value) => value && setDraft((current) => ({ ...current, startDate: value }))}
          />
        </div>

        <div className="filter-field">
          <label htmlFor="recurring-commitment-end-date">Data de término (opcional)</label>
          <DatePicker
            id="recurring-commitment-end-date"
            style={{ width: "100%" }}
            format="DD/MM/YYYY"
            value={draft.endDate}
            onChange={(value) => setDraft((current) => ({ ...current, endDate: value }))}
          />
        </div>

        <Flex align="center" gap="small">
          <Switch
            id="recurring-commitment-active"
            checked={draft.isActive}
            onChange={(checked) => setDraft((current) => ({ ...current, isActive: checked }))}
          />
          <label htmlFor="recurring-commitment-active">Ativo</label>
        </Flex>
      </Flex>
    </form>
  );
}

const drawerFormId = "recurring-commitment-form";

/** The fields inside a side drawer — the Recorrências page's own editor. */
export function RecurringCommitmentForm({
  open,
  submitting,
  onCancel,
  ...fields
}: Omit<FieldsProps, "formId"> & {
  open: boolean;
  submitting: boolean;
  onCancel: () => void;
}) {
  const isEditing = fields.commitment !== null;
  return (
    <Drawer
      title={isEditing ? "Editar compromisso" : "Novo compromisso"}
      open={open}
      onClose={onCancel}
      width={480}
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
      <RecurringCommitmentFields key={fields.commitment?.id ?? "new"} formId={drawerFormId} {...fields} />
    </Drawer>
  );
}
