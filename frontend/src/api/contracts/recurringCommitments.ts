import { parseEligibleTransaction } from "./payables";
import type { EligibleTransaction } from "./payables";
import { dateOnlyPattern, decimal, isUuid, isValidDate, requiredRecord } from "./shared";

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
    "Recorrência inválida.",
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
    throw new TypeError("Recorrência inválida.");
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
    throw new TypeError("Lista de recorrências inválida.");
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
