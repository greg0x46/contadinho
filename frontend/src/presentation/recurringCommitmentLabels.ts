import type { RecurringCommitmentCadence, RecurringCommitmentKind } from "../api/contracts";

export const recurringCommitmentKindLabel: Record<RecurringCommitmentKind, string> = {
  income: "Receita",
  expense: "Despesa",
};

export const recurringCommitmentCadenceLabel: Record<RecurringCommitmentCadence, string> = {
  monthly: "Mensal",
  annual: "Anual",
};

const monthNames = [
  "Janeiro",
  "Fevereiro",
  "Março",
  "Abril",
  "Maio",
  "Junho",
  "Julho",
  "Agosto",
  "Setembro",
  "Outubro",
  "Novembro",
  "Dezembro",
];

export function monthOfYearLabel(monthOfYear: number): string {
  return monthNames[monthOfYear - 1] ?? String(monthOfYear);
}

export function recurrenceScheduleLabel(
  cadence: RecurringCommitmentCadence,
  dayOfMonth: number,
  monthOfYear: number | null,
): string {
  if (cadence === "annual" && monthOfYear !== null) {
    return `Dia ${dayOfMonth} de ${monthOfYearLabel(monthOfYear)}`;
  }
  return `Todo dia ${dayOfMonth}`;
}
