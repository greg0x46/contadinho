export const syncStatuses = [
  "in_progress",
  "completed",
  "completed_with_failures",
  "failed",
] as const;

export type SyncStatus = (typeof syncStatuses)[number];

export const failureStages = [
  "auth",
  "item",
  "accounts",
  "account",
  "transactions",
  "normalize",
  "interrupted",
  "worker_unavailable",
] as const;

export type FailureStage = (typeof failureStages)[number];

export interface SyncRun {
  id: string;
  status: SyncStatus;
  /** Which connection this run refreshed — with several banks, a run means nothing without it. */
  source_id: string;
  source_name: string;
  started_at: string;
  finished_at: string | null;
  accounts_processed: number;
  transactions_inserted: number;
  transactions_updated: number;
  result_message: string | null;
}

export interface SyncFailure {
  stage: FailureStage;
  code: string;
  message: string;
  external_account_id: string | null;
  external_transaction_id: string | null;
  occurred_at: string;
}

export interface SyncRunDetail extends SyncRun {
  failures: SyncFailure[];
}

/**
 * requested counts every connection the create request targeted, which can
 * exceed runs.length: a connection already syncing, or one whose insert
 * failed outright, is skipped rather than failing the whole request, so a
 * shorter runs list is otherwise indistinguishable from "that's everything".
 */
export interface CreateSyncRunResult {
  runs: SyncRun[];
  requested: number;
}

/** One provider connection (a Pluggy item): the unit that owns accounts and syncs. */
export interface DataSource {
  id: string;
  provider: string;
  external_item_id: string;
  /** The institution reported by the last sync; overwritten on every run. */
  display_name: string | null;
  /** The user's own name for the connection, which wins over display_name. */
  label: string | null;
  /** What to show: label, else display_name, else the raw item id. */
  name: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface Problem {
  type: string;
  title: string;
  status: number;
  detail?: string;
  active_sync_run_id?: string | null;
}

export const transactionClassifications = ["inflow", "outflow", "unclassified"] as const;
export type TransactionClassification = (typeof transactionClassifications)[number];
export const transactionGroupings = ["none", "day", "week", "month", "year"] as const;
export type TransactionGrouping = (typeof transactionGroupings)[number];
export const transactionInclusionStates = ["considered", "ignored"] as const;
export type TransactionInclusionState = (typeof transactionInclusionStates)[number];
export const transactionInclusionOrigins = ["manual", "rule"] as const;
export type TransactionInclusionOrigin = (typeof transactionInclusionOrigins)[number];
export const transactionOrigins = ["synced", "manual"] as const;
export type TransactionOrigin = (typeof transactionOrigins)[number];

export const categoryKinds = ["expense", "income", "transfer"] as const;
export type CategoryKind = (typeof categoryKinds)[number];
// "rule" mirrors categories.OriginRule (internal/categories/decisions.go):
// set when a matching automation rule's set_category action assigns the
// category, distinct from "automatic" (the source_category mapping) and
// "manual" (the user). "learned" mirrors categories.OriginLearned: the
// category was copied from a past manual decision on a similar transaction.
export const categoryOrigins = ["manual", "automatic", "rule", "learned"] as const;
export type CategoryOrigin = (typeof categoryOrigins)[number];

export interface TransactionFilters {
  origin?: "manual" | "synced" | null;
  card_balance?: boolean | null;
  credit_card?: boolean | null;
  date_from: string | null;
  date_to: string | null;
  description: string | null;
  account_id: string | null;
  institution: string | null;
  category_id: string | null;
  classification: TransactionClassification | null;
  provider_status: "POSTED" | "PENDING" | null;
  amount_min: string | null;
  amount_max: string | null;
  uncategorized: boolean | null;
}

export interface TransactionQuery {
  timezone: string;
  group_by: TransactionGrouping;
  page: number;
  page_size: number;
  filters: TransactionFilters;
}

export interface CurrencyTotals {
  currency_code: string;
  inflow: string;
  outflow: string;
  balance: string;
}

export interface InternalCategory {
  id: string;
  name: string;
  kind: CategoryKind;
  is_active: boolean;
  icon: string;
  color: string;
  origin: CategoryOrigin;
  changed_at: string;
}

export interface TransactionItem {
  id: string;
  external_id: string;
  origin: TransactionOrigin;
  occurred_at: string | null;
  description: string | null;
  account: {
    id: string;
    name: string | null;
    institution: string | null;
    currency_code: string | null;
  };
  source_category: string | null;
  internal_category: InternalCategory | null;
  movement_type: string | null;
  provider_status: string | null;
  classification: TransactionClassification;
  amount: string | null;
  currency_code: string | null;
  amount_in_account_currency: string | null;
  effective_money: {
    value: string;
    currency_code: string;
    source: "account_currency" | "transaction_currency";
  } | null;
  /**
   * Total already allocated from this bank transaction to investment
   * deposits/withdrawals. It is zero when no allocation exists. The bank
   * amount itself remains in effective_money.
   */
  investment_transfer_amount: string;
  /** Amount that can be reported after currency normalization, if known. */
  reportable_amount: string | null;
  card: {
    number: string;
    installment_number: number | null;
    total_installments: number | null;
  } | null;
  inclusion: {
    state: TransactionInclusionState;
    changed_at: string | null;
    origin: TransactionInclusionOrigin;
    rule_name: string | null;
  };
  totals_eligibility: {
    included: boolean;
    reason:
      | "ignored"
      | "transfer_category"
      | "unclassified"
      | "ineligible_status"
      | "missing_money_pair"
      | "zero_value"
      | "investment_transfer"
      | null;
  };
  group_key: string;
}

export interface TransactionInclusionResult {
  transaction_id: string;
  state: TransactionInclusionState;
  changed_at: string | null;
}

export interface TransactionCategoryResult {
  transaction_id: string;
  category_id: string;
  origin: "manual";
  changed_at: string;
}

// ManualTransactionWrite is the request body for both creating and editing a
// lançamento manual. amount is already signed — positive for money coming
// in, negative for money going out — matching how the backend's
// transactions.ManualInput expects it.
export interface ManualTransactionWrite {
  account_id: string;
  description: string;
  amount: string;
  occurred_at: string;
  category_id: string | null;
}

export interface TransactionGroup {
  key: string;
  kind: TransactionGrouping | "undated";
  start_date: string | null;
  end_date: string | null;
  item_count: number;
  page_item_count: number;
  has_items_before: boolean;
  has_items_after: boolean;
  totals: CurrencyTotals[];
}

export interface CategoryOption {
  id: string;
  name: string;
  kind: CategoryKind;
  is_active: boolean;
  icon: string;
  color: string;
}

export interface TransactionQueryResult {
  confirmed_at: string;
  stored_total: number;
  page: { number: number; size: number; total_items: number; total_pages: number };
  items: TransactionItem[];
  totals: CurrencyTotals[];
  groups: TransactionGroup[];
  available_filters: {
    accounts: { id: string; name: string | null; institution: string | null }[];
    institutions: string[];
    categories: CategoryOption[];
  };
}

export const categorySpendingSources = ["real", "projetado"] as const;
export type CategorySpendingSource = (typeof categorySpendingSources)[number];

export interface CategorySpendingItem {
  category_id: string | null;
  category_name: string;
  category_icon: string;
  category_color: string;
  amount: string;
  source: CategorySpendingSource;
}

export interface SpendingByCategory {
  month: string;
  currency_code: string;
  total: string;
  items: CategorySpendingItem[];
}

const uuidPattern =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === "object" && value !== null && !Array.isArray(value);

const isValidDate = (value: unknown): value is string =>
  typeof value === "string" && value.trim() !== "" && !Number.isNaN(Date.parse(value));

const isNullableString = (value: unknown): value is string | null =>
  value === null || typeof value === "string";

const isCount = (value: unknown): value is number =>
  Number.isInteger(value) && typeof value === "number" && value >= 0;

export const isUuid = (value: string): boolean => uuidPattern.test(value);

const decimalPattern = /^-?(0|[1-9][0-9]*)(\.[0-9]+)?$/;
const dateOnlyPattern = /^\d{4}-\d{2}-\d{2}$/;

function hasOnlyKeys(value: Record<string, unknown>, keys: readonly string[]): boolean {
  const actual = Object.keys(value);
  return actual.length === keys.length && actual.every((key) => keys.includes(key));
}

function requiredRecord(
  value: unknown,
  keys: readonly string[],
  message = "Resposta de transações inválida.",
): Record<string, unknown> {
  if (!isRecord(value) || !hasOnlyKeys(value, keys)) throw new TypeError(message);
  return value;
}

/**
 * A few response fields were added after the first released frontend. Keep
 * their absence readable for cached responses and test fixtures, while still
 * rejecting misspelled server fields instead of quietly rendering nonsense.
 */
function requiredRecordWithOptionalKeys(
  value: unknown,
  requiredKeys: readonly string[],
  optionalKeys: readonly string[],
  message = "Resposta de transações inválida.",
): Record<string, unknown> {
  if (!isRecord(value)) throw new TypeError(message);
  const allowed = [...requiredKeys, ...optionalKeys];
  const actual = Object.keys(value);
  if (!requiredKeys.every((key) => key in value) || !actual.every((key) => allowed.includes(key))) {
    throw new TypeError(message);
  }
  return value;
}

function decimal(value: unknown): string {
  if (typeof value !== "string" || !decimalPattern.test(value)) {
    throw new TypeError("Valor monetário inválido.");
  }
  return value;
}

function nullableDecimal(value: unknown): string | null {
  return value === null ? null : decimal(value);
}

function nullableText(value: unknown): string | null {
  if (!isNullableString(value)) throw new TypeError("Texto opcional inválido.");
  return value;
}

function nullableDate(value: unknown): string | null {
  if (!(value === null || isValidDate(value))) throw new TypeError("Data opcional inválida.");
  return value;
}

function positiveCount(value: unknown): number {
  if (!isCount(value) || value < 1) throw new TypeError("Contagem inválida.");
  return value;
}

function parseCurrencyTotals(value: unknown): CurrencyTotals {
  const item = requiredRecord(value, ["currency_code", "inflow", "outflow", "balance"]);
  if (typeof item.currency_code !== "string" || item.currency_code === "") {
    throw new TypeError("Moeda inválida.");
  }
  return {
    currency_code: item.currency_code,
    inflow: decimal(item.inflow),
    outflow: decimal(item.outflow),
    balance: decimal(item.balance),
  };
}

const monthOnlyPattern = /^\d{4}-\d{2}$/;

function parseCategorySpendingItem(value: unknown): CategorySpendingItem {
  const item = requiredRecord(
    value,
    ["category_id", "category_name", "category_icon", "category_color", "amount", "source"],
    "Gasto por categoria inválido.",
  );
  if (item.category_id !== null && typeof item.category_id !== "string") {
    throw new TypeError("Gasto por categoria inválido.");
  }
  if (typeof item.category_name !== "string" || item.category_name === "") {
    throw new TypeError("Gasto por categoria inválido.");
  }
  if (typeof item.category_icon !== "string" || typeof item.category_color !== "string") {
    throw new TypeError("Gasto por categoria inválido.");
  }
  if (!categorySpendingSources.includes(item.source as CategorySpendingSource)) {
    throw new TypeError("Gasto por categoria inválido.");
  }
  return {
    category_id: item.category_id,
    category_name: item.category_name,
    category_icon: item.category_icon,
    category_color: item.category_color,
    amount: decimal(item.amount),
    source: item.source as CategorySpendingSource,
  };
}

export function parseSpendingByCategory(value: unknown): SpendingByCategory {
  const item = requiredRecord(
    value,
    ["month", "currency_code", "total", "items"],
    "Gastos por categoria inválidos.",
  );
  if (typeof item.month !== "string" || !monthOnlyPattern.test(item.month)) {
    throw new TypeError("Gastos por categoria inválidos.");
  }
  if (typeof item.currency_code !== "string" || item.currency_code === "") {
    throw new TypeError("Gastos por categoria inválidos.");
  }
  if (!Array.isArray(item.items)) {
    throw new TypeError("Gastos por categoria inválidos.");
  }
  return {
    month: item.month,
    currency_code: item.currency_code,
    total: decimal(item.total),
    items: item.items.map(parseCategorySpendingItem),
  };
}

function parseInternalCategory(value: unknown): InternalCategory {
  const category = requiredRecord(
    value,
    ["id", "name", "kind", "is_active", "icon", "color", "origin", "changed_at"],
    "Categoria interna inválida.",
  );
  if (
    typeof category.id !== "string" ||
    !isUuid(category.id) ||
    typeof category.name !== "string" ||
    category.name === "" ||
    !categoryKinds.includes(category.kind as CategoryKind) ||
    typeof category.is_active !== "boolean" ||
    typeof category.icon !== "string" ||
    typeof category.color !== "string" ||
    !categoryOrigins.includes(category.origin as CategoryOrigin) ||
    !isValidDate(category.changed_at)
  ) {
    throw new TypeError("Categoria interna inválida.");
  }
  return {
    id: category.id,
    name: category.name,
    kind: category.kind as CategoryKind,
    is_active: category.is_active,
    icon: category.icon,
    color: category.color,
    origin: category.origin as CategoryOrigin,
    changed_at: category.changed_at as string,
  };
}

function parseCardInfo(value: unknown): NonNullable<TransactionItem["card"]> {
  const card = requiredRecord(
    value,
    ["number", "installment_number", "total_installments"],
    "Cartão inválido.",
  );
  if (
    typeof card.number !== "string" ||
    card.number === "" ||
    !(card.installment_number === null || isCount(card.installment_number)) ||
    !(card.total_installments === null || isCount(card.total_installments))
  ) {
    throw new TypeError("Cartão inválido.");
  }
  return {
    number: card.number,
    installment_number: card.installment_number as number | null,
    total_installments: card.total_installments as number | null,
  };
}

export function parseTransactionItem(value: unknown): TransactionItem {
  const item = requiredRecordWithOptionalKeys(value, [
    "id",
    "external_id",
    "origin",
    "occurred_at",
    "description",
    "account",
    "source_category",
    "internal_category",
    "movement_type",
    "provider_status",
    "classification",
    "amount",
    "currency_code",
    "amount_in_account_currency",
    "effective_money",
    "card",
    "inclusion",
    "totals_eligibility",
    "group_key",
  ], ["investment_transfer_amount", "reportable_amount"]);
  const account = requiredRecord(item.account, ["id", "name", "institution", "currency_code"]);
  const eligibility = requiredRecord(item.totals_eligibility, ["included", "reason"]);
  const inclusion = requiredRecord(item.inclusion, [
    "state",
    "changed_at",
    "origin",
    "rule_name",
  ]);
  const reasons = [
    "ignored",
    "transfer_category",
    "unclassified",
    "ineligible_status",
    "missing_money_pair",
    "zero_value",
    "investment_transfer",
  ];
  if (
    typeof item.id !== "string" ||
    !isUuid(item.id) ||
    typeof item.external_id !== "string" ||
    !transactionOrigins.includes(item.origin as TransactionOrigin) ||
    !(item.occurred_at === null || isValidDate(item.occurred_at)) ||
    !transactionClassifications.includes(item.classification as TransactionClassification) ||
    typeof item.group_key !== "string" ||
    typeof account.id !== "string" ||
    !isUuid(account.id) ||
    typeof eligibility.included !== "boolean" ||
    !(eligibility.reason === null || reasons.includes(String(eligibility.reason))) ||
    !transactionInclusionStates.includes(inclusion.state as TransactionInclusionState) ||
    !(inclusion.changed_at === null || isValidDate(inclusion.changed_at)) ||
    !transactionInclusionOrigins.includes(inclusion.origin as TransactionInclusionOrigin) ||
    !isNullableString(inclusion.rule_name) ||
    !isNullableString(item.source_category)
  ) {
    throw new TypeError("Item de transação inválido.");
  }
  if (
    eligibility.included !== (eligibility.reason === null) ||
    (inclusion.state === "ignored" &&
      (eligibility.included || eligibility.reason !== "ignored")) ||
    (inclusion.state === "considered" && eligibility.reason === "ignored")
  ) {
    throw new TypeError("Elegibilidade de transação inválida.");
  }
  let effectiveMoney: TransactionItem["effective_money"] = null;
  if (item.effective_money !== null) {
    const money = requiredRecord(item.effective_money, ["value", "currency_code", "source"]);
    if (
      typeof money.currency_code !== "string" ||
      money.currency_code === "" ||
      !["account_currency", "transaction_currency"].includes(String(money.source))
    ) {
      throw new TypeError("Par monetário inválido.");
    }
    effectiveMoney = {
      value: decimal(money.value),
      currency_code: money.currency_code,
      source: money.source as "account_currency" | "transaction_currency",
    };
  }
  return {
    id: item.id,
    external_id: item.external_id,
    origin: item.origin as TransactionOrigin,
    occurred_at: item.occurred_at as string | null,
    description: nullableText(item.description),
    account: {
      id: account.id,
      name: nullableText(account.name),
      institution: nullableText(account.institution),
      currency_code: nullableText(account.currency_code),
    },
    source_category: item.source_category as string | null,
    internal_category: item.internal_category === null ? null : parseInternalCategory(item.internal_category),
    movement_type: nullableText(item.movement_type),
    provider_status: nullableText(item.provider_status),
    classification: item.classification as TransactionClassification,
    amount: nullableDecimal(item.amount),
    currency_code: nullableText(item.currency_code),
    amount_in_account_currency: nullableDecimal(item.amount_in_account_currency),
    effective_money: effectiveMoney,
    // Older persisted fixtures and servers predate investment allocation.
    // Treat their absence as an empty allocation while real API responses
    // always carry both fields.
    investment_transfer_amount:
      item.investment_transfer_amount === undefined ? "0" : decimal(item.investment_transfer_amount),
    reportable_amount:
      item.reportable_amount === undefined ? effectiveMoney?.value ?? null : nullableDecimal(item.reportable_amount),
    card: item.card === null ? null : parseCardInfo(item.card),
    inclusion: {
      state: inclusion.state as TransactionInclusionState,
      changed_at: inclusion.changed_at as string | null,
      origin: inclusion.origin as TransactionInclusionOrigin,
      rule_name: inclusion.rule_name as string | null,
    },
    totals_eligibility: {
      included: eligibility.included,
      reason: eligibility.reason as TransactionItem["totals_eligibility"]["reason"],
    },
    group_key: item.group_key,
  };
}

export function parseTransactionCategoryResult(value: unknown): TransactionCategoryResult {
  const result = requiredRecord(
    value,
    ["transaction_id", "category_id", "origin", "changed_at"],
    "Confirmação de categoria inválida.",
  );
  if (
    typeof result.transaction_id !== "string" ||
    !isUuid(result.transaction_id) ||
    typeof result.category_id !== "string" ||
    !isUuid(result.category_id) ||
    result.origin !== "manual" ||
    !isValidDate(result.changed_at)
  ) {
    throw new TypeError("Confirmação de categoria inválida.");
  }
  return {
    transaction_id: result.transaction_id,
    category_id: result.category_id,
    origin: "manual",
    changed_at: result.changed_at as string,
  };
}

export function parseTransactionInclusionResult(value: unknown): TransactionInclusionResult {
  const result = requiredRecord(
    value,
    ["transaction_id", "state", "changed_at"],
    "Confirmação de inclusão inválida.",
  );
  if (
    typeof result.transaction_id !== "string" ||
    !isUuid(result.transaction_id) ||
    !transactionInclusionStates.includes(result.state as TransactionInclusionState) ||
    !(result.changed_at === null || isValidDate(result.changed_at))
  ) {
    throw new TypeError("Confirmação de inclusão inválida.");
  }
  return {
    transaction_id: result.transaction_id,
    state: result.state as TransactionInclusionState,
    changed_at: result.changed_at as string | null,
  };
}

// RuleCondition is the shared field/operator/value shape both automation
// rules and recurring commitments compose — see internal/rules (backend)
// and .specs/relatorio-financeiro/m0-motor-de-regras.md. "amount"/
// "day_of_month" only make sense where there's an occurrence to compare
// against — on an AutomationRule that means a "reconcile" action is present
// (see ruleConditionFieldOperators and AutomationAction below); the backend
// rejects them otherwise.
export const ruleConditionFields = ["description", "card", "account", "amount", "day_of_month"] as const;
export type RuleConditionField = (typeof ruleConditionFields)[number];
export const ruleConditionOperators = ["contains", "equals", "within_percent", "day_range"] as const;
export type RuleConditionOperator = (typeof ruleConditionOperators)[number];
export const ruleLogicOperators = ["and", "or"] as const;
export type RuleLogicOperator = (typeof ruleLogicOperators)[number];

export interface RuleCondition {
  field: RuleConditionField;
  operator: RuleConditionOperator;
  value: string;
}

// ruleConditionFieldOperators pins which operator(s) a field accepts —
// mirrors internal/automation/model.go's validateCondition. amount/
// day_of_month are only ever valid on a rule with a reconcile action (see
// parseRuleCondition's allowReconcileFields parameter).
const ruleConditionFieldOperators: Record<RuleConditionField, readonly RuleConditionOperator[]> = {
  description: ["contains", "equals"],
  card: ["contains", "equals"],
  account: ["contains", "equals"],
  amount: ["within_percent"],
  day_of_month: ["day_range"],
};

function parseRuleCondition(value: unknown, allowReconcileFields: boolean): RuleCondition {
  const condition = requiredRecord(value, ["field", "operator", "value"], "Condição inválida.");
  const field = condition.field as RuleConditionField;
  const operator = condition.operator as RuleConditionOperator;
  const isReconcileField = field === "amount" || field === "day_of_month";
  if (
    !ruleConditionFields.includes(field) ||
    !ruleConditionOperators.includes(operator) ||
    !ruleConditionFieldOperators[field].includes(operator) ||
    (isReconcileField && !allowReconcileFields) ||
    typeof condition.value !== "string" ||
    condition.value === ""
  ) {
    throw new TypeError("Condição inválida.");
  }
  return { field, operator, value: condition.value };
}

export type AutomationLogicOperator = RuleLogicOperator;
export const automationLogicOperators = ruleLogicOperators;

export const automationActionTypes = ["ignore", "reconcile", "set_category"] as const;
export type AutomationActionType = (typeof automationActionTypes)[number];

export interface AutomationAction {
  type: AutomationActionType;
  /** The recurring Scenario a reconcile action targets; null otherwise. */
  scenario_id: string | null;
  category_id?: string | null;
}

export interface AutomationRule {
  id: string;
  name: string;
  is_active: boolean;
  logic_operator: AutomationLogicOperator;
  conditions: RuleCondition[];
  actions: AutomationAction[];
  created_at: string;
  updated_at: string;
}

export interface AutomationRuleWrite {
  name: string;
  is_active: boolean;
  logic_operator: AutomationLogicOperator;
  conditions: RuleCondition[];
  actions: AutomationAction[];
  apply_retroactively: boolean;
}

export interface RetroactiveApplyResult {
  matched: number;
  ignored: number;
  categorized: number;
}

export interface AutomationRuleWriteResult {
  rule: AutomationRule;
  retroactive_apply: RetroactiveApplyResult | null;
}

export interface AutomationRuleConditionOptions {
  accounts: string[];
  cards: string[];
}

export function parseAutomationRuleConditionOptions(
  value: unknown,
): AutomationRuleConditionOptions {
  const options = requiredRecord(
    value,
    ["accounts", "cards"],
    "Opções de condições inválidas.",
  );
  if (
    !Array.isArray(options.accounts) ||
    !options.accounts.every((item) => typeof item === "string" && item !== "") ||
    !Array.isArray(options.cards) ||
    !options.cards.every((item) => typeof item === "string" && item !== "")
  ) {
    throw new TypeError("Opções de condições inválidas.");
  }
  return {
    accounts: options.accounts as string[],
    cards: options.cards as string[],
  };
}

function parseAutomationAction(value: unknown): AutomationAction {
  const record = isRecord(value) ? value : null;
  const hasCategoryID = record !== null && "category_id" in record;
  const action = requiredRecord(
    value,
    ["type", "scenario_id", ...(hasCategoryID ? ["category_id"] : [])],
    "Ação inválida.",
  );
  const type = action.type as AutomationActionType;
  const scenarioID = action.scenario_id;
  const categoryID = action.category_id;
  if (
    !automationActionTypes.includes(type) ||
    !(scenarioID === null || (typeof scenarioID === "string" && isUuid(scenarioID))) ||
    !(categoryID === undefined || categoryID === null || (typeof categoryID === "string" && isUuid(categoryID))) ||
    (type === "ignore" && (scenarioID !== null || (categoryID !== null && categoryID !== undefined))) ||
    (type === "reconcile" && (scenarioID === null || (categoryID !== null && categoryID !== undefined))) ||
    (type === "set_category" && (categoryID == null || scenarioID !== null))
  ) {
    throw new TypeError("Ação inválida.");
  }
  const result: AutomationAction = { type, scenario_id: scenarioID as string | null };
  if (hasCategoryID) result.category_id = categoryID as string | null;
  return result;
}

export function parseAutomationRule(value: unknown): AutomationRule {
  const rule = requiredRecord(
    value,
    ["id", "name", "is_active", "logic_operator", "conditions", "actions", "created_at", "updated_at"],
    "Regra de automação inválida.",
  );
  if (
    typeof rule.id !== "string" ||
    !isUuid(rule.id) ||
    typeof rule.name !== "string" ||
    rule.name === "" ||
    typeof rule.is_active !== "boolean" ||
    !automationLogicOperators.includes(rule.logic_operator as AutomationLogicOperator) ||
    !Array.isArray(rule.conditions) ||
    rule.conditions.length === 0 ||
    !Array.isArray(rule.actions) ||
    rule.actions.length === 0 ||
    !isValidDate(rule.created_at) ||
    !isValidDate(rule.updated_at)
  ) {
    throw new TypeError("Regra de automação inválida.");
  }
  const actions = rule.actions.map(parseAutomationAction);
  const hasReconcileAction = actions.some((action) => action.type === "reconcile");
  return {
    id: rule.id,
    name: rule.name,
    is_active: rule.is_active,
    logic_operator: rule.logic_operator as AutomationLogicOperator,
    conditions: rule.conditions.map((c) => parseRuleCondition(c, hasReconcileAction)),
    actions,
    created_at: rule.created_at,
    updated_at: rule.updated_at,
  };
}

export function parseAutomationRuleList(value: unknown): AutomationRule[] {
  if (!Array.isArray(value)) {
    throw new TypeError("Lista de regras de automação inválida.");
  }
  return value.map(parseAutomationRule);
}

function parseRetroactiveApplyResult(value: unknown): RetroactiveApplyResult {
  const hasCategorized = isRecord(value) && "categorized" in value;
  const result = requiredRecord(
    value,
    hasCategorized ? ["matched", "ignored", "categorized"] : ["matched", "ignored"],
    "Resultado retroativo inválido.",
  );
  if (!isCount(result.matched) || !isCount(result.ignored) || (hasCategorized && !isCount(result.categorized))) {
    throw new TypeError("Resultado retroativo inválido.");
  }
  const parsed = { matched: result.matched, ignored: result.ignored } as RetroactiveApplyResult;
  if (hasCategorized) parsed.categorized = result.categorized as number;
  return parsed;
}

export function parseAutomationRuleWriteResult(value: unknown): AutomationRuleWriteResult {
  const result = requiredRecord(
    value,
    ["rule", "retroactive_apply"],
    "Resposta de regra de automação inválida.",
  );
  return {
    rule: parseAutomationRule(result.rule),
    retroactive_apply:
      result.retroactive_apply === null ? null : parseRetroactiveApplyResult(result.retroactive_apply),
  };
}

export const recurringCommitmentKinds = ["income", "expense"] as const;
export type RecurringCommitmentKind = (typeof recurringCommitmentKinds)[number];
export const recurringCommitmentCadences = ["monthly", "annual"] as const;
export type RecurringCommitmentCadence = (typeof recurringCommitmentCadences)[number];

export interface RecurringCommitment {
  /** The recurring Scenario's id: what the projection, the automation
   *  reconcile target and the occurrence decisions are all keyed by. */
  id: string;
  name: string;
  kind: RecurringCommitmentKind;
  amount: string;
  category_id: string;
  account_id: string | null;
  cadence: RecurringCommitmentCadence;
  day_of_month: number;
  month_of_year: number | null;
  start_date: string;
  end_date: string | null;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface RecurringCommitmentWrite {
  name: string;
  kind: RecurringCommitmentKind;
  amount: string;
  category_id: string;
  account_id: string | null;
  cadence: RecurringCommitmentCadence;
  day_of_month: number;
  month_of_year: number | null;
  start_date: string;
  end_date: string | null;
  is_active: boolean;
}

const isDayOfMonth = (value: unknown): value is number =>
  Number.isInteger(value) && typeof value === "number" && value >= 1 && value <= 31;

const isMonthOfYear = (value: unknown): value is number =>
  Number.isInteger(value) && typeof value === "number" && value >= 1 && value <= 12;

export function parseRecurringCommitment(value: unknown): RecurringCommitment {
  const commitment = requiredRecord(
    value,
    [
      "id",
      "name",
      "kind",
      "amount",
      "category_id",
      "account_id",
      "cadence",
      "day_of_month",
      "month_of_year",
      "start_date",
      "end_date",
      "is_active",
      "created_at",
      "updated_at",
    ],
    "Compromisso recorrente inválido.",
  );
  if (
    typeof commitment.id !== "string" ||
    !isUuid(commitment.id) ||
    typeof commitment.name !== "string" ||
    commitment.name === "" ||
    !recurringCommitmentKinds.includes(commitment.kind as RecurringCommitmentKind) ||
    typeof commitment.category_id !== "string" ||
    !isUuid(commitment.category_id) ||
    !(commitment.account_id === null || typeof commitment.account_id === "string") ||
    !recurringCommitmentCadences.includes(commitment.cadence as RecurringCommitmentCadence) ||
    !isDayOfMonth(commitment.day_of_month) ||
    !(commitment.month_of_year === null || isMonthOfYear(commitment.month_of_year)) ||
    !dateOnlyPattern.test(commitment.start_date as string) ||
    !(commitment.end_date === null || dateOnlyPattern.test(commitment.end_date as string)) ||
    typeof commitment.is_active !== "boolean" ||
    !isValidDate(commitment.created_at) ||
    !isValidDate(commitment.updated_at)
  ) {
    throw new TypeError("Compromisso recorrente inválido.");
  }
  return {
    id: commitment.id,
    name: commitment.name,
    kind: commitment.kind as RecurringCommitmentKind,
    amount: decimal(commitment.amount),
    category_id: commitment.category_id,
    account_id: commitment.account_id as string | null,
    cadence: commitment.cadence as RecurringCommitmentCadence,
    day_of_month: commitment.day_of_month,
    month_of_year: commitment.month_of_year as number | null,
    start_date: commitment.start_date as string,
    end_date: commitment.end_date as string | null,
    is_active: commitment.is_active,
    created_at: commitment.created_at,
    updated_at: commitment.updated_at,
  };
}

export function parseRecurringCommitmentList(value: unknown): RecurringCommitment[] {
  if (!Array.isArray(value)) {
    throw new TypeError("Lista de compromissos recorrentes inválida.");
  }
  return value.map(parseRecurringCommitment);
}

export const reconciliationStatuses = ["reconciled", "unreconciled", "detached"] as const;
export type ReconciliationStatus = (typeof reconciliationStatuses)[number];
export const reconciliationOrigins = ["rule", "manual"] as const;
export type ReconciliationOrigin = (typeof reconciliationOrigins)[number];

/**
 * One scheduled instance of a commitment, resolved against the user's manual
 * decisions and the automation rule. Occurrences aren't stored — the date is
 * their only identity, which is why every write below is keyed by it.
 */
export interface RecurrenceOccurrence {
  date: string;
  expected_amount: string;
  status: ReconciliationStatus;
  /** Who decided it: "rule" for the automation, "manual" for the user. Null when unreconciled. */
  origin: ReconciliationOrigin | null;
  transaction: EligibleTransaction | null;
}

export function parseRecurrenceOccurrence(value: unknown): RecurrenceOccurrence {
  const occurrence = requiredRecord(
    value,
    ["date", "expected_amount", "status", "origin", "transaction"],
    "Ocorrência inválida.",
  );
  if (
    !dateOnlyPattern.test(occurrence.date as string) ||
    !reconciliationStatuses.includes(occurrence.status as ReconciliationStatus) ||
    !(
      occurrence.origin === null ||
      reconciliationOrigins.includes(occurrence.origin as ReconciliationOrigin)
    )
  ) {
    throw new TypeError("Ocorrência inválida.");
  }
  return {
    date: occurrence.date as string,
    expected_amount: decimal(occurrence.expected_amount),
    status: occurrence.status as ReconciliationStatus,
    origin: occurrence.origin as ReconciliationOrigin | null,
    transaction:
      occurrence.transaction === null ? null : parseEligibleTransaction(occurrence.transaction),
  };
}

export function parseRecurrenceOccurrenceList(value: unknown): RecurrenceOccurrence[] {
  if (!Array.isArray(value)) {
    throw new TypeError("Lista de ocorrências inválida.");
  }
  return value.map(parseRecurrenceOccurrence);
}

/** The body of a reconciliation write: pick a transaction, or detach the occurrence. */
export type ReconciliationWrite =
  | { state: "linked"; transaction_id: string }
  | { state: "detached" };

/** One occurrence a transaction could settle, seen from the transaction's side. */
export interface ReconciliationOption {
  commitment_id: string;
  commitment_name: string;
  kind: RecurringCommitmentKind;
  occurrence_date: string;
  expected_amount: string;
}

export interface CurrentReconciliation extends ReconciliationOption {
  origin: ReconciliationOrigin;
}

/**
 * What a transaction currently settles plus what it could settle, in one
 * payload — the transaction drawer needs both at once.
 */
export interface TransactionReconciliation {
  current: CurrentReconciliation | null;
  options: ReconciliationOption[];
}

const reconciliationOptionKeys = [
  "commitment_id",
  "commitment_name",
  "kind",
  "occurrence_date",
  "expected_amount",
] as const;

function reconciliationOptionFieldsFrom(option: Record<string, unknown>): ReconciliationOption {
  if (
    typeof option.commitment_id !== "string" ||
    !isUuid(option.commitment_id) ||
    typeof option.commitment_name !== "string" ||
    option.commitment_name === "" ||
    !recurringCommitmentKinds.includes(option.kind as RecurringCommitmentKind) ||
    !dateOnlyPattern.test(option.occurrence_date as string)
  ) {
    throw new TypeError("Opção de conciliação inválida.");
  }
  return {
    commitment_id: option.commitment_id,
    commitment_name: option.commitment_name,
    kind: option.kind as RecurringCommitmentKind,
    occurrence_date: option.occurrence_date as string,
    expected_amount: decimal(option.expected_amount),
  };
}

function parseReconciliationOption(value: unknown): ReconciliationOption {
  return reconciliationOptionFieldsFrom(
    requiredRecord(value, reconciliationOptionKeys, "Opção de conciliação inválida."),
  );
}

function parseCurrentReconciliation(value: unknown): CurrentReconciliation {
  const current = requiredRecord(
    value,
    [...reconciliationOptionKeys, "origin"],
    "Conciliação atual inválida.",
  );
  if (!reconciliationOrigins.includes(current.origin as ReconciliationOrigin)) {
    throw new TypeError("Conciliação atual inválida.");
  }
  return {
    ...reconciliationOptionFieldsFrom(current),
    origin: current.origin as ReconciliationOrigin,
  };
}

export function parseTransactionReconciliation(value: unknown): TransactionReconciliation {
  const payload = requiredRecord(value, ["current", "options"], "Conciliação inválida.");
  if (!Array.isArray(payload.options)) {
    throw new TypeError("Conciliação inválida.");
  }
  return {
    current: payload.current === null ? null : parseCurrentReconciliation(payload.current),
    options: payload.options.map(parseReconciliationOption),
  };
}

export interface Category {
  id: string;
  name: string;
  kind: CategoryKind;
  is_active: boolean;
  icon: string;
  color: string;
  created_at: string;
  updated_at: string;
}

export interface CategoryCreate {
  name: string;
  kind: CategoryKind;
  icon: string;
  color: string;
}

export interface CategoryUpdate {
  name?: string;
  is_active?: boolean;
  icon?: string;
  color?: string;
}

export function parseCategory(value: unknown): Category {
  const category = requiredRecord(
    value,
    ["id", "name", "kind", "is_active", "icon", "color", "created_at", "updated_at"],
    "Categoria inválida.",
  );
  if (
    typeof category.id !== "string" ||
    !isUuid(category.id) ||
    typeof category.name !== "string" ||
    category.name === "" ||
    !categoryKinds.includes(category.kind as CategoryKind) ||
    typeof category.is_active !== "boolean" ||
    typeof category.icon !== "string" ||
    category.icon === "" ||
    typeof category.color !== "string" ||
    category.color === "" ||
    !isValidDate(category.created_at) ||
    !isValidDate(category.updated_at)
  ) {
    throw new TypeError("Categoria inválida.");
  }
  return {
    id: category.id,
    name: category.name,
    kind: category.kind as CategoryKind,
    is_active: category.is_active,
    icon: category.icon,
    color: category.color,
    created_at: category.created_at as string,
    updated_at: category.updated_at as string,
  };
}

export function parseCategoryList(value: unknown): Category[] {
  if (!Array.isArray(value)) {
    throw new TypeError("Lista de categorias inválida.");
  }
  return value.map(parseCategory);
}

export type TransactionsPeriodBasis = "occurred_at" | "paid_at";

export interface Preferences {
  transactions_period_basis: TransactionsPeriodBasis;
}

export function parsePreferences(value: unknown): Preferences {
  const preferences = requiredRecord(value, ["transactions_period_basis"], "Preferências inválidas.");
  if (preferences.transactions_period_basis !== "occurred_at" && preferences.transactions_period_basis !== "paid_at") {
    throw new TypeError("Preferências inválidas.");
  }
  return { transactions_period_basis: preferences.transactions_period_basis };
}

function parseTransactionGroup(value: unknown): TransactionGroup {
  const group = requiredRecord(value, [
    "key",
    "kind",
    "start_date",
    "end_date",
    "item_count",
    "page_item_count",
    "has_items_before",
    "has_items_after",
    "totals",
  ]);
  const kinds = [...transactionGroupings, "undated"];
  if (
    typeof group.key !== "string" ||
    !kinds.includes(group.kind as TransactionGrouping | "undated") ||
    !(
      group.start_date === null ||
      (typeof group.start_date === "string" && dateOnlyPattern.test(group.start_date))
    ) ||
    !(
      group.end_date === null ||
      (typeof group.end_date === "string" && dateOnlyPattern.test(group.end_date))
    ) ||
    typeof group.has_items_before !== "boolean" ||
    typeof group.has_items_after !== "boolean" ||
    !Array.isArray(group.totals)
  ) {
    throw new TypeError("Grupo de transações inválido.");
  }
  return {
    key: group.key,
    kind: group.kind as TransactionGrouping | "undated",
    start_date: group.start_date as string | null,
    end_date: group.end_date as string | null,
    item_count: positiveCount(group.item_count),
    page_item_count: positiveCount(group.page_item_count),
    has_items_before: group.has_items_before,
    has_items_after: group.has_items_after,
    totals: group.totals.map(parseCurrencyTotals),
  };
}

export function parseTransactionQueryResult(value: unknown): TransactionQueryResult {
  const result = requiredRecord(value, [
    "confirmed_at",
    "stored_total",
    "page",
    "items",
    "totals",
    "groups",
    "available_filters",
  ]);
  const page = requiredRecord(result.page, ["number", "size", "total_items", "total_pages"]);
  const filters = requiredRecord(result.available_filters, [
    "accounts",
    "institutions",
    "categories",
  ]);
  if (
    !isValidDate(result.confirmed_at) ||
    !isCount(result.stored_total) ||
    !Array.isArray(result.items) ||
    !Array.isArray(result.totals) ||
    !Array.isArray(result.groups) ||
    !Array.isArray(filters.accounts) ||
    !Array.isArray(filters.institutions) ||
    !Array.isArray(filters.categories)
  ) {
    throw new TypeError("Resposta de transações inválida.");
  }
  const accounts = filters.accounts.map((value) => {
    const account = requiredRecord(value, ["id", "name", "institution"]);
    if (typeof account.id !== "string" || !isUuid(account.id)) {
      throw new TypeError("Opção de conta inválida.");
    }
    return {
      id: account.id,
      name: nullableText(account.name),
      institution: nullableText(account.institution),
    };
  });
  if (!filters.institutions.every((item) => typeof item === "string")) {
    throw new TypeError("Facetas inválidas.");
  }
  const categories = filters.categories.map((value) => {
    const option = requiredRecord(value, ["id", "name", "kind", "is_active", "icon", "color"]);
    if (
      typeof option.id !== "string" ||
      !isUuid(option.id) ||
      typeof option.name !== "string" ||
      option.name === "" ||
      !categoryKinds.includes(option.kind as CategoryKind) ||
      typeof option.is_active !== "boolean" ||
      typeof option.icon !== "string" ||
      typeof option.color !== "string"
    ) {
      throw new TypeError("Opção de categoria inválida.");
    }
    return {
      id: option.id,
      name: option.name,
      kind: option.kind as CategoryKind,
      is_active: option.is_active,
      icon: option.icon,
      color: option.color,
    };
  });
  const number = positiveCount(page.number);
  const size = positiveCount(page.size);
  const totalItems = isCount(page.total_items) ? page.total_items : NaN;
  const totalPages = isCount(page.total_pages) ? page.total_pages : NaN;
  if (size > 100 || Number.isNaN(totalItems) || Number.isNaN(totalPages)) {
    throw new TypeError("Paginação inválida.");
  }
  return {
    confirmed_at: result.confirmed_at,
    stored_total: result.stored_total,
    page: { number, size, total_items: totalItems, total_pages: totalPages },
    items: result.items.map(parseTransactionItem),
    totals: result.totals.map(parseCurrencyTotals),
    groups: result.groups.map(parseTransactionGroup),
    available_filters: {
      accounts,
      institutions: filters.institutions as string[],
      categories,
    },
  };
}

export function parseSyncRun(value: unknown): SyncRun {
  if (
    !isRecord(value) ||
    typeof value.id !== "string" ||
    !isUuid(value.id) ||
    typeof value.source_id !== "string" ||
    !isUuid(value.source_id) ||
    typeof value.source_name !== "string" ||
    !syncStatuses.includes(value.status as SyncStatus) ||
    !isValidDate(value.started_at) ||
    !(value.finished_at === null || isValidDate(value.finished_at)) ||
    !isCount(value.accounts_processed) ||
    !isCount(value.transactions_inserted) ||
    !isCount(value.transactions_updated) ||
    !isNullableString(value.result_message)
  ) {
    throw new TypeError("Resposta de execução inválida.");
  }

  return {
    id: value.id,
    status: value.status as SyncStatus,
    source_id: value.source_id,
    source_name: value.source_name,
    started_at: value.started_at,
    finished_at: value.finished_at,
    accounts_processed: value.accounts_processed,
    transactions_inserted: value.transactions_inserted,
    transactions_updated: value.transactions_updated,
    result_message: value.result_message,
  };
}

function parseFailure(value: unknown): SyncFailure {
  if (
    !isRecord(value) ||
    !failureStages.includes(value.stage as FailureStage) ||
    typeof value.code !== "string" ||
    typeof value.message !== "string" ||
    !isNullableString(value.external_account_id) ||
    !isNullableString(value.external_transaction_id) ||
    !isValidDate(value.occurred_at)
  ) {
    throw new TypeError("Falha de sincronização inválida.");
  }

  return {
    stage: value.stage as FailureStage,
    code: value.code,
    message: value.message,
    external_account_id: value.external_account_id,
    external_transaction_id: value.external_transaction_id,
    occurred_at: value.occurred_at,
  };
}

export function parseSyncRunDetail(value: unknown): SyncRunDetail {
  if (!isRecord(value) || !Array.isArray(value.failures)) {
    throw new TypeError("Detalhe de execução inválido.");
  }
  return { ...parseSyncRun(value), failures: value.failures.map(parseFailure) };
}

export function parseSyncRunList(value: unknown): SyncRun[] {
  if (!Array.isArray(value)) {
    throw new TypeError("Lista de execuções inválida.");
  }
  return value.map(parseSyncRun);
}

export function parseCreateSyncRunResult(value: unknown): CreateSyncRunResult {
  if (!isRecord(value) || !Array.isArray(value.runs) || !isCount(value.requested)) {
    throw new TypeError("Resposta de criação de execução inválida.");
  }
  return { runs: value.runs.map(parseSyncRun), requested: value.requested };
}

export function parseDataSource(value: unknown): DataSource {
  if (
    !isRecord(value) ||
    typeof value.id !== "string" ||
    !isUuid(value.id) ||
    typeof value.provider !== "string" ||
    typeof value.external_item_id !== "string" ||
    !isNullableString(value.display_name) ||
    !isNullableString(value.label) ||
    typeof value.name !== "string" ||
    typeof value.is_active !== "boolean" ||
    !isValidDate(value.created_at) ||
    !isValidDate(value.updated_at)
  ) {
    throw new TypeError("Conexão inválida.");
  }

  return {
    id: value.id,
    provider: value.provider,
    external_item_id: value.external_item_id,
    display_name: value.display_name,
    label: value.label,
    name: value.name,
    is_active: value.is_active,
    created_at: value.created_at,
    updated_at: value.updated_at,
  };
}

export function parseDataSourceList(value: unknown): DataSource[] {
  if (!Array.isArray(value)) {
    throw new TypeError("Lista de conexões inválida.");
  }
  return value.map(parseDataSource);
}

export const payableKinds = ["debt", "receivable"] as const;
export type PayableKind = (typeof payableKinds)[number];

export const payableStatuses = ["open", "settled"] as const;
export type PayableStatus = (typeof payableStatuses)[number];

export interface Payable {
  id: string;
  kind: PayableKind;
  name: string;
  total_amount: string;
  starting_settled_amount: string;
  settled_amount: string;
  remaining_amount: string;
  status: PayableStatus;
  link_count: number;
  created_at: string;
  updated_at: string;
}

export interface PayableLinkedTransaction {
  id: string;
  transaction_id: string;
  occurred_at: string | null;
  description: string | null;
  linked_amount: string;
  current_amount: string;
  linked_at: string;
}

export interface PayableDetail extends Payable {
  links: PayableLinkedTransaction[];
}

export interface EligibleTransaction {
  id: string;
  occurred_at: string | null;
  description: string | null;
  account_name: string | null;
  effective_money: { value: string; currency_code: string };
}

export interface PayableCreate {
  kind: PayableKind;
  name: string;
  total_amount: number;
  initial_remaining_amount?: number | null;
}

export interface PayableUpdate {
  name: string;
  total_amount: number;
}

export interface PayableLinkCreate {
  transaction_id: string;
}

export interface PayableLink {
  id: string;
  transaction_id: string;
  linked_amount: string;
  linked_at: string;
}

const payableKeys = [
  "id",
  "kind",
  "name",
  "total_amount",
  "starting_settled_amount",
  "settled_amount",
  "remaining_amount",
  "status",
  "link_count",
  "created_at",
  "updated_at",
] as const;

function payableFieldsFrom(payable: Record<string, unknown>): Payable {
  if (
    typeof payable.id !== "string" ||
    !isUuid(payable.id) ||
    !payableKinds.includes(payable.kind as PayableKind) ||
    typeof payable.name !== "string" ||
    payable.name === "" ||
    !payableStatuses.includes(payable.status as PayableStatus) ||
    !isCount(payable.link_count) ||
    !isValidDate(payable.created_at) ||
    !isValidDate(payable.updated_at)
  ) {
    throw new TypeError("Pendência inválida.");
  }
  return {
    id: payable.id,
    kind: payable.kind as PayableKind,
    name: payable.name,
    total_amount: decimal(payable.total_amount),
    starting_settled_amount: decimal(payable.starting_settled_amount),
    settled_amount: decimal(payable.settled_amount),
    remaining_amount: decimal(payable.remaining_amount),
    status: payable.status as PayableStatus,
    link_count: payable.link_count,
    created_at: payable.created_at,
    updated_at: payable.updated_at,
  };
}

export function parsePayable(value: unknown): Payable {
  return payableFieldsFrom(requiredRecord(value, payableKeys, "Pendência inválida."));
}

export function parsePayableList(value: unknown): Payable[] {
  if (!Array.isArray(value)) {
    throw new TypeError("Lista de pendências inválida.");
  }
  return value.map(parsePayable);
}

export interface PayableTotalOwed {
  remaining_debts_total: string;
  future_installments_total: string;
  total_owed: string;
  currency_code: string;
}

export function parsePayableTotalOwed(value: unknown): PayableTotalOwed {
  const item = requiredRecord(
    value,
    ["remaining_debts_total", "future_installments_total", "total_owed", "currency_code"],
    "Total de dívida inválido.",
  );
  if (typeof item.currency_code !== "string" || item.currency_code === "") {
    throw new TypeError("Total de dívida inválido.");
  }
  return {
    remaining_debts_total: decimal(item.remaining_debts_total),
    future_installments_total: decimal(item.future_installments_total),
    total_owed: decimal(item.total_owed),
    currency_code: item.currency_code,
  };
}

export interface PayableTotalToReceive {
  remaining_receivables_total: string;
  total_to_receive: string;
  currency_code: string;
}

export function parsePayableTotalToReceive(value: unknown): PayableTotalToReceive {
  const item = requiredRecord(
    value,
    ["remaining_receivables_total", "total_to_receive", "currency_code"],
    "Total a receber inválido.",
  );
  if (typeof item.currency_code !== "string" || item.currency_code === "") {
    throw new TypeError("Total a receber inválido.");
  }
  return {
    remaining_receivables_total: decimal(item.remaining_receivables_total),
    total_to_receive: decimal(item.total_to_receive),
    currency_code: item.currency_code,
  };
}

function parsePayableLinkedTransaction(value: unknown): PayableLinkedTransaction {
  const link = requiredRecord(
    value,
    [
      "id",
      "transaction_id",
      "occurred_at",
      "description",
      "linked_amount",
      "current_amount",
      "linked_at",
    ],
    "Vínculo inválido.",
  );
  if (
    typeof link.id !== "string" ||
    !isUuid(link.id) ||
    typeof link.transaction_id !== "string" ||
    !isUuid(link.transaction_id) ||
    !(link.occurred_at === null || isValidDate(link.occurred_at)) ||
    !isNullableString(link.description) ||
    !isValidDate(link.linked_at)
  ) {
    throw new TypeError("Vínculo inválido.");
  }
  return {
    id: link.id,
    transaction_id: link.transaction_id,
    occurred_at: link.occurred_at as string | null,
    description: nullableText(link.description),
    linked_amount: decimal(link.linked_amount),
    current_amount: decimal(link.current_amount),
    linked_at: link.linked_at,
  };
}

export function parsePayableDetail(value: unknown): PayableDetail {
  const detail = requiredRecord(value, [...payableKeys, "links"], "Detalhe de pendência inválido.");
  if (!Array.isArray(detail.links)) {
    throw new TypeError("Detalhe de pendência inválido.");
  }
  return { ...payableFieldsFrom(detail), links: detail.links.map(parsePayableLinkedTransaction) };
}

export function parsePayableLink(value: unknown): PayableLink {
  const link = requiredRecord(
    value,
    ["id", "transaction_id", "linked_amount", "linked_at"],
    "Vínculo inválido.",
  );
  if (
    typeof link.id !== "string" ||
    !isUuid(link.id) ||
    typeof link.transaction_id !== "string" ||
    !isUuid(link.transaction_id) ||
    !isValidDate(link.linked_at)
  ) {
    throw new TypeError("Vínculo inválido.");
  }
  return {
    id: link.id,
    transaction_id: link.transaction_id,
    linked_amount: decimal(link.linked_amount),
    linked_at: link.linked_at,
  };
}

export function parseEligibleTransaction(value: unknown): EligibleTransaction {
  const item = requiredRecord(
    value,
    ["id", "occurred_at", "description", "account_name", "effective_money"],
    "Transação elegível inválida.",
  );
  const money = requiredRecord(
    item.effective_money,
    ["value", "currency_code"],
    "Valor efetivo inválido.",
  );
  if (
    typeof item.id !== "string" ||
    !isUuid(item.id) ||
    !(item.occurred_at === null || isValidDate(item.occurred_at)) ||
    !isNullableString(item.description) ||
    !isNullableString(item.account_name) ||
    typeof money.currency_code !== "string" ||
    money.currency_code === ""
  ) {
    throw new TypeError("Transação elegível inválida.");
  }
  return {
    id: item.id,
    occurred_at: item.occurred_at as string | null,
    description: nullableText(item.description),
    account_name: nullableText(item.account_name),
    effective_money: { value: decimal(money.value), currency_code: money.currency_code },
  };
}

export function parseEligibleTransactionList(value: unknown): EligibleTransaction[] {
  if (!Array.isArray(value)) {
    throw new TypeError("Lista de transações elegíveis inválida.");
  }
  return value.map(parseEligibleTransaction);
}

export const scenarioKinds = ["debt_plan", "receivable_plan", "standalone", "recurring"] as const;
export type ScenarioKind = (typeof scenarioKinds)[number];

export interface Scenario {
  id: string;
  kind: ScenarioKind;
  name: string;
  payable_id: string | null;
  // Optional only for responses from the pre-unification compatibility API;
  // the canonical Scenario endpoint always supplies both booleans.
  is_active?: boolean;
  is_accounting_source?: boolean;
  created_at: string;
  updated_at: string;
}

export const scenarioTransactionStatuses = [
  "atrasada",
  "projetada",
  "paga_parcialmente",
  "paga",
  "paga_a_mais",
] as const;
export type ScenarioTransactionStatus = (typeof scenarioTransactionStatuses)[number];

export interface Realization {
  id: string;
  payable_link_id: string | null;
  allocated_amount: string;
  created_at: string;
}

export interface ScenarioTransaction {
  id: string;
  scenario_id: string;
  description: string;
  amount: string;
  projected_at: string;
  category: string | null;
  status: ScenarioTransactionStatus;
  realizations: Realization[];
}

export interface ScenarioDetail extends Scenario {
  transactions: ScenarioTransaction[];
  accumulated_deviation: string;
}

export interface ScenarioCreate {
  name: string;
}

export interface ScenarioTransactionWrite {
  description: string;
  amount: number;
  projected_at: string;
  category?: string | null;
}

export interface RealizationWrite {
  payable_link_id: string;
  allocated_amount: number;
}

export const cadences = ["mensal", "semanal", "quinzenal"] as const;
export type Cadence = (typeof cadences)[number];

export interface GenerateInstallmentsWrite {
  cadence: Cadence;
  months?: number;
  installment_amount?: number;
  start_date?: string;
}

export const readjustStrategies = ["abater_do_final", "redistribuir"] as const;
export type ReadjustStrategy = (typeof readjustStrategies)[number];

export interface ReadjustWrite {
  strategy: ReadjustStrategy;
}

const legacyScenarioKeys = ["id", "kind", "name", "payable_id", "created_at", "updated_at"] as const;
const scenarioKeys = [
  ...legacyScenarioKeys,
  "is_active",
  "is_accounting_source",
] as const;

function isNullableUuid(value: unknown): value is string | null {
  return value === null || (typeof value === "string" && isUuid(value));
}

function scenarioFieldsFrom(scenario: Record<string, unknown>): Scenario {
  if (
    typeof scenario.id !== "string" ||
    !isUuid(scenario.id) ||
    !scenarioKinds.includes(scenario.kind as ScenarioKind) ||
    typeof scenario.name !== "string" ||
    scenario.name === "" ||
    !isNullableUuid(scenario.payable_id) ||
    (scenario.is_active !== undefined && typeof scenario.is_active !== "boolean") ||
    (scenario.is_accounting_source !== undefined && typeof scenario.is_accounting_source !== "boolean") ||
    !isValidDate(scenario.created_at) ||
    !isValidDate(scenario.updated_at)
  ) {
    throw new TypeError("Cenário inválido.");
  }
  const result: Scenario = {
    id: scenario.id,
    kind: scenario.kind as ScenarioKind,
    name: scenario.name,
    payable_id: scenario.payable_id as string | null,
    created_at: scenario.created_at,
    updated_at: scenario.updated_at,
  };
  if (scenario.is_active !== undefined) result.is_active = scenario.is_active as boolean;
  if (scenario.is_accounting_source !== undefined) {
    result.is_accounting_source = scenario.is_accounting_source as boolean;
  }
  return result;
}

export function parseScenario(value: unknown): Scenario {
  const hasActivation = isRecord(value) && "is_active" in value;
  const record = requiredRecord(value, hasActivation ? scenarioKeys : legacyScenarioKeys, "Cenário inválido.");
  return scenarioFieldsFrom(record);
}

export function parseScenarioList(value: unknown): Scenario[] {
  if (!Array.isArray(value)) {
    throw new TypeError("Lista de cenários inválida.");
  }
  return value.map(parseScenario);
}

const realizationKeys = ["id", "payable_link_id", "allocated_amount", "created_at"] as const;

function parseRealization(value: unknown): Realization {
  const item = requiredRecord(value, realizationKeys, "Alocação inválida.");
  if (
    typeof item.id !== "string" ||
    !isUuid(item.id) ||
    !isNullableUuid(item.payable_link_id) ||
    !isValidDate(item.created_at)
  ) {
    throw new TypeError("Alocação inválida.");
  }
  return {
    id: item.id,
    payable_link_id: item.payable_link_id as string | null,
    allocated_amount: decimal(item.allocated_amount),
    created_at: item.created_at,
  };
}

function parseRealizationList(value: unknown): Realization[] {
  if (!Array.isArray(value)) {
    throw new TypeError("Lista de alocações inválida.");
  }
  return value.map(parseRealization);
}

const scenarioTransactionKeys = [
  "id",
  "scenario_id",
  "description",
  "amount",
  "projected_at",
  "category",
  "status",
  "realizations",
] as const;

export function parseScenarioTransaction(value: unknown): ScenarioTransaction {
  const item = requiredRecord(value, scenarioTransactionKeys, "Parcela inválida.");
  if (
    typeof item.id !== "string" ||
    !isUuid(item.id) ||
    typeof item.scenario_id !== "string" ||
    !isUuid(item.scenario_id) ||
    typeof item.description !== "string" ||
    item.description === "" ||
    typeof item.projected_at !== "string" ||
    !dateOnlyPattern.test(item.projected_at) ||
    !isNullableString(item.category) ||
    !scenarioTransactionStatuses.includes(item.status as ScenarioTransactionStatus)
  ) {
    throw new TypeError("Parcela inválida.");
  }
  return {
    id: item.id,
    scenario_id: item.scenario_id,
    description: item.description,
    amount: decimal(item.amount),
    projected_at: item.projected_at,
    category: item.category,
    status: item.status as ScenarioTransactionStatus,
    realizations: parseRealizationList(item.realizations),
  };
}

export function parseScenarioTransactionList(value: unknown): ScenarioTransaction[] {
  if (!Array.isArray(value)) {
    throw new TypeError("Lista de parcelas inválida.");
  }
  return value.map(parseScenarioTransaction);
}

export function parseScenarioDetail(value: unknown): ScenarioDetail {
  const keys = isRecord(value) && "is_active" in value
    ? [...scenarioKeys, "transactions", "accumulated_deviation"]
    : [...legacyScenarioKeys, "transactions", "accumulated_deviation"];
  const detail = requiredRecord(
    value,
    keys,
    "Detalhe de cenário inválido.",
  );
  if (!Array.isArray(detail.transactions)) {
    throw new TypeError("Detalhe de cenário inválido.");
  }
  return {
    ...scenarioFieldsFrom(detail),
    transactions: detail.transactions.map(parseScenarioTransaction),
    accumulated_deviation: decimal(detail.accumulated_deviation),
  };
}

export const projectionTiers = ["realizado", "confirmado", "projetado", "hipotetico"] as const;
export type ProjectionTier = (typeof projectionTiers)[number];
// Planned transactions use the same serialized source vocabulary as the
// Timeline endpoint. Keeping both parsers on one set of values prevents a
// recurring/plan event returned by /planned-transactions from being rejected
// while the Timeline accepts it.
export const projectionSources = ["real", "recorrente", "plano_pagamento", "cenario"] as const;
export type ProjectionSource = (typeof projectionSources)[number];

export interface PlannedTransaction {
  scenario_id: string;
  event_key: string;
  scenario_kind: ScenarioKind;
  date: string;
  description: string;
  amount: string;
  category_id: string | null;
  category_name: string;
  tier: ProjectionTier;
  source: ProjectionSource;
  payable_id: string | null;
  realized: boolean;
  realization_origin: string;
  detached: boolean;
}

const plannedTransactionKeys = [
  "scenario_id",
  "event_key",
  "scenario_kind",
  "date",
  "description",
  "amount",
  "category_id",
  "category_name",
  "tier",
  "source",
  "payable_id",
  "realized",
  "realization_origin",
  "detached",
] as const;

export function parsePlannedTransaction(value: unknown): PlannedTransaction {
  const item = requiredRecord(value, plannedTransactionKeys, "Evento previsto inválido.");
  if (
    typeof item.scenario_id !== "string" ||
    !isUuid(item.scenario_id) ||
    typeof item.event_key !== "string" ||
    item.event_key === "" ||
    !scenarioKinds.includes(item.scenario_kind as ScenarioKind) ||
    typeof item.date !== "string" ||
    !dateOnlyPattern.test(item.date) ||
    typeof item.description !== "string" ||
    item.description === "" ||
    !isNullableUuid(item.category_id) ||
    typeof item.category_name !== "string" ||
    !projectionTiers.includes(item.tier as ProjectionTier) ||
    !projectionSources.includes(item.source as ProjectionSource) ||
    !isNullableUuid(item.payable_id) ||
    typeof item.realized !== "boolean" ||
    typeof item.realization_origin !== "string" ||
    typeof item.detached !== "boolean"
  ) {
    throw new TypeError("Evento previsto inválido.");
  }
  return {
    scenario_id: item.scenario_id,
    event_key: item.event_key,
    scenario_kind: item.scenario_kind as ScenarioKind,
    date: item.date,
    description: item.description,
    amount: decimal(item.amount),
    category_id: item.category_id as string | null,
    category_name: item.category_name,
    tier: item.tier as ProjectionTier,
    source: item.source as ProjectionSource,
    payable_id: item.payable_id as string | null,
    realized: item.realized,
    realization_origin: item.realization_origin,
    detached: item.detached,
  };
}

export function parsePlannedTransactionList(value: unknown): PlannedTransaction[] {
  if (!Array.isArray(value)) throw new TypeError("Lista de eventos previstos inválida.");
  return value.map(parsePlannedTransaction);
}

export interface ScenarioRealization {
  id: string;
  scenario_id: string;
  scenario_transaction_id: string | null;
  occurrence_date: string | null;
  transaction_id: string | null;
  relation_type: "settlement" | "allocation" | "reconciliation";
  state: "linked" | "detached";
  origin: string;
  allocated_amount: string | null;
  linked_amount: string | null;
  created_at: string;
}

const scenarioRealizationKeys = [
  "id",
  "scenario_id",
  "scenario_transaction_id",
  "occurrence_date",
  "transaction_id",
  "relation_type",
  "state",
  "origin",
  "allocated_amount",
  "linked_amount",
  "created_at",
] as const;

export function parseScenarioRealization(value: unknown): ScenarioRealization {
  const item = requiredRecord(value, scenarioRealizationKeys, "Realização inválida.");
  if (
    typeof item.id !== "string" ||
    !isUuid(item.id) ||
    typeof item.scenario_id !== "string" ||
    !isUuid(item.scenario_id) ||
    !isNullableUuid(item.scenario_transaction_id) ||
    !(item.occurrence_date === null || (typeof item.occurrence_date === "string" && dateOnlyPattern.test(item.occurrence_date))) ||
    !isNullableUuid(item.transaction_id) ||
    !["settlement", "allocation", "reconciliation"].includes(item.relation_type as string) ||
    !["linked", "detached"].includes(item.state as string) ||
    typeof item.origin !== "string" ||
    !isNullableString(item.allocated_amount) ||
    !isNullableString(item.linked_amount) ||
    !isValidDate(item.created_at)
  ) {
    throw new TypeError("Realização inválida.");
  }
  return {
    id: item.id,
    scenario_id: item.scenario_id,
    scenario_transaction_id: item.scenario_transaction_id as string | null,
    occurrence_date: item.occurrence_date as string | null,
    transaction_id: item.transaction_id as string | null,
    relation_type: item.relation_type as ScenarioRealization["relation_type"],
    state: item.state as ScenarioRealization["state"],
    origin: item.origin,
    allocated_amount: nullableDecimal(item.allocated_amount),
    linked_amount: nullableDecimal(item.linked_amount),
    created_at: item.created_at,
  };
}

/**
 * Why the backend could not state a yield. "sem_historico": no movements
 * synced at all. "historico_incompleto": movements arrived, but they cannot
 * account for the position (no aplicação behind the resgates, more cotas sold
 * than bought, or a movement whose direction or amount is unknown) — netting
 * them would report principal as profit. "saldo_indisponivel": the history
 * nets fine, there is just no current balance to net it against.
 */
export const yieldUnavailableReasons = [
  "sem_historico",
  "historico_incompleto",
  "saldo_indisponivel",
] as const;
export type YieldUnavailableReason = (typeof yieldUnavailableReasons)[number];

export interface Investment {
  id: string;
  external_id: string;
  source_display_name: string | null;
  investment_type: string | null;
  subtype: string | null;
  name: string | null;
  balance: string | null;
  currency_code: string | null;
  quantity: string | null;
  value: string | null;
  amount: string | null;
  amount_profit: string | null;
  amount_withdrawal: string | null;
  rate: string | null;
  rate_type: string | null;
  fixed_annual_rate: string | null;
  annual_rate: string | null;
  last_twelve_months_rate: string | null;
  issuer: string | null;
  due_date: string | null;
  as_of_date: string | null;
  provider_updated_at: string | null;
  yield_value: string | null;
  yield_source: "informado" | "calculado" | null;
  /** Why yield_value is null. Always null when yield_value is present. */
  yield_unavailable_reason: YieldUnavailableReason | null;
}

const investmentKeys = [
  "id",
  "external_id",
  "source_display_name",
  "investment_type",
  "subtype",
  "name",
  "balance",
  "currency_code",
  "quantity",
  "value",
  "amount",
  "amount_profit",
  "amount_withdrawal",
  "rate",
  "rate_type",
  "fixed_annual_rate",
  "annual_rate",
  "last_twelve_months_rate",
  "issuer",
  "due_date",
  "as_of_date",
  "provider_updated_at",
  "yield_value",
  "yield_source",
  "yield_unavailable_reason",
] as const;

export function parseInvestment(value: unknown): Investment {
  const item = requiredRecord(value, investmentKeys, "Investimento inválido.");
  if (
    typeof item.id !== "string" ||
    !isUuid(item.id) ||
    typeof item.external_id !== "string" ||
    item.external_id === "" ||
    !isNullableString(item.source_display_name) ||
    !isNullableString(item.investment_type) ||
    !isNullableString(item.subtype) ||
    !isNullableString(item.name) ||
    !isNullableString(item.currency_code) ||
    !isNullableString(item.rate_type) ||
    !isNullableString(item.issuer) ||
    !(item.yield_source === null || item.yield_source === "informado" || item.yield_source === "calculado") ||
    (item.yield_source === null) !== (item.yield_value === null) ||
    !(item.yield_unavailable_reason === null ||
      yieldUnavailableReasons.includes(item.yield_unavailable_reason as YieldUnavailableReason))
  ) {
    throw new TypeError("Investimento inválido.");
  }
  return {
    id: item.id,
    external_id: item.external_id,
    source_display_name: item.source_display_name,
    investment_type: item.investment_type,
    subtype: item.subtype,
    name: item.name,
    balance: nullableDecimal(item.balance),
    currency_code: item.currency_code,
    quantity: nullableDecimal(item.quantity),
    value: nullableDecimal(item.value),
    amount: nullableDecimal(item.amount),
    amount_profit: nullableDecimal(item.amount_profit),
    amount_withdrawal: nullableDecimal(item.amount_withdrawal),
    rate: nullableDecimal(item.rate),
    rate_type: item.rate_type,
    fixed_annual_rate: nullableDecimal(item.fixed_annual_rate),
    annual_rate: nullableDecimal(item.annual_rate),
    last_twelve_months_rate: nullableDecimal(item.last_twelve_months_rate),
    issuer: item.issuer,
    due_date: nullableDate(item.due_date),
    as_of_date: nullableDate(item.as_of_date),
    provider_updated_at: nullableDate(item.provider_updated_at),
    yield_value: nullableDecimal(item.yield_value),
    yield_source: item.yield_source as "informado" | "calculado" | null,
    yield_unavailable_reason: item.yield_unavailable_reason as YieldUnavailableReason | null,
  };
}

export function parseInvestmentList(value: unknown): Investment[] {
  if (!Array.isArray(value)) {
    throw new TypeError("Lista de investimentos inválida.");
  }
  return value.map(parseInvestment);
}

/** Which way the money moved, normalized by the backend at ingestion. */
export const investmentDirections = ["inflow", "outflow"] as const;
export type InvestmentDirection = (typeof investmentDirections)[number];

export interface InvestmentTransaction {
  id: string;
  external_id: string;
  movement_type: string | null;
  direction: InvestmentDirection | null;
  quantity: string | null;
  value: string | null;
  amount: string | null;
  occurred_at: string | null;
  trade_date: string | null;
}

const investmentTransactionKeys = [
  "id",
  "external_id",
  "movement_type",
  "direction",
  "quantity",
  "value",
  "amount",
  "occurred_at",
  "trade_date",
] as const;

export function parseInvestmentTransaction(value: unknown): InvestmentTransaction {
  const item = requiredRecord(value, investmentTransactionKeys, "Movimentação de investimento inválida.");
  if (
    typeof item.id !== "string" ||
    !isUuid(item.id) ||
    typeof item.external_id !== "string" ||
    item.external_id === "" ||
    !isNullableString(item.movement_type) ||
    !(item.direction === null || investmentDirections.includes(item.direction as InvestmentDirection))
  ) {
    throw new TypeError("Movimentação de investimento inválida.");
  }
  return {
    id: item.id,
    external_id: item.external_id,
    movement_type: item.movement_type,
    direction: item.direction as InvestmentDirection | null,
    quantity: nullableDecimal(item.quantity),
    value: nullableDecimal(item.value),
    amount: nullableDecimal(item.amount),
    occurred_at: nullableDate(item.occurred_at),
    trade_date: nullableDate(item.trade_date),
  };
}

export function parseInvestmentTransactionList(value: unknown): InvestmentTransaction[] {
  if (!Array.isArray(value)) {
    throw new TypeError("Lista de movimentações de investimento inválida.");
  }
  return value.map(parseInvestmentTransaction);
}

// Investment workspace ----------------------------------------------------
//
// These are deliberately separate from Investment above. Investment is the
// preserved, provider-imported detail API; the workspace below is the ledger
// used for custody accounts, goals, cash and manually recorded positions.

export const investmentAccountKinds = ["manual", "integrated", "synced"] as const;
export type InvestmentAccountKind = (typeof investmentAccountKinds)[number];

export interface InvestmentAccount {
  id: string;
  name: string;
  kind: InvestmentAccountKind;
  currency_code: "BRL";
  source_id: string | null;
  source_display_name: string | null;
  financial_account_id: string | null;
  active: boolean;
  cash_balance: string;
  created_at: string;
  updated_at: string;
}

export interface InvestmentAccountWrite {
  name: string;
  currency_code: "BRL";
  source_id?: string | null;
  financial_account_id?: string | null;
}

export interface InvestmentAccountUpdate {
  name?: string;
  active?: boolean;
  financial_account_id?: string | null;
}

const investmentAccountKeys = [
  "id",
  "name",
  "kind",
  "currency_code",
  "source_id",
  "source_display_name",
  "financial_account_id",
  "active",
  "cash_balance",
  "created_at",
  "updated_at",
] as const;

/**
 * An investment account id is not always a uuid: besides the accounts created
 * locally, the backend materialises one grouping row per provider connection
 * with the deterministic id `integrated:<data-source uuid>` (see
 * EnsureIntegratedAccounts and migration 00036/00037). The prefix exists so a
 * local grouping id can never be confused with an imported one — every other
 * investment id (positions, operations, portfolios, reconciliations) stays a
 * plain uuid.
 */
const integratedAccountIdPrefix = "integrated:";

function isInvestmentAccountId(value: unknown): value is string {
  if (typeof value !== "string") return false;
  return value.startsWith(integratedAccountIdPrefix)
    ? isUuid(value.slice(integratedAccountIdPrefix.length))
    : isUuid(value);
}

export function parseInvestmentAccount(value: unknown): InvestmentAccount {
  const item = requiredRecord(value, investmentAccountKeys, "Conta de investimento inválida.");
  if (
    !isInvestmentAccountId(item.id) ||
    typeof item.name !== "string" ||
    item.name.trim() === "" ||
    !investmentAccountKinds.includes(item.kind as InvestmentAccountKind) ||
    item.currency_code !== "BRL" ||
    !isNullableUuid(item.source_id) ||
    !isNullableString(item.source_display_name) ||
    !isNullableUuid(item.financial_account_id) ||
    typeof item.active !== "boolean" ||
    !isValidDate(item.created_at) ||
    !isValidDate(item.updated_at)
  ) {
    throw new TypeError("Conta de investimento inválida.");
  }
  return {
    id: item.id,
    name: item.name,
    kind: item.kind as InvestmentAccountKind,
    currency_code: "BRL",
    source_id: item.source_id as string | null,
    source_display_name: item.source_display_name as string | null,
    financial_account_id: item.financial_account_id as string | null,
    active: item.active,
    cash_balance: decimal(item.cash_balance),
    created_at: item.created_at as string,
    updated_at: item.updated_at as string,
  };
}

function parseInvestmentListEnvelope<T>(
  value: unknown,
  parser: (item: unknown) => T,
  message: string,
): T[] {
  const payload = requiredRecord(value, ["items"], message);
  if (!Array.isArray(payload.items)) throw new TypeError(message);
  return payload.items.map(parser);
}

export function parseInvestmentAccountList(value: unknown): InvestmentAccount[] {
  return parseInvestmentListEnvelope(value, parseInvestmentAccount, "Lista de contas de investimento inválida.");
}

export interface InvestmentAsset {
  id: string;
  name: string;
  ticker: string | null;
  asset_type: string;
  currency_code: string;
  created_at: string;
  updated_at: string;
}

export interface InvestmentAssetWrite {
  name: string;
  ticker: string | null;
  asset_type: string;
  currency_code: string;
}

const investmentAssetKeys = [
  "id",
  "name",
  "ticker",
  "asset_type",
  "currency_code",
  "created_at",
  "updated_at",
] as const;

export function parseInvestmentAsset(value: unknown): InvestmentAsset {
  const item = requiredRecord(value, investmentAssetKeys, "Ativo de investimento inválido.");
  if (
    typeof item.id !== "string" ||
    !isUuid(item.id) ||
    typeof item.name !== "string" ||
    item.name.trim() === "" ||
    !isNullableString(item.ticker) ||
    typeof item.asset_type !== "string" ||
    item.asset_type.trim() === "" ||
    typeof item.currency_code !== "string" ||
    !/^[A-Z]{3}$/.test(item.currency_code) ||
    !isValidDate(item.created_at) ||
    !isValidDate(item.updated_at)
  ) {
    throw new TypeError("Ativo de investimento inválido.");
  }
  return {
    id: item.id,
    name: item.name,
    ticker: item.ticker as string | null,
    asset_type: item.asset_type,
    currency_code: item.currency_code,
    created_at: item.created_at as string,
    updated_at: item.updated_at as string,
  };
}

export function parseInvestmentAssetList(value: unknown): InvestmentAsset[] {
  return parseInvestmentListEnvelope(value, parseInvestmentAsset, "Lista de ativos de investimento inválida.");
}

export interface InvestmentPortfolio {
  id: string;
  name: string;
  target_amount: string | null;
  target_date: string | null;
  notes: string | null;
  current_value: string;
  progress: string | null;
  created_at: string;
  updated_at: string;
}

export interface InvestmentPortfolioWrite {
  name: string;
  target_amount: string | null;
  target_date: string | null;
  notes: string | null;
}

const investmentPortfolioKeys = [
  "id",
  "name",
  "target_amount",
  "target_date",
  "notes",
  "current_value",
  "progress",
  "created_at",
  "updated_at",
] as const;

export function parseInvestmentPortfolio(value: unknown): InvestmentPortfolio {
  const item = requiredRecord(value, investmentPortfolioKeys, "Objetivo de investimento inválido.");
  if (
    typeof item.id !== "string" ||
    !isUuid(item.id) ||
    typeof item.name !== "string" ||
    item.name.trim() === "" ||
    !(item.target_date === null || (typeof item.target_date === "string" && dateOnlyPattern.test(item.target_date))) ||
    !isNullableString(item.notes) ||
    !isValidDate(item.created_at) ||
    !isValidDate(item.updated_at)
  ) {
    throw new TypeError("Objetivo de investimento inválido.");
  }
  return {
    id: item.id,
    name: item.name,
    target_amount: nullableDecimal(item.target_amount),
    target_date: item.target_date as string | null,
    notes: item.notes as string | null,
    current_value: decimal(item.current_value),
    progress: nullableDecimal(item.progress),
    created_at: item.created_at as string,
    updated_at: item.updated_at as string,
  };
}

export function parseInvestmentPortfolioList(value: unknown): InvestmentPortfolio[] {
  return parseInvestmentListEnvelope(value, parseInvestmentPortfolio, "Lista de objetivos inválida.");
}

export const investmentPositionSources = ["manual", "synced"] as const;
export type InvestmentPositionSource = (typeof investmentPositionSources)[number];
export const investmentValuationBases = ["manual_valuation", "cost_basis", "provider_balance"] as const;
export type InvestmentValuationBasis = (typeof investmentValuationBases)[number];

export interface InvestmentPosition {
  id: string;
  source: InvestmentPositionSource;
  account_id: string;
  asset_id: string | null;
  portfolio_id: string | null;
  name: string;
  ticker: string | null;
  asset_type: string;
  quantity: string;
  average_cost: string | null;
  current_value: string;
  current_unit_price: string | null;
  valued_on: string | null;
  currency_code: string;
  closed: boolean;
  linked_investment_id: string | null;
  notes: string | null;
  valuation_basis: InvestmentValuationBasis;
}

export interface InvestmentPositionWrite {
  account_id: string;
  asset_id?: string | null;
  name: string;
  ticker?: string | null;
  asset_type?: string;
  portfolio_id?: string | null;
  initial_quantity?: string;
  initial_unit_cost?: string;
  initial_value?: string;
  occurred_on?: string;
  notes?: string | null;
}

export interface InvestmentPositionUpdate {
  name: string;
  ticker: string | null;
  asset_type: string;
  portfolio_id: string | null;
  notes: string | null;
}

const investmentPositionKeys = [
  "id",
  "source",
  "account_id",
  "asset_id",
  "portfolio_id",
  "name",
  "ticker",
  "asset_type",
  "quantity",
  "average_cost",
  "current_value",
  "current_unit_price",
  "valued_on",
  "currency_code",
  "closed",
  "linked_investment_id",
  "notes",
  "valuation_basis",
] as const;

export function parseInvestmentPosition(value: unknown): InvestmentPosition {
  const item = requiredRecord(value, investmentPositionKeys, "Posição de investimento inválida.");
  if (
    typeof item.id !== "string" ||
    !isUuid(item.id) ||
    !investmentPositionSources.includes(item.source as InvestmentPositionSource) ||
    !isInvestmentAccountId(item.account_id) ||
    !isNullableUuid(item.asset_id) ||
    !isNullableUuid(item.portfolio_id) ||
    typeof item.name !== "string" ||
    item.name.trim() === "" ||
    !isNullableString(item.ticker) ||
    typeof item.asset_type !== "string" ||
    item.asset_type.trim() === "" ||
    !(item.valued_on === null || (typeof item.valued_on === "string" && dateOnlyPattern.test(item.valued_on))) ||
    typeof item.currency_code !== "string" || item.currency_code.length !== 3 ||
    typeof item.closed !== "boolean" ||
    !isNullableUuid(item.linked_investment_id) ||
    !isNullableString(item.notes) ||
    !investmentValuationBases.includes(item.valuation_basis as InvestmentValuationBasis)
  ) {
    throw new TypeError("Posição de investimento inválida.");
  }
  return {
    id: item.id,
    source: item.source as InvestmentPositionSource,
    account_id: item.account_id,
    asset_id: item.asset_id as string | null,
    portfolio_id: item.portfolio_id as string | null,
    name: item.name,
    ticker: item.ticker as string | null,
    asset_type: item.asset_type,
    quantity: decimal(item.quantity),
    average_cost: nullableDecimal(item.average_cost),
    current_value: decimal(item.current_value),
    current_unit_price: nullableDecimal(item.current_unit_price),
    valued_on: item.valued_on as string | null,
    currency_code: item.currency_code,
    closed: item.closed,
    linked_investment_id: item.linked_investment_id as string | null,
    notes: item.notes as string | null,
    valuation_basis: item.valuation_basis as InvestmentValuationBasis,
  };
}

export function parseInvestmentPositionList(value: unknown): InvestmentPosition[] {
  return parseInvestmentListEnvelope(value, parseInvestmentPosition, "Lista de posições inválida.");
}

export const investmentOperationKinds = [
  "initial_balance",
  "deposit",
  "withdrawal",
  "buy",
  "sell",
  "income",
  "fee",
  "tax",
  "valuation",
  "transfer_out",
  "transfer_in",
] as const;
export type InvestmentOperationKind = (typeof investmentOperationKinds)[number];

export const investmentOperationSources = ["manual", "synced"] as const;
export type InvestmentOperationSource = (typeof investmentOperationSources)[number];

export interface InvestmentOperation {
  id: string;
  account_id: string;
  position_id: string | null;
  transfer_id: string | null;
  kind: InvestmentOperationKind;
  occurred_on: string;
  amount: string;
  quantity: string | null;
  unit_price: string | null;
  fees: string | null;
  taxes: string | null;
  notes: string | null;
  source: InvestmentOperationSource;
  is_editable: boolean;
  created_at: string;
  updated_at: string;
}

export interface InvestmentOperationWrite {
  account_id: string;
  position_id: string | null;
  kind: InvestmentOperationKind;
  occurred_on: string;
  amount?: string;
  quantity?: string | null;
  unit_price?: string | null;
  fees?: string | null;
  taxes?: string | null;
  notes?: string | null;
}

const investmentOperationKeys = [
  "id",
  "account_id",
  "position_id",
  "transfer_id",
  "kind",
  "occurred_on",
  "amount",
  "quantity",
  "unit_price",
  "fees",
  "taxes",
  "notes",
  "source",
  "is_editable",
  "created_at",
  "updated_at",
] as const;

export function parseInvestmentOperation(value: unknown): InvestmentOperation {
  const item = requiredRecord(value, investmentOperationKeys, "Movimentação de carteira inválida.");
  if (
    typeof item.id !== "string" ||
    !isUuid(item.id) ||
    !isInvestmentAccountId(item.account_id) ||
    !isNullableUuid(item.position_id) ||
    !isNullableUuid(item.transfer_id) ||
    !investmentOperationKinds.includes(item.kind as InvestmentOperationKind) ||
    typeof item.occurred_on !== "string" ||
    !dateOnlyPattern.test(item.occurred_on) ||
    !isNullableString(item.notes) ||
    !investmentOperationSources.includes(item.source as InvestmentOperationSource) ||
    typeof item.is_editable !== "boolean" ||
    !isValidDate(item.created_at) ||
    !isValidDate(item.updated_at)
  ) {
    throw new TypeError("Movimentação de carteira inválida.");
  }
  return {
    id: item.id,
    account_id: item.account_id,
    position_id: item.position_id as string | null,
    transfer_id: item.transfer_id as string | null,
    kind: item.kind as InvestmentOperationKind,
    occurred_on: item.occurred_on,
    amount: decimal(item.amount),
    quantity: nullableDecimal(item.quantity),
    unit_price: nullableDecimal(item.unit_price),
    fees: nullableDecimal(item.fees),
    taxes: nullableDecimal(item.taxes),
    notes: item.notes as string | null,
    source: item.source as InvestmentOperationSource,
    is_editable: item.is_editable,
    created_at: item.created_at as string,
    updated_at: item.updated_at as string,
  };
}

export function parseInvestmentOperationList(value: unknown): InvestmentOperation[] {
  return parseInvestmentListEnvelope(value, parseInvestmentOperation, "Lista de movimentações de carteira inválida.");
}

export interface InvestmentReconciliation {
  id: string;
  operation_id: string;
  financial_transaction_id: string | null;
  financial_investment_transaction_id: string | null;
  amount: string;
  created_at: string;
}

export interface InvestmentReconciliationWrite {
  operation_id?: string | null;
  financial_transaction_id?: string | null;
  financial_investment_transaction_id?: string | null;
  amount: string;
}

const investmentReconciliationKeys = [
  "id",
  "operation_id",
  "financial_transaction_id",
  "financial_investment_transaction_id",
  "amount",
  "created_at",
] as const;

export function parseInvestmentReconciliation(value: unknown): InvestmentReconciliation {
  const item = requiredRecord(value, investmentReconciliationKeys, "Vínculo de investimento inválido.");
  if (
    typeof item.id !== "string" ||
    !isUuid(item.id) ||
    typeof item.operation_id !== "string" ||
    !isUuid(item.operation_id) ||
    !isNullableUuid(item.financial_transaction_id) ||
    !isNullableUuid(item.financial_investment_transaction_id) ||
    (item.financial_transaction_id === null && item.financial_investment_transaction_id === null) ||
    !isValidDate(item.created_at)
  ) {
    throw new TypeError("Vínculo de investimento inválido.");
  }
  return {
    id: item.id,
    operation_id: item.operation_id,
    financial_transaction_id: item.financial_transaction_id as string | null,
    financial_investment_transaction_id: item.financial_investment_transaction_id as string | null,
    amount: decimal(item.amount),
    created_at: item.created_at as string,
  };
}

export function parseInvestmentReconciliationList(value: unknown): InvestmentReconciliation[] {
  return parseInvestmentListEnvelope(value, parseInvestmentReconciliation, "Lista de vínculos inválida.");
}

export interface InvestmentSummaryPortfolio {
  portfolio_id: string | null;
  name: string;
  current_value: string;
  target_amount: string | null;
  progress: string | null;
}

export interface InvestmentSummaryAccount {
  account_id: string;
  name: string;
  kind: InvestmentAccountKind;
  current_value: string;
  cash_balance: string;
}

export interface InvestmentSummary {
  currency_code: "BRL";
  total_value: string;
  manual_value: string;
  synced_value: string;
  cash_balance: string;
  unrealized_gain: string;
  portfolios: InvestmentSummaryPortfolio[];
  accounts: InvestmentSummaryAccount[];
}

function parseInvestmentSummaryPortfolio(value: unknown): InvestmentSummaryPortfolio {
  const item = requiredRecord(
    value,
    ["portfolio_id", "name", "current_value", "target_amount", "progress"],
    "Resumo de objetivo inválido.",
  );
  if (!isNullableUuid(item.portfolio_id) || typeof item.name !== "string" || item.name.trim() === "") {
    throw new TypeError("Resumo de objetivo inválido.");
  }
  return {
    portfolio_id: item.portfolio_id as string | null,
    name: item.name,
    current_value: decimal(item.current_value),
    target_amount: nullableDecimal(item.target_amount),
    progress: nullableDecimal(item.progress),
  };
}

function parseInvestmentSummaryAccount(value: unknown): InvestmentSummaryAccount {
  const item = requiredRecord(
    value,
    ["account_id", "name", "kind", "current_value", "cash_balance"],
    "Resumo de conta de investimento inválido.",
  );
  if (
    !isInvestmentAccountId(item.account_id) ||
    typeof item.name !== "string" ||
    item.name.trim() === "" ||
    !investmentAccountKinds.includes(item.kind as InvestmentAccountKind)
  ) {
    throw new TypeError("Resumo de conta de investimento inválido.");
  }
  return {
    account_id: item.account_id,
    name: item.name,
    kind: item.kind as InvestmentAccountKind,
    current_value: decimal(item.current_value),
    cash_balance: decimal(item.cash_balance),
  };
}

export function parseInvestmentSummary(value: unknown): InvestmentSummary {
  const item = requiredRecord(
    value,
    [
      "currency_code",
      "total_value",
      "manual_value",
      "synced_value",
      "cash_balance",
      "unrealized_gain",
      "portfolios",
      "accounts",
    ],
    "Resumo de investimentos inválido.",
  );
  if (item.currency_code !== "BRL" || !Array.isArray(item.portfolios) || !Array.isArray(item.accounts)) {
    throw new TypeError("Resumo de investimentos inválido.");
  }
  return {
    currency_code: "BRL",
    total_value: decimal(item.total_value),
    manual_value: decimal(item.manual_value),
    synced_value: decimal(item.synced_value),
    cash_balance: decimal(item.cash_balance),
    unrealized_gain: decimal(item.unrealized_gain),
    portfolios: item.portfolios.map(parseInvestmentSummaryPortfolio),
    accounts: item.accounts.map(parseInvestmentSummaryAccount),
  };
}

const isClosingDay = (value: unknown): boolean =>
  value === null || (isCount(value) && value >= 1 && value <= 31);

const isClosingDaySource = (value: unknown): boolean =>
  value === null || accountClosingDaySources.includes(value as AccountClosingDaySource);

export type AccountType = "BANK" | "CREDIT";

const accountTypes = ["BANK", "CREDIT"] as const;

/** Where a card's closing day came from, in decreasing order of trust. */
export const accountClosingDaySources = ["manual", "informado", "estimado"] as const;
export type AccountClosingDaySource = (typeof accountClosingDaySources)[number];

export interface Account {
  id: string;
  external_id: string;
  source_display_name: string | null;
  institution: string | null;
  name: string | null;
  number: string | null;
  account_type: AccountType | null;
  account_subtype: string | null;
  balance: string | null;
  credit_limit: string | null;
  available_credit_limit: string | null;
  credit_usage_ratio: string | null;
  currency_code: string | null;
  balance_close_date: string | null;
  balance_due_date: string | null;
  /** Day of month the card's invoice closes, 1-31. */
  closing_day: number | null;
  closing_day_source: AccountClosingDaySource | null;
  provider_updated_at: string | null;
  updated_at: string | null;
}

const accountKeys = [
  "id",
  "external_id",
  "source_display_name",
  "institution",
  "name",
  "number",
  "account_type",
  "account_subtype",
  "balance",
  "credit_limit",
  "available_credit_limit",
  "credit_usage_ratio",
  "currency_code",
  "balance_close_date",
  "balance_due_date",
  "closing_day",
  "closing_day_source",
  "provider_updated_at",
  "updated_at",
] as const;

export function parseAccount(value: unknown): Account {
  const item = requiredRecord(value, accountKeys, "Conta inválida.");
  if (
    typeof item.id !== "string" ||
    !isUuid(item.id) ||
    typeof item.external_id !== "string" ||
    item.external_id === "" ||
    !(item.account_type === null || accountTypes.includes(item.account_type as AccountType)) ||
    !isClosingDay(item.closing_day) ||
    !isClosingDaySource(item.closing_day_source) ||
    (item.closing_day === null) !== (item.closing_day_source === null)
  ) {
    throw new TypeError("Conta inválida.");
  }
  return {
    id: item.id,
    external_id: item.external_id,
    source_display_name: nullableText(item.source_display_name),
    institution: nullableText(item.institution),
    name: nullableText(item.name),
    number: nullableText(item.number),
    account_type: item.account_type as AccountType | null,
    account_subtype: nullableText(item.account_subtype),
    balance: nullableDecimal(item.balance),
    credit_limit: nullableDecimal(item.credit_limit),
    available_credit_limit: nullableDecimal(item.available_credit_limit),
    credit_usage_ratio: nullableDecimal(item.credit_usage_ratio),
    currency_code: nullableText(item.currency_code),
    balance_close_date: nullableDate(item.balance_close_date),
    balance_due_date: nullableDate(item.balance_due_date),
    closing_day: item.closing_day as number | null,
    closing_day_source: item.closing_day_source as AccountClosingDaySource | null,
    provider_updated_at: nullableDate(item.provider_updated_at),
    updated_at: nullableDate(item.updated_at),
  };
}

export function parseAccountList(value: unknown): Account[] {
  if (!Array.isArray(value)) {
    throw new TypeError("Lista de contas inválida.");
  }
  return value.map(parseAccount);
}

/** One physical card spending against a credit account. */
export interface AccountCard {
  card_number: string;
  transaction_count: number;
  last_transaction_at: string | null;
}

const accountCardKeys = ["card_number", "transaction_count", "last_transaction_at"] as const;

export function parseAccountCard(value: unknown): AccountCard {
  const item = requiredRecord(value, accountCardKeys, "Cartão inválido.");
  if (
    typeof item.card_number !== "string" ||
    item.card_number === "" ||
    !isCount(item.transaction_count)
  ) {
    throw new TypeError("Cartão inválido.");
  }
  return {
    card_number: item.card_number,
    transaction_count: item.transaction_count,
    last_transaction_at: nullableDate(item.last_transaction_at),
  };
}

export function parseAccountCardList(value: unknown): AccountCard[] {
  if (!Array.isArray(value)) {
    throw new TypeError("Lista de cartões inválida.");
  }
  return value.map(parseAccountCard);
}

export interface AccountBill {
  id: string;
  external_id: string;
  due_date: string | null;
  closing_date: string | null;
  total_amount: string | null;
  currency_code: string | null;
  minimum_payment_amount: string | null;
}

const accountBillKeys = [
  "id",
  "external_id",
  "due_date",
  "closing_date",
  "total_amount",
  "currency_code",
  "minimum_payment_amount",
] as const;

export function parseAccountBill(value: unknown): AccountBill {
  const item = requiredRecord(value, accountBillKeys, "Fatura inválida.");
  if (
    typeof item.id !== "string" ||
    !isUuid(item.id) ||
    typeof item.external_id !== "string" ||
    item.external_id === ""
  ) {
    throw new TypeError("Fatura inválida.");
  }
  return {
    id: item.id,
    external_id: item.external_id,
    due_date: nullableDate(item.due_date),
    closing_date: nullableDate(item.closing_date),
    total_amount: nullableDecimal(item.total_amount),
    currency_code: nullableText(item.currency_code),
    minimum_payment_amount: nullableDecimal(item.minimum_payment_amount),
  };
}

export function parseAccountBillList(value: unknown): AccountBill[] {
  if (!Array.isArray(value)) {
    throw new TypeError("Lista de faturas inválida.");
  }
  return value.map(parseAccountBill);
}

export function parseProblem(value: unknown): Problem {
  if (
    !isRecord(value) ||
    typeof value.type !== "string" ||
    typeof value.title !== "string" ||
    !Number.isInteger(value.status) ||
    (value.detail !== undefined && typeof value.detail !== "string") ||
    (value.active_sync_run_id !== undefined &&
      value.active_sync_run_id !== null &&
      (typeof value.active_sync_run_id !== "string" || !isUuid(value.active_sync_run_id)))
  ) {
    throw new TypeError("Resposta de problema inválida.");
  }
  return {
    type: value.type,
    title: value.title,
    status: value.status as number,
    ...(value.detail === undefined ? {} : { detail: value.detail }),
    ...(value.active_sync_run_id === undefined
      ? {}
      : { active_sync_run_id: value.active_sync_run_id }),
  };
}

export const certaintyTiers = ["realizado", "confirmado", "projetado", "hipotetico"] as const;
export type CertaintyTier = (typeof certaintyTiers)[number];

export const timelineSourceKinds = [
  "real",
  "investment",
  "recorrente",
  "plano_pagamento",
  "cenario",
] as const;
export type TimelineSourceKind = (typeof timelineSourceKinds)[number];

export const investmentTransferKinds = ["deposit", "withdrawal"] as const;
export type InvestmentTransferKind = (typeof investmentTransferKinds)[number];

export interface TimelineEntry {
  date: string;
  description: string;
  amount: string;
  /**
   * The parcel of `amount` that counts as income/expense in the reports. It
   * equals `amount` for an ordinary entry; for a transfer to or from an
   * investment it is what is left after the transferred parcel is taken out.
   */
  reportable_amount: string;
  /** Absolute amount moved to/from investments — "0" when nothing was moved. */
  investment_transfer_amount: string;
  /** Direction of that movement, null when the entry is not a transfer. */
  investment_transfer_kind: InvestmentTransferKind | null;
  category_id: string | null;
  category_name: string;
  tier: CertaintyTier;
  source: TimelineSourceKind;
  source_ref_id: string;
  scenario_id: string | null;
}

export interface TimelineDayPoint {
  date: string;
  balance: string;
  inflow: string;
  outflow: string;
  lowest_tier: CertaintyTier;
}

export interface TimelineSeries {
  points: TimelineDayPoint[];
  entries: TimelineEntry[];
  starting_balance: string;
  lowest_balance: TimelineDayPoint;
  first_negative: string | null;
}

export interface MonthSummary {
  month: string;
  income: string;
  expense: string;
  result: string;
  /**
   * Money moved into and out of investments in the month. Kept apart from
   * income/expense on purpose: an aporte is not a despesa, a resgate is not
   * uma receita — both only move money between the caixa and the patrimônio.
   */
  investment_contributions: string;
  investment_withdrawals: string;
}

export interface CategoryImpact {
  category_id: string | null;
  category_name: string;
  amount: string;
  percentage: string;
}

export interface ScenarioImpact {
  scenario_id: string;
  scenario_name: string;
  delta: string;
}

export interface Comparison2 {
  current: string;
  previous: string;
  delta_percent: string;
}

export interface MonthAmount {
  month: string;
  amount: string;
}

export interface TimelineResponse {
  base: TimelineSeries;
  monthly_breakdown: MonthSummary[];
  category_breakdown: CategoryImpact[];
  simulation: TimelineSeries | null;
  scenario_impacts: ScenarioImpact[];
  month_over_month: Comparison2 | null;
  year_over_year: Comparison2 | null;
  category_evolution: MonthAmount[] | null;
}

export interface TimelineParams {
  /** Balance anchor — always today. Never the month being browsed. */
  referenceDate: string;
  from: string;
  to: string;
  /**
   * First day of the month the retrospective aggregations are about
   * (category breakdown, month-over-month, year-over-year). Defaults
   * server-side to referenceDate's month.
   */
  analysisMonth?: string;
  accountIds?: string[];
  categoryIds?: string[];
  cardNumbers?: string[];
  scenarioIds?: string[];
  yearOverYear?: boolean;
  categoryEvolutionId?: string | null;
  /**
   * Set false to drop monthly_breakdown, category_breakdown and
   * month_over_month from the response — for callers that only plot the
   * balance curve. Defaults to true server-side.
   */
  aggregations?: boolean;
}

function isNullableCategoryId(value: unknown): value is string | null {
  return value === null || (typeof value === "string" && isUuid(value));
}

function parseTimelineEntry(value: unknown): TimelineEntry {
  const entry = requiredRecordWithOptionalKeys(
    value,
    [
      "date",
      "description",
      "amount",
      "category_id",
      "category_name",
      "tier",
      "source",
      "source_ref_id",
      "scenario_id",
    ],
    ["reportable_amount", "investment_transfer_amount", "investment_transfer_kind"],
    "Entrada do relatório inválida.",
  );
  if (
    !dateOnlyPattern.test(entry.date as string) ||
    typeof entry.description !== "string" ||
    !isNullableCategoryId(entry.category_id) ||
    typeof entry.category_name !== "string" ||
    !certaintyTiers.includes(entry.tier as CertaintyTier) ||
    !timelineSourceKinds.includes(entry.source as TimelineSourceKind) ||
    typeof entry.source_ref_id !== "string" ||
    entry.source_ref_id === "" ||
    !isNullableCategoryId(entry.scenario_id) ||
    !(
      entry.investment_transfer_kind === undefined ||
      entry.investment_transfer_kind === null ||
      investmentTransferKinds.includes(entry.investment_transfer_kind as InvestmentTransferKind)
    )
  ) {
    throw new TypeError("Entrada do relatório inválida.");
  }
  const amount = decimal(entry.amount);
  return {
    date: entry.date as string,
    description: entry.description,
    amount,
    // Servers and cached responses that predate the investment allocation
    // report nothing transferred, so the whole entry is reportable.
    reportable_amount:
      entry.reportable_amount === undefined ? amount : decimal(entry.reportable_amount),
    investment_transfer_amount:
      entry.investment_transfer_amount === undefined ? "0" : decimal(entry.investment_transfer_amount),
    investment_transfer_kind:
      entry.investment_transfer_kind === undefined
        ? null
        : (entry.investment_transfer_kind as InvestmentTransferKind | null),
    category_id: entry.category_id as string | null,
    category_name: entry.category_name,
    tier: entry.tier as CertaintyTier,
    source: entry.source as TimelineSourceKind,
    source_ref_id: entry.source_ref_id,
    scenario_id: entry.scenario_id as string | null,
  };
}

function parseTimelineDayPoint(value: unknown): TimelineDayPoint {
  const point = requiredRecord(
    value,
    ["date", "balance", "inflow", "outflow", "lowest_tier"],
    "Ponto do relatório inválido.",
  );
  if (
    !dateOnlyPattern.test(point.date as string) ||
    !certaintyTiers.includes(point.lowest_tier as CertaintyTier)
  ) {
    throw new TypeError("Ponto do relatório inválido.");
  }
  return {
    date: point.date as string,
    balance: decimal(point.balance),
    inflow: decimal(point.inflow),
    outflow: decimal(point.outflow),
    lowest_tier: point.lowest_tier as CertaintyTier,
  };
}

function parseTimelineSeries(value: unknown): TimelineSeries {
  const series = requiredRecord(
    value,
    ["points", "entries", "starting_balance", "lowest_balance", "first_negative"],
    "Série do relatório inválida.",
  );
  if (
    !Array.isArray(series.points) ||
    !Array.isArray(series.entries) ||
    !(series.first_negative === null || dateOnlyPattern.test(series.first_negative as string))
  ) {
    throw new TypeError("Série do relatório inválida.");
  }
  return {
    points: series.points.map(parseTimelineDayPoint),
    entries: series.entries.map(parseTimelineEntry),
    starting_balance: decimal(series.starting_balance),
    lowest_balance: parseTimelineDayPoint(series.lowest_balance),
    first_negative: series.first_negative as string | null,
  };
}

function parseMonthSummary(value: unknown): MonthSummary {
  const summary = requiredRecordWithOptionalKeys(
    value,
    ["month", "income", "expense", "result"],
    ["investment_contributions", "investment_withdrawals"],
    "Resumo mensal inválido.",
  );
  if (!dateOnlyPattern.test(summary.month as string)) {
    throw new TypeError("Resumo mensal inválido.");
  }
  return {
    month: summary.month as string,
    income: decimal(summary.income),
    expense: decimal(summary.expense),
    result: decimal(summary.result),
    // Absent on servers and fixtures that predate the investment reading:
    // nothing was moved, not "unknown".
    investment_contributions:
      summary.investment_contributions === undefined ? "0" : decimal(summary.investment_contributions),
    investment_withdrawals:
      summary.investment_withdrawals === undefined ? "0" : decimal(summary.investment_withdrawals),
  };
}

function parseCategoryImpact(value: unknown): CategoryImpact {
  const impact = requiredRecord(
    value,
    ["category_id", "category_name", "amount", "percentage"],
    "Impacto de categoria inválido.",
  );
  if (!isNullableCategoryId(impact.category_id) || typeof impact.category_name !== "string") {
    throw new TypeError("Impacto de categoria inválido.");
  }
  return {
    category_id: impact.category_id as string | null,
    category_name: impact.category_name,
    amount: decimal(impact.amount),
    percentage: decimal(impact.percentage),
  };
}

function parseScenarioImpact(value: unknown): ScenarioImpact {
  const impact = requiredRecord(
    value,
    ["scenario_id", "scenario_name", "delta"],
    "Impacto de cenário inválido.",
  );
  if (
    typeof impact.scenario_id !== "string" ||
    !isUuid(impact.scenario_id) ||
    typeof impact.scenario_name !== "string"
  ) {
    throw new TypeError("Impacto de cenário inválido.");
  }
  return {
    scenario_id: impact.scenario_id,
    scenario_name: impact.scenario_name,
    delta: decimal(impact.delta),
  };
}

function parseComparison2(value: unknown): Comparison2 {
  const comparison = requiredRecord(
    value,
    ["current", "previous", "delta_percent"],
    "Comparação inválida.",
  );
  return {
    current: decimal(comparison.current),
    previous: decimal(comparison.previous),
    delta_percent: decimal(comparison.delta_percent),
  };
}

function parseMonthAmount(value: unknown): MonthAmount {
  const item = requiredRecord(value, ["month", "amount"], "Evolução de categoria inválida.");
  if (!dateOnlyPattern.test(item.month as string)) {
    throw new TypeError("Evolução de categoria inválida.");
  }
  return { month: item.month as string, amount: decimal(item.amount) };
}

export function parseTimelineResponse(value: unknown): TimelineResponse {
  const response = requiredRecord(
    value,
    [
      "base",
      "monthly_breakdown",
      "category_breakdown",
      "simulation",
      "scenario_impacts",
      "month_over_month",
      "year_over_year",
      "category_evolution",
    ],
    "Resposta do relatório financeiro inválida.",
  );
  if (
    !Array.isArray(response.monthly_breakdown) ||
    !Array.isArray(response.category_breakdown) ||
    !Array.isArray(response.scenario_impacts) ||
    !(response.category_evolution === null || Array.isArray(response.category_evolution))
  ) {
    throw new TypeError("Resposta do relatório financeiro inválida.");
  }
  return {
    base: parseTimelineSeries(response.base),
    monthly_breakdown: response.monthly_breakdown.map(parseMonthSummary),
    category_breakdown: response.category_breakdown.map(parseCategoryImpact),
    simulation: response.simulation === null ? null : parseTimelineSeries(response.simulation),
    scenario_impacts: response.scenario_impacts.map(parseScenarioImpact),
    month_over_month: response.month_over_month === null ? null : parseComparison2(response.month_over_month),
    year_over_year: response.year_over_year === null ? null : parseComparison2(response.year_over_year),
    category_evolution:
      response.category_evolution === null ? null : (response.category_evolution as unknown[]).map(parseMonthAmount),
  };
}

/**
 * The widest window the balance curve can cover, from internal/timeline's
 * LoadDataRange: `from` is the oldest transaction on record, `to` the last
 * planned installment (or this month's end when there is none).
 */
export interface TimelineDataRange {
  from: string;
  to: string;
}

export function parseTimelineDataRange(value: unknown): TimelineDataRange {
  const range = requiredRecord(value, ["from", "to"], "Intervalo do relatório inválido.");
  if (!dateOnlyPattern.test(range.from as string) || !dateOnlyPattern.test(range.to as string)) {
    throw new TypeError("Intervalo do relatório inválido.");
  }
  return { from: range.from as string, to: range.to as string };
}

// NetWorthBreakdown mirrors internal/httpapi's netWorthBreakdownDTO: the
// four underlying balances plus the three totals derived from them
// (assets/liabilities/net worth), all pre-computed server-side so the
// frontend never has to re-derive money math from parts. Receivables
// (money owed to the user) never enter this model — see internal/networth's
// Breakdown doc comment for why.
export interface NetWorthBreakdown {
  cash_balance: string;
  investment_balance: string;
  credit_card_balance: string;
  payables_debt: string;
  total_assets: string;
  total_liabilities: string;
  net_worth: string;
}

export interface NetWorthSnapshot extends NetWorthBreakdown {
  captured_at: string;
  // Mirrors internal/httpapi's netWorthSnapshotDTO.IsBackfilled: true when
  // this row was reconstructed from past transactions rather than captured
  // live, in which case investment_balance is always "0" — investment
  // history can't be reconstructed (see internal/networth's Backfill doc
  // comment).
  is_backfilled: boolean;
}

export interface NetWorthSeries {
  series: NetWorthSnapshot[];
  latest: NetWorthSnapshot;
}

function parseNetWorthBreakdown(value: Record<string, unknown>): NetWorthBreakdown {
  return {
    cash_balance: decimal(value.cash_balance),
    investment_balance: decimal(value.investment_balance),
    credit_card_balance: decimal(value.credit_card_balance),
    payables_debt: decimal(value.payables_debt),
    total_assets: decimal(value.total_assets),
    total_liabilities: decimal(value.total_liabilities),
    net_worth: decimal(value.net_worth),
  };
}

const netWorthBreakdownKeys = [
  "cash_balance",
  "investment_balance",
  "credit_card_balance",
  "payables_debt",
  "total_assets",
  "total_liabilities",
  "net_worth",
] as const;

export function parseNetWorthSnapshot(value: unknown): NetWorthSnapshot {
  const snapshot = requiredRecord(
    value,
    ["captured_at", "is_backfilled", ...netWorthBreakdownKeys],
    "Patrimônio líquido inválido.",
  );
  if (!isValidDate(snapshot.captured_at)) {
    throw new TypeError("Patrimônio líquido inválido.");
  }
  if (typeof snapshot.is_backfilled !== "boolean") {
    throw new TypeError("Patrimônio líquido inválido.");
  }
  return {
    captured_at: snapshot.captured_at,
    is_backfilled: snapshot.is_backfilled,
    ...parseNetWorthBreakdown(snapshot),
  };
}

export function parseNetWorthSeries(value: unknown): NetWorthSeries {
  const response = requiredRecord(value, ["series", "latest"], "Patrimônio líquido inválido.");
  if (!Array.isArray(response.series)) {
    throw new TypeError("Patrimônio líquido inválido.");
  }
  return {
    series: response.series.map(parseNetWorthSnapshot),
    latest: parseNetWorthSnapshot(response.latest),
  };
}

export type CategoryDirection = "inflow" | "outflow";

export type CategoryBreakdown = Omit<SpendingByCategory, "month"> & {
  classification: CategoryDirection;
} & ({ month: string } | { date_from: string | null; date_to: string | null });

export function parseCategoryBreakdown(value: unknown): CategoryBreakdown {
  const periodKeys = isRecord(value) && "month" in value ? ["month"] : ["date_from", "date_to"];
  const item = requiredRecord(value, [...periodKeys, "currency_code", "total", "items", "classification"], "Distribuição por categoria inválida.");
  if (item.classification !== "inflow" && item.classification !== "outflow") {
    throw new TypeError("Distribuição por categoria inválida.");
  }
  if ("month" in item) return { ...parseSpendingByCategory({ month: item.month, currency_code: item.currency_code, total: item.total, items: item.items }), classification: item.classification };
  const validDate = (date: unknown): date is string => {
    if (typeof date !== "string" || !/^\d{4}-\d{2}-\d{2}$/.test(date) || date < "0001-01-01") return false;
    const parsed = new Date(`${date}T00:00:00Z`);
    return Number.isFinite(parsed.getTime()) && parsed.toISOString().slice(0, 10) === date;
  };
  const { date_from: from, date_to: to } = item;
  if (!(from === null && to === null) && !(validDate(from) && validDate(to) && from <= to)) {
    throw new TypeError("Distribuição por categoria inválida.");
  }
  if (typeof item.currency_code !== "string" || !item.currency_code || !Array.isArray(item.items)) {
    throw new TypeError("Distribuição por categoria inválida.");
  }
  return {
    date_from: from as string | null, date_to: to as string | null,
    classification: item.classification, currency_code: item.currency_code,
    total: decimal(item.total), items: item.items.map(parseCategorySpendingItem),
  };
}
