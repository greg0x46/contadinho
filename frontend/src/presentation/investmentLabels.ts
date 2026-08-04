import type { Investment, InvestmentTransaction } from "../api/contracts";

// Pluggy's investment "type"/"subtype"/movement "type" fields are free text
// from the provider (not a closed enum contadinho-go validates), so these
// maps only translate the values we've actually seen — anything else falls
// back to the raw string instead of failing.
const investmentTypeLabels: Record<string, string> = {
  MUTUAL_FUND: "Fundo de investimento",
  FIXED_INCOME: "Renda fixa",
  EQUITY: "Renda variável",
  SECURITY: "Título",
  COE: "COE",
  ETF: "ETF",
  PENSION: "Previdência",
};

export function investmentTypeLabel(type: string | null): string {
  if (type === null) return "Tipo não informado";
  return investmentTypeLabels[type] ?? type;
}

const movementTypeLabels: Record<string, string> = {
  BUY: "Aplicação",
  SELL: "Resgate",
  APPLICATION: "Aplicação",
  REDEMPTION: "Resgate",
  DIVIDEND: "Dividendo",
  INCOME: "Rendimento",
  TAX: "Imposto",
  FEE: "Taxa",
};

export function movementTypeLabel(type: string | null): string {
  if (type === null) return "Movimentação";
  return movementTypeLabels[type] ?? type;
}

export type YieldEstimate = {
  value: string;
  source: "informado" | "calculado";
} | null;

// The backend computes this (GET /api/investments[/:id]): Pluggy's own
// amount_profit when the provider sends it ("informado"), otherwise balance
// minus net contributed from the investment's buy/sell history ("calculado").
// Computed server-side so the list page gets it without an N+1 fetch of every
// investment's transactions. Do not resurrect a client-side balance - amount
// fallback: investment.amount for renda fixa holdings is quantity × value
// (current gross mark value), not invested principal, and using it produced
// false "negative yields" on healthy CDBs (see two CDBs flagged in August
// 2026 — the gap was withheld tax on early redemption, not a loss).
export function investmentYield(investment: Investment): YieldEstimate {
  if (investment.yield_value === null || investment.yield_source === null) return null;
  return { value: investment.yield_value, source: investment.yield_source };
}

export function netContributed(transactions: InvestmentTransaction[]): string {
  const total = transactions.reduce((sum, t) => {
    if (t.amount === null) return sum;
    const amount = Number(t.amount);
    const isWithdrawal = t.movement_type === "SELL" || t.movement_type === "REDEMPTION";
    return sum + (isWithdrawal ? -amount : amount);
  }, 0);
  return total.toFixed(2);
}
