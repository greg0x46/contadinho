import {
  Alert,
  Button,
  Checkbox,
  DatePicker,
  Divider,
  Drawer,
  Flex,
  Input,
  InputNumber,
  Segmented,
  Select,
  Switch,
  Typography,
} from "antd";
import dayjs, { type Dayjs } from "dayjs";
import { useEffect, useState } from "react";

import type {
  AutomationActionType,
  AutomationRule,
  AutomationRuleConditionOptions,
  Category,
  RecurringCommitment,
  RecurringCommitmentCadence,
  RecurringCommitmentKind,
  RecurringCommitmentWrite,
  RuleCondition,
  RuleLogicOperator,
} from "../../api/contracts";
import {
  recurringCommitmentCadenceLabel,
  recurringCommitmentKindLabel,
} from "../../presentation/recurringCommitmentLabels";
import {
  ruleConditionFieldLabel,
  ruleConditionOperatorLabel,
  ruleLogicOperatorLabel,
} from "../../presentation/ruleConditionLabels";
import { ConditionFieldConfig, ConditionListEditor } from "../shared/ConditionListEditor";

const dateFormat = "YYYY-MM-DD";

const actionOptions: { value: AutomationActionType; label: string }[] = [
  { value: "ignore", label: "Ignorar transação" },
  { value: "reconcile", label: "Conciliar recorrência" },
  { value: "set_category", label: "Aplicar categoria" },
];

export type AutomationEntry = {
  rule: AutomationRule;
  linkedCommitment: RecurringCommitment | null;
};

export type AutomationEntrySubmitPayload = {
  name: string;
  isActive: boolean;
  logicOperator: RuleLogicOperator;
  conditions: RuleCondition[];
  actionTypes: AutomationActionType[];
  applyRetroactively: boolean;
  categoryId: string | null;
  reconcile: {
    commitmentMode: "new" | "existing";
    existingCommitmentId: string | null;
    commitment: RecurringCommitmentWrite;
  } | null;
};

const ruleLogicOptions = (Object.keys(ruleLogicOperatorLabel) as RuleLogicOperator[]).map((value) => ({
  value,
  label: ruleLogicOperatorLabel[value],
}));
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
const commitmentModeOptions: { value: "new" | "existing"; label: string }[] = [
  { value: "new", label: "Nova recorrência" },
  { value: "existing", label: "Recorrência existente" },
];

type RuleDraft = {
  name: string;
  isActive: boolean;
  logicOperator: RuleLogicOperator;
  conditions: RuleCondition[];
};

const blankIgnoreConditions: RuleCondition[] = [{ field: "description", operator: "contains", value: "" }];
// Tolerance fields by default, not "description" — avoids a stray blank
// text condition colliding (same "Valor" accessible name) with the "Dados
// da recorrência" section's own Valor field once that section renders.
const blankReconcileConditions: RuleCondition[] = [
  { field: "amount", operator: "within_percent", value: "0:10" },
  { field: "day_of_month", operator: "day_range", value: "1:10" },
];

function blankConditionsFor(actionTypes: AutomationActionType[]): RuleCondition[] {
  return (actionTypes.includes("reconcile") ? blankReconcileConditions : blankIgnoreConditions).map(
    (condition) => ({ ...condition }),
  );
}

function ruleDraftFrom(rule: AutomationRule | null, actionTypes: AutomationActionType[]): RuleDraft {
  return rule
    ? {
        name: rule.name,
        isActive: rule.is_active,
        logicOperator: rule.logic_operator,
        conditions: rule.conditions.map((condition) => ({ ...condition })),
      }
    : { name: "", isActive: true, logicOperator: "or", conditions: blankConditionsFor(actionTypes) };
}

type CommitmentDraft = {
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

const blankCommitmentDraft: CommitmentDraft = {
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

function commitmentDraftFrom(commitment: RecurringCommitment | null): CommitmentDraft {
  if (!commitment) return blankCommitmentDraft;
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

export function AutomationEntryForm({
  open,
  entry,
  conditionOptions,
  conditionOptionsLoading,
  categories,
  commitments,
  commitmentsLoading,
  submitting,
  submitError,
  onSubmit,
  onCancel,
}: {
  open: boolean;
  entry: AutomationEntry | null;
  conditionOptions: AutomationRuleConditionOptions;
  conditionOptionsLoading: boolean;
  categories: Category[];
  commitments: RecurringCommitment[];
  commitmentsLoading: boolean;
  submitting: boolean;
  submitError: string | null;
  onSubmit: (payload: AutomationEntrySubmitPayload) => void;
  onCancel: () => void;
}) {
  const isEditing = entry !== null;
  const initialActionTypes: AutomationActionType[] = entry
    ? entry.rule.actions.map((action) => action.type)
    : ["ignore"];
  const initialCategoryId =
    entry?.rule.actions.find((action) => action.type === "set_category")?.category_id ?? null;

  const [actionTypes, setActionTypes] = useState<AutomationActionType[]>(initialActionTypes);
  const [categoryId, setCategoryId] = useState<string | null>(initialCategoryId);
  const [commitmentMode, setCommitmentMode] = useState<"new" | "existing">(
    entry?.linkedCommitment ? "existing" : "new",
  );
  const [selectedCommitmentId, setSelectedCommitmentId] = useState<string | null>(
    entry?.linkedCommitment?.id ?? null,
  );
  const [ruleDraft, setRuleDraft] = useState<RuleDraft>(() => ruleDraftFrom(entry?.rule ?? null, initialActionTypes));
  const [commitmentDraft, setCommitmentDraft] = useState<CommitmentDraft>(() =>
    commitmentDraftFrom(entry?.linkedCommitment ?? null),
  );
  const [applyRetroactively, setApplyRetroactively] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (open) {
      setActionTypes(initialActionTypes);
      setCategoryId(initialCategoryId);
      setCommitmentMode(entry?.linkedCommitment ? "existing" : "new");
      setSelectedCommitmentId(entry?.linkedCommitment?.id ?? null);
      setRuleDraft(ruleDraftFrom(entry?.rule ?? null, initialActionTypes));
      setCommitmentDraft(commitmentDraftFrom(entry?.linkedCommitment ?? null));
      setApplyRetroactively(false);
      setError(null);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, entry]);

  const selectExistingCommitment = (commitmentId: string | null) => {
    const selected = commitmentId ? commitments.find((item) => item.id === commitmentId) ?? null : null;
    setSelectedCommitmentId(selected?.id ?? null);
    setCommitmentDraft(commitmentDraftFrom(selected));
  };

  const setCommitmentModeAndReset = (mode: "new" | "existing") => {
    setCommitmentMode(mode);
    if (mode === "new") {
      setSelectedCommitmentId(null);
      setCommitmentDraft(blankCommitmentDraft);
    }
  };

  const setActionTypesAndReset = (nextActionTypes: AutomationActionType[]) => {
    setActionTypes(nextActionTypes);
    if (!isEditing) {
      setRuleDraft((current) => ({ ...current, conditions: blankConditionsFor(nextActionTypes) }));
    }
    if (!nextActionTypes.includes("reconcile")) {
      setCommitmentMode("new");
      setSelectedCommitmentId(null);
      setCommitmentDraft(blankCommitmentDraft);
    }
    if (!nextActionTypes.includes("set_category")) {
      setCategoryId(null);
    }
  };

  const baseFieldConfigs: ConditionFieldConfig[] = [
    {
      field: "description",
      label: ruleConditionFieldLabel.description,
      operators: [
        { value: "contains", label: ruleConditionOperatorLabel.contains },
        { value: "equals", label: ruleConditionOperatorLabel.equals },
      ],
    },
    {
      field: "card",
      label: ruleConditionFieldLabel.card,
      operators: [
        { value: "contains", label: ruleConditionOperatorLabel.contains },
        { value: "equals", label: ruleConditionOperatorLabel.equals },
      ],
      valueOptions: () => conditionOptions.cards.map((value) => ({ value, label: value })),
      valueOptionsLoading: conditionOptionsLoading,
      valuePlaceholder: () => "Selecione um cartão",
      valueNotFoundContent: () => (conditionOptionsLoading ? "Carregando…" : "Nenhum cartão encontrado"),
    },
    {
      field: "account",
      label: ruleConditionFieldLabel.account,
      operators: [
        { value: "contains", label: ruleConditionOperatorLabel.contains },
        { value: "equals", label: ruleConditionOperatorLabel.equals },
      ],
      valueOptions: () => conditionOptions.accounts.map((value) => ({ value, label: value })),
      valueOptionsLoading: conditionOptionsLoading,
      valuePlaceholder: () => "Selecione uma conta",
      valueNotFoundContent: () => (conditionOptionsLoading ? "Carregando…" : "Nenhuma conta encontrada"),
    },
  ];

  const reconcileFieldConfigs: ConditionFieldConfig[] = [
    {
      field: "amount",
      label: ruleConditionFieldLabel.amount,
      operators: [{ value: "within_percent", label: ruleConditionOperatorLabel.within_percent }],
      pairOperator: "within_percent",
      pairFirstLabel: "Valor esperado",
      pairSecondLabel: "Tolerância",
      pairSecondSuffix: "%",
      pairMin: 0,
    },
    {
      field: "day_of_month",
      label: ruleConditionFieldLabel.day_of_month,
      operators: [{ value: "day_range", label: ruleConditionOperatorLabel.day_range }],
      pairOperator: "day_range",
      pairFirstLabel: "Do dia",
      pairSecondLabel: "Até o dia",
      pairMin: 1,
      pairMax: 31,
    },
  ];

  const conditionFieldConfigs: ConditionFieldConfig[] = [...reconcileFieldConfigs, ...baseFieldConfigs];

  const categoryOptions = categories
    .filter((category) => category.is_active && category.kind !== "transfer")
    .map((category) => ({ value: category.id, label: category.name }));

  const submit = () => {
    if (ruleDraft.name.trim() === "") {
      setError("Informe um nome para a regra.");
      return;
    }
    if (ruleDraft.conditions.length === 0) {
      setError("Adicione ao menos uma condição.");
      return;
    }
    if (ruleDraft.conditions.some((condition) => condition.value.trim() === "")) {
      setError("Preencha o valor de todas as condições.");
      return;
    }
    if (actionTypes.length === 0) {
      setError("Selecione ao menos uma ação.");
      return;
    }
    if (actionTypes.includes("set_category") && categoryId === null) {
      setError("Selecione uma categoria para aplicar.");
      return;
    }

    let reconcile: AutomationEntrySubmitPayload["reconcile"] = null;
    if (actionTypes.includes("reconcile")) {
      if (commitmentDraft.name.trim() === "") {
        setError("Informe um nome para o compromisso.");
        return;
      }
      if (commitmentDraft.amount === null || commitmentDraft.amount <= 0) {
        setError("Informe um valor maior que zero.");
        return;
      }
      if (commitmentDraft.categoryId === null) {
        setError("Selecione uma categoria.");
        return;
      }
      if (commitmentDraft.dayOfMonth === null) {
        setError("Informe o dia do mês.");
        return;
      }
      if (commitmentDraft.cadence === "annual" && commitmentDraft.monthOfYear === null) {
        setError("Informe o mês do ano para uma recorrência anual.");
        return;
      }
      if (commitmentDraft.endDate && commitmentDraft.endDate.isBefore(commitmentDraft.startDate, "day")) {
        setError("A data de término não pode ser anterior à data de início.");
        return;
      }
      reconcile = {
        commitmentMode,
        existingCommitmentId: commitmentMode === "existing" ? selectedCommitmentId : null,
        commitment: {
          name: commitmentDraft.name.trim(),
          kind: commitmentDraft.kind,
          amount: commitmentDraft.amount.toFixed(2),
          category_id: commitmentDraft.categoryId,
          account_id: commitmentDraft.accountId.trim() === "" ? null : commitmentDraft.accountId.trim(),
          cadence: commitmentDraft.cadence,
          day_of_month: commitmentDraft.dayOfMonth,
          month_of_year: commitmentDraft.cadence === "annual" ? commitmentDraft.monthOfYear : null,
          start_date: commitmentDraft.startDate.format(dateFormat),
          end_date: commitmentDraft.endDate ? commitmentDraft.endDate.format(dateFormat) : null,
          is_active: commitmentDraft.isActive,
        },
      };
    }

    setError(null);
    onSubmit({
      name: ruleDraft.name.trim(),
      isActive: ruleDraft.isActive,
      logicOperator: ruleDraft.logicOperator,
      conditions: ruleDraft.conditions.map((condition) => ({ ...condition, value: condition.value.trim() })),
      actionTypes,
      applyRetroactively,
      categoryId: actionTypes.includes("set_category") ? categoryId : null,
      reconcile,
    });
  };

  return (
    <Drawer
      title={isEditing ? "Editar automação" : "Nova automação"}
      open={open}
      onClose={onCancel}
      width={480}
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
      <Flex vertical gap="middle" className="automation-entry-form">
        {(error ?? submitError) && <Alert type="error" showIcon message={error ?? submitError} />}

        <div className="filter-field">
          <label htmlFor="automation-rule-name">Nome da automação</label>
          <Input
            id="automation-rule-name"
            value={ruleDraft.name}
            onChange={(event) => setRuleDraft((current) => ({ ...current, name: event.target.value }))}
            placeholder="Ex.: Ignorar assinaturas, Conciliar aluguel"
          />
        </div>

        <Flex align="center" gap="small">
          <Switch
            id="automation-rule-active"
            checked={ruleDraft.isActive}
            onChange={(checked) => setRuleDraft((current) => ({ ...current, isActive: checked }))}
          />
          <label htmlFor="automation-rule-active">Ativa</label>
        </Flex>

        <div className="filter-field">
          <label htmlFor="automation-rule-logic">Combinar condições com</label>
          <Select
            id="automation-rule-logic"
            value={ruleDraft.logicOperator}
            options={ruleLogicOptions}
            onChange={(value: RuleLogicOperator) =>
              setRuleDraft((current) => ({ ...current, logicOperator: value }))
            }
          />
        </div>

        <Flex vertical gap="small">
          <span>Condições</span>
          <ConditionListEditor
            conditions={ruleDraft.conditions}
            fieldConfigs={conditionFieldConfigs}
            onChange={(conditions) => setRuleDraft((current) => ({ ...current, conditions }))}
          />
        </Flex>

        <div className="filter-field">
          <span>Ações</span>
          <Checkbox.Group
            aria-label="Ações"
            value={actionTypes}
            options={actionOptions}
            disabled={isEditing}
            onChange={(value) => setActionTypesAndReset(value as AutomationActionType[])}
          />
        </div>

        {actionTypes.includes("set_category") && (
          <div className="filter-field">
            <label htmlFor="automation-entry-category">Categoria a aplicar</label>
            <Select
              id="automation-entry-category"
              value={categoryId ?? undefined}
              options={categoryOptions}
              showSearch
              optionFilterProp="label"
              placeholder="Selecione uma categoria"
              onChange={(value: string) => setCategoryId(value)}
            />
          </div>
        )}

        {(actionTypes.includes("ignore") || actionTypes.includes("set_category")) && (
          <Checkbox checked={applyRetroactively} onChange={(event) => setApplyRetroactively(event.target.checked)}>
            Aplicar agora às transações existentes que casarem com as condições
          </Checkbox>
        )}

        {actionTypes.includes("reconcile") && (
          <>
            {!isEditing && (
              <div className="filter-field">
                <label htmlFor="recurring-commitment-mode">Recorrência</label>
                <Segmented
                  id="recurring-commitment-mode"
                  value={commitmentMode}
                  options={commitmentModeOptions}
                  onChange={(value) => setCommitmentModeAndReset(value as "new" | "existing")}
                />
              </div>
            )}

            {commitmentMode === "existing" && (
              <div className="filter-field">
                <label htmlFor="recurring-commitment-existing">Selecione uma recorrência</label>
                <Select
                  id="recurring-commitment-existing"
                  showSearch
                  optionFilterProp="label"
                  loading={commitmentsLoading}
                  placeholder="Selecione uma recorrência existente"
                  value={selectedCommitmentId ?? undefined}
                  options={commitments.map((commitment) => ({ value: commitment.id, label: commitment.name }))}
                  onChange={(value: string) => selectExistingCommitment(value)}
                />
              </div>
            )}

            <Divider orientation="left" plain style={{ margin: 0 }}>
              Dados da recorrência
            </Divider>

            <div className="filter-field">
              <label htmlFor="recurring-commitment-name">Nome</label>
              <Input
                id="recurring-commitment-name"
                value={commitmentDraft.name}
                onChange={(event) =>
                  setCommitmentDraft((current) => ({ ...current, name: event.target.value }))
                }
                placeholder="Ex.: Salário, Aluguel, Netflix"
              />
            </div>

            <div className="filter-field">
              <label htmlFor="recurring-commitment-kind">Tipo</label>
              <Select
                id="recurring-commitment-kind"
                value={commitmentDraft.kind}
                options={kindOptions}
                onChange={(value: RecurringCommitmentKind) =>
                  setCommitmentDraft((current) => ({ ...current, kind: value }))
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
                value={commitmentDraft.amount}
                onChange={(value) => setCommitmentDraft((current) => ({ ...current, amount: value }))}
                placeholder="0,00"
              />
            </div>

            <div className="filter-field">
              <label htmlFor="recurring-commitment-category">Categoria</label>
              <Select
                id="recurring-commitment-category"
                value={commitmentDraft.categoryId ?? undefined}
                options={categoryOptions}
                showSearch
                optionFilterProp="label"
                placeholder="Selecione uma categoria"
                onChange={(value: string) => setCommitmentDraft((current) => ({ ...current, categoryId: value }))}
              />
            </div>

            <div className="filter-field">
              <label htmlFor="recurring-commitment-account">Conta (opcional)</label>
              <Input
                id="recurring-commitment-account"
                value={commitmentDraft.accountId}
                onChange={(event) =>
                  setCommitmentDraft((current) => ({ ...current, accountId: event.target.value }))
                }
                placeholder="ID da conta"
              />
            </div>

            <div className="filter-field">
              <label htmlFor="recurring-commitment-cadence">Cadência</label>
              <Select
                id="recurring-commitment-cadence"
                value={commitmentDraft.cadence}
                options={cadenceOptions}
                onChange={(value: RecurringCommitmentCadence) =>
                  setCommitmentDraft((current) => ({ ...current, cadence: value }))
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
                value={commitmentDraft.dayOfMonth}
                onChange={(value) => setCommitmentDraft((current) => ({ ...current, dayOfMonth: value }))}
              />
              <Typography.Text type="secondary">
                Usado só para agendar a data esperada no calendário — não obriga a conciliação a exigir esse dia.
              </Typography.Text>
            </div>

            {commitmentDraft.cadence === "annual" && (
              <div className="filter-field">
                <label htmlFor="recurring-commitment-month">Mês do ano</label>
                <Select
                  id="recurring-commitment-month"
                  value={commitmentDraft.monthOfYear ?? undefined}
                  options={monthOptions}
                  placeholder="Selecione um mês"
                  onChange={(value: number) =>
                    setCommitmentDraft((current) => ({ ...current, monthOfYear: value }))
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
                value={commitmentDraft.startDate}
                allowClear={false}
                onChange={(value) =>
                  value && setCommitmentDraft((current) => ({ ...current, startDate: value }))
                }
              />
            </div>

            <div className="filter-field">
              <label htmlFor="recurring-commitment-end-date">Data de término (opcional)</label>
              <DatePicker
                id="recurring-commitment-end-date"
                style={{ width: "100%" }}
                format="DD/MM/YYYY"
                value={commitmentDraft.endDate}
                onChange={(value) => setCommitmentDraft((current) => ({ ...current, endDate: value }))}
              />
            </div>

            <Flex align="center" gap="small">
              <Switch
                id="recurring-commitment-active"
                checked={commitmentDraft.isActive}
                onChange={(checked) => setCommitmentDraft((current) => ({ ...current, isActive: checked }))}
              />
              <label htmlFor="recurring-commitment-active">Ativo</label>
            </Flex>
          </>
        )}
      </Flex>
    </Drawer>
  );
}
