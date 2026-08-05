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

export const categoryKinds = ["expense", "income", "transfer"] as const;
export type CategoryKind = (typeof categoryKinds)[number];
export const categoryOrigins = ["manual", "automatic"] as const;
export type CategoryOrigin = (typeof categoryOrigins)[number];

export interface TransactionFilters {
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
  origin: CategoryOrigin;
  changed_at: string;
}

export interface TransactionItem {
  id: string;
  external_id: string;
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
      | "unclassified"
      | "ineligible_status"
      | "missing_money_pair"
      | "zero_value"
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
    ["category_id", "category_name", "amount", "source"],
    "Gasto por categoria inválido.",
  );
  if (item.category_id !== null && typeof item.category_id !== "string") {
    throw new TypeError("Gasto por categoria inválido.");
  }
  if (typeof item.category_name !== "string" || item.category_name === "") {
    throw new TypeError("Gasto por categoria inválido.");
  }
  if (!categorySpendingSources.includes(item.source as CategorySpendingSource)) {
    throw new TypeError("Gasto por categoria inválido.");
  }
  return {
    category_id: item.category_id,
    category_name: item.category_name,
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
    ["id", "name", "kind", "is_active", "origin", "changed_at"],
    "Categoria interna inválida.",
  );
  if (
    typeof category.id !== "string" ||
    !isUuid(category.id) ||
    typeof category.name !== "string" ||
    category.name === "" ||
    !categoryKinds.includes(category.kind as CategoryKind) ||
    typeof category.is_active !== "boolean" ||
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

function parseTransactionItem(value: unknown): TransactionItem {
  const item = requiredRecord(value, [
    "id",
    "external_id",
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
  ]);
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
    "unclassified",
    "ineligible_status",
    "missing_money_pair",
    "zero_value",
  ];
  if (
    typeof item.id !== "string" ||
    !isUuid(item.id) ||
    typeof item.external_id !== "string" ||
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

export const automationConditionFields = ["description", "card", "account"] as const;
export type AutomationConditionField = (typeof automationConditionFields)[number];
export const automationConditionOperators = ["contains", "equals"] as const;
export type AutomationConditionOperator = (typeof automationConditionOperators)[number];
export const automationLogicOperators = ["and", "or"] as const;
export type AutomationLogicOperator = (typeof automationLogicOperators)[number];

export interface AutomationRuleCondition {
  field: AutomationConditionField;
  operator: AutomationConditionOperator;
  value: string;
}

export interface AutomationRule {
  id: string;
  name: string;
  is_active: boolean;
  logic_operator: AutomationLogicOperator;
  conditions: AutomationRuleCondition[];
  created_at: string;
  updated_at: string;
}

export interface AutomationRuleWrite {
  name: string;
  is_active: boolean;
  logic_operator: AutomationLogicOperator;
  conditions: AutomationRuleCondition[];
  apply_retroactively: boolean;
}

export interface RetroactiveApplyResult {
  matched: number;
  ignored: number;
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

function parseAutomationRuleCondition(value: unknown): AutomationRuleCondition {
  const condition = requiredRecord(value, ["field", "operator", "value"], "Condição inválida.");
  if (
    !automationConditionFields.includes(condition.field as AutomationConditionField) ||
    !automationConditionOperators.includes(condition.operator as AutomationConditionOperator) ||
    typeof condition.value !== "string" ||
    condition.value === ""
  ) {
    throw new TypeError("Condição inválida.");
  }
  return {
    field: condition.field as AutomationConditionField,
    operator: condition.operator as AutomationConditionOperator,
    value: condition.value,
  };
}

export function parseAutomationRule(value: unknown): AutomationRule {
  const rule = requiredRecord(
    value,
    ["id", "name", "is_active", "logic_operator", "conditions", "created_at", "updated_at"],
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
    !isValidDate(rule.created_at) ||
    !isValidDate(rule.updated_at)
  ) {
    throw new TypeError("Regra de automação inválida.");
  }
  return {
    id: rule.id,
    name: rule.name,
    is_active: rule.is_active,
    logic_operator: rule.logic_operator as AutomationLogicOperator,
    conditions: rule.conditions.map(parseAutomationRuleCondition),
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
  const result = requiredRecord(value, ["matched", "ignored"], "Resultado retroativo inválido.");
  if (!isCount(result.matched) || !isCount(result.ignored)) {
    throw new TypeError("Resultado retroativo inválido.");
  }
  return { matched: result.matched, ignored: result.ignored };
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

export interface Category {
  id: string;
  name: string;
  kind: CategoryKind;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface CategoryCreate {
  name: string;
  kind: CategoryKind;
}

export interface CategoryUpdate {
  name?: string;
  is_active?: boolean;
}

export function parseCategory(value: unknown): Category {
  const category = requiredRecord(
    value,
    ["id", "name", "kind", "is_active", "created_at", "updated_at"],
    "Categoria inválida.",
  );
  if (
    typeof category.id !== "string" ||
    !isUuid(category.id) ||
    typeof category.name !== "string" ||
    category.name === "" ||
    !categoryKinds.includes(category.kind as CategoryKind) ||
    typeof category.is_active !== "boolean" ||
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
    const option = requiredRecord(value, ["id", "name", "kind", "is_active"]);
    if (
      typeof option.id !== "string" ||
      !isUuid(option.id) ||
      typeof option.name !== "string" ||
      option.name === "" ||
      !categoryKinds.includes(option.kind as CategoryKind) ||
      typeof option.is_active !== "boolean"
    ) {
      throw new TypeError("Opção de categoria inválida.");
    }
    return {
      id: option.id,
      name: option.name,
      kind: option.kind as CategoryKind,
      is_active: option.is_active,
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

export const debtStatuses = ["open", "settled"] as const;
export type DebtStatus = (typeof debtStatuses)[number];

export interface Debt {
  id: string;
  name: string;
  total_amount: string;
  starting_paid_amount: string;
  paid_amount: string;
  remaining_amount: string;
  status: DebtStatus;
  link_count: number;
  created_at: string;
  updated_at: string;
}

export interface DebtLinkedTransaction {
  id: string;
  transaction_id: string;
  occurred_at: string | null;
  description: string | null;
  linked_amount: string;
  current_amount: string;
  linked_at: string;
}

export interface DebtDetail extends Debt {
  links: DebtLinkedTransaction[];
}

export interface EligibleTransaction {
  id: string;
  occurred_at: string | null;
  description: string | null;
  account_name: string | null;
  effective_money: { value: string; currency_code: string };
}

export interface DebtCreate {
  name: string;
  total_amount: number;
  initial_remaining_amount?: number | null;
}

export interface DebtUpdate {
  name: string;
  total_amount: number;
}

export interface DebtLinkCreate {
  transaction_id: string;
}

export interface DebtLink {
  id: string;
  transaction_id: string;
  linked_amount: string;
  linked_at: string;
}

const debtKeys = [
  "id",
  "name",
  "total_amount",
  "starting_paid_amount",
  "paid_amount",
  "remaining_amount",
  "status",
  "link_count",
  "created_at",
  "updated_at",
] as const;

function debtFieldsFrom(debt: Record<string, unknown>): Debt {
  if (
    typeof debt.id !== "string" ||
    !isUuid(debt.id) ||
    typeof debt.name !== "string" ||
    debt.name === "" ||
    !debtStatuses.includes(debt.status as DebtStatus) ||
    !isCount(debt.link_count) ||
    !isValidDate(debt.created_at) ||
    !isValidDate(debt.updated_at)
  ) {
    throw new TypeError("Dívida inválida.");
  }
  return {
    id: debt.id,
    name: debt.name,
    total_amount: decimal(debt.total_amount),
    starting_paid_amount: decimal(debt.starting_paid_amount),
    paid_amount: decimal(debt.paid_amount),
    remaining_amount: decimal(debt.remaining_amount),
    status: debt.status as DebtStatus,
    link_count: debt.link_count,
    created_at: debt.created_at,
    updated_at: debt.updated_at,
  };
}

export function parseDebt(value: unknown): Debt {
  return debtFieldsFrom(requiredRecord(value, debtKeys, "Dívida inválida."));
}

export function parseDebtList(value: unknown): Debt[] {
  if (!Array.isArray(value)) {
    throw new TypeError("Lista de dívidas inválida.");
  }
  return value.map(parseDebt);
}

export interface DebtTotalOwed {
  remaining_debts_total: string;
  future_installments_total: string;
  total_owed: string;
  currency_code: string;
}

export function parseDebtTotalOwed(value: unknown): DebtTotalOwed {
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

function parseDebtLinkedTransaction(value: unknown): DebtLinkedTransaction {
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
    "Vínculo de dívida inválido.",
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
    throw new TypeError("Vínculo de dívida inválido.");
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

export function parseDebtDetail(value: unknown): DebtDetail {
  const detail = requiredRecord(value, [...debtKeys, "links"], "Detalhe de dívida inválido.");
  if (!Array.isArray(detail.links)) {
    throw new TypeError("Detalhe de dívida inválido.");
  }
  return { ...debtFieldsFrom(detail), links: detail.links.map(parseDebtLinkedTransaction) };
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

export const receivableStatuses = ["open", "settled"] as const;
export type ReceivableStatus = (typeof receivableStatuses)[number];

export interface Receivable {
  id: string;
  name: string;
  total_amount: string;
  starting_received_amount: string;
  received_amount: string;
  remaining_amount: string;
  status: ReceivableStatus;
  link_count: number;
  created_at: string;
  updated_at: string;
}

export interface ReceivableLinkedTransaction {
  id: string;
  transaction_id: string;
  occurred_at: string | null;
  description: string | null;
  linked_amount: string;
  current_amount: string;
  linked_at: string;
}

export interface ReceivableDetail extends Receivable {
  links: ReceivableLinkedTransaction[];
}

export interface ReceivableCreate {
  name: string;
  total_amount: number;
  initial_remaining_amount?: number | null;
}

export interface ReceivableUpdate {
  name: string;
  total_amount: number;
}

export interface ReceivableLinkCreate {
  transaction_id: string;
}

export interface ReceivableLink {
  id: string;
  transaction_id: string;
  linked_amount: string;
  linked_at: string;
}

const receivableKeys = [
  "id",
  "name",
  "total_amount",
  "starting_received_amount",
  "received_amount",
  "remaining_amount",
  "status",
  "link_count",
  "created_at",
  "updated_at",
] as const;

function receivableFieldsFrom(receivable: Record<string, unknown>): Receivable {
  if (
    typeof receivable.id !== "string" ||
    !isUuid(receivable.id) ||
    typeof receivable.name !== "string" ||
    receivable.name === "" ||
    !receivableStatuses.includes(receivable.status as ReceivableStatus) ||
    !isCount(receivable.link_count) ||
    !isValidDate(receivable.created_at) ||
    !isValidDate(receivable.updated_at)
  ) {
    throw new TypeError("Conta a receber inválida.");
  }
  return {
    id: receivable.id,
    name: receivable.name,
    total_amount: decimal(receivable.total_amount),
    starting_received_amount: decimal(receivable.starting_received_amount),
    received_amount: decimal(receivable.received_amount),
    remaining_amount: decimal(receivable.remaining_amount),
    status: receivable.status as ReceivableStatus,
    link_count: receivable.link_count,
    created_at: receivable.created_at,
    updated_at: receivable.updated_at,
  };
}

export function parseReceivable(value: unknown): Receivable {
  return receivableFieldsFrom(requiredRecord(value, receivableKeys, "Conta a receber inválida."));
}

export function parseReceivableList(value: unknown): Receivable[] {
  if (!Array.isArray(value)) {
    throw new TypeError("Lista de contas a receber inválida.");
  }
  return value.map(parseReceivable);
}

export interface ReceivableTotalToReceive {
  remaining_receivables_total: string;
  total_to_receive: string;
  currency_code: string;
}

export function parseReceivableTotalToReceive(value: unknown): ReceivableTotalToReceive {
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

function parseReceivableLinkedTransaction(value: unknown): ReceivableLinkedTransaction {
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
    "Vínculo de conta a receber inválido.",
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
    throw new TypeError("Vínculo de conta a receber inválido.");
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

export function parseReceivableDetail(value: unknown): ReceivableDetail {
  const detail = requiredRecord(
    value,
    [...receivableKeys, "links"],
    "Detalhe de conta a receber inválido.",
  );
  if (!Array.isArray(detail.links)) {
    throw new TypeError("Detalhe de conta a receber inválido.");
  }
  return {
    ...receivableFieldsFrom(detail),
    links: detail.links.map(parseReceivableLinkedTransaction),
  };
}

export function parseReceivableLink(value: unknown): ReceivableLink {
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

export function parseDebtLink(value: unknown): DebtLink {
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

export const scenarioKinds = ["debt_plan", "receivable_plan"] as const;
export type ScenarioKind = (typeof scenarioKinds)[number];

export interface Scenario {
  id: string;
  kind: ScenarioKind;
  name: string;
  debt_id: string | null;
  receivable_id: string | null;
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
  debt_link_id: string | null;
  receivable_link_id: string | null;
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

// RealizationWrite carries exactly one of debt_link_id/receivable_link_id,
// matching the scenario's own kind — never both, never neither.
export type RealizationWrite =
  | { debt_link_id: string; receivable_link_id?: undefined; allocated_amount: number }
  | { debt_link_id?: undefined; receivable_link_id: string; allocated_amount: number };

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

const scenarioKeys = [
  "id",
  "kind",
  "name",
  "debt_id",
  "receivable_id",
  "created_at",
  "updated_at",
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
    !isNullableUuid(scenario.debt_id) ||
    !isNullableUuid(scenario.receivable_id) ||
    !isValidDate(scenario.created_at) ||
    !isValidDate(scenario.updated_at)
  ) {
    throw new TypeError("Cenário inválido.");
  }
  return {
    id: scenario.id,
    kind: scenario.kind as ScenarioKind,
    name: scenario.name,
    debt_id: scenario.debt_id as string | null,
    receivable_id: scenario.receivable_id as string | null,
    created_at: scenario.created_at,
    updated_at: scenario.updated_at,
  };
}

export function parseScenario(value: unknown): Scenario {
  return scenarioFieldsFrom(requiredRecord(value, scenarioKeys, "Cenário inválido."));
}

export function parseScenarioList(value: unknown): Scenario[] {
  if (!Array.isArray(value)) {
    throw new TypeError("Lista de cenários inválida.");
  }
  return value.map(parseScenario);
}

const realizationKeys = [
  "id",
  "debt_link_id",
  "receivable_link_id",
  "allocated_amount",
  "created_at",
] as const;

function parseRealization(value: unknown): Realization {
  const item = requiredRecord(value, realizationKeys, "Alocação inválida.");
  if (
    typeof item.id !== "string" ||
    !isUuid(item.id) ||
    !isNullableUuid(item.debt_link_id) ||
    !isNullableUuid(item.receivable_link_id) ||
    !isValidDate(item.created_at)
  ) {
    throw new TypeError("Alocação inválida.");
  }
  return {
    id: item.id,
    debt_link_id: item.debt_link_id as string | null,
    receivable_link_id: item.receivable_link_id as string | null,
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
  const detail = requiredRecord(
    value,
    [...scenarioKeys, "transactions", "accumulated_deviation"],
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
    (item.yield_source === null) !== (item.yield_value === null)
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
  };
}

export function parseInvestmentList(value: unknown): Investment[] {
  if (!Array.isArray(value)) {
    throw new TypeError("Lista de investimentos inválida.");
  }
  return value.map(parseInvestment);
}

export interface InvestmentTransaction {
  id: string;
  external_id: string;
  movement_type: string | null;
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
    !isNullableString(item.movement_type)
  ) {
    throw new TypeError("Movimentação de investimento inválida.");
  }
  return {
    id: item.id,
    external_id: item.external_id,
    movement_type: item.movement_type,
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

export interface SetupStatus {
  configured: boolean;
  unlocked: boolean;
}

export interface SetupRequest {
  password: string;
  pluggy_client_id: string;
  pluggy_client_secret: string;
  pluggy_item_id: string;
}

export interface UnlockRequest {
  password: string;
}

export function parseSetupStatus(value: unknown): SetupStatus {
  const status = requiredRecord(value, ["configured", "unlocked"], "Estado de configuração inválido.");
  if (typeof status.configured !== "boolean" || typeof status.unlocked !== "boolean") {
    throw new TypeError("Estado de configuração inválido.");
  }
  return { configured: status.configured, unlocked: status.unlocked };
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
