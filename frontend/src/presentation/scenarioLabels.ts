import type { ScenarioTransactionStatus } from "../api/contracts";

export const scenarioTransactionStatusLabel: Record<ScenarioTransactionStatus, string> = {
  atrasada: "Atrasada",
  projetada: "Planejado",
  paga_parcialmente: "Paga parcialmente",
  paga: "Paga",
  paga_a_mais: "Paga a mais",
};

export const scenarioTransactionStatusColor: Record<ScenarioTransactionStatus, string> = {
  atrasada: "error",
  projetada: "default",
  paga_parcialmente: "warning",
  paga: "success",
  paga_a_mais: "processing",
};

// A "projetada" tag renders dashed, per the spec: it's the one status that
// hasn't happened yet (unlike atrasada, which is also unpaid but already
// past its date), so it needs to read visually different from the "paga*"
// tags that represent a real, already-happened allocation.
export const scenarioTransactionStatusDashed: Record<ScenarioTransactionStatus, boolean> = {
  atrasada: false,
  projetada: true,
  paga_parcialmente: false,
  paga: false,
  paga_a_mais: false,
};
