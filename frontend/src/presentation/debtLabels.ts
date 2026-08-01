import type { DebtStatus } from "../api/contracts";

export const debtStatusLabel: Record<DebtStatus, string> = {
  open: "Aberta",
  settled: "Quitada",
};

export const debtStatusColor: Record<DebtStatus, string> = {
  open: "processing",
  settled: "success",
};
