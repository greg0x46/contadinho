import type { ReconciliationOrigin, ReconciliationStatus } from "../api/contracts";

export const reconciliationStatusLabel: Record<ReconciliationStatus, string> = {
  reconciled: "Conciliada",
  unreconciled: "Não conciliada",
  detached: "Desconciliada",
};

// antd Tag colors: green for the settled case, a dashed default for "nothing
// happened yet", and a warning tone for a state the user deliberately put the
// occurrence into and may want to undo.
export const reconciliationStatusColor: Record<ReconciliationStatus, string | undefined> = {
  reconciled: "success",
  unreconciled: undefined,
  detached: "warning",
};

export const reconciliationOriginLabel: Record<ReconciliationOrigin, string> = {
  rule: "automática",
  manual: "manual",
};

/**
 * The status a user reads on an occurrence row, with the origin folded in —
 * "Conciliada (automática)" vs "Conciliada (manual)" is the distinction that
 * tells them whether an automation rule or their own choice is behind it,
 * and therefore what desconciliar will undo.
 */
export function reconciliationLabel(
  status: ReconciliationStatus,
  origin: ReconciliationOrigin | null,
): string {
  const base = reconciliationStatusLabel[status];
  if (status !== "reconciled" || origin === null) return base;
  return `${base} (${reconciliationOriginLabel[origin]})`;
}
