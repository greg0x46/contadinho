import type { CertaintyTier, TimelineSourceKind } from "../api/contracts";

export const certaintyTierLabel: Record<CertaintyTier, string> = {
  realizado: "Realizado",
  confirmado: "Confirmado",
  projetado: "Projetado",
  hipotetico: "Hipotético",
};

export const timelineSourceLabel: Record<TimelineSourceKind, string> = {
  real: "Transação real",
  recorrente: "Compromisso recorrente",
  plano_pagamento: "Plano de pagamento",
  cenario: "Cenário",
};

// Fixed categorical trio for the monthly evolution chart — validated with
// the dataviz skill's validator (adjacent-pair CVD/normal-vision/contrast
// all PASS in both light and dark): slot 1 (blue) for income, slot 8 (red)
// for expense, slot 7 (violet) for the net result — chosen over an
// arbitrary hue cycle because these three roles repeat across every chart
// in this feature and must always mean the same thing.
export const monthlyEvolutionColor = {
  income: "#2a78d6",
  expense: "#e34948",
  result: "#4a3aa7",
} as const;
