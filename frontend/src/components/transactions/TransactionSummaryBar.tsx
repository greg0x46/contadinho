import { ArrowDownOutlined, ArrowUpOutlined, RightOutlined } from "@ant-design/icons";
import { useState } from "react";

import type { CurrencyTotals } from "../../api/contracts";
import { formatBRL } from "../../presentation/money";

function balanceSign(balance: string): "negative" | "positive" | "zero" {
  if (balance.startsWith("-")) return "negative";
  if (Number(balance) === 0) return "zero";
  return "positive";
}

export function TransactionSummaryBar({
  totals,
  totalItems,
  busy,
}: {
  totals: CurrencyTotals[];
  totalItems: number;
  busy?: boolean;
}) {
  const [expanded, setExpanded] = useState(false);
  const brl = totals.find((total) => total.currency_code === "BRL") ?? {
    currency_code: "BRL",
    inflow: "0",
    outflow: "0",
    balance: "0",
  };

  return (
    <section className="transaction-summary" aria-label="Resumo financeiro" aria-busy={busy}>
      <div className={`transaction-summary-bar ${expanded ? "is-expanded" : ""}`}>
        <p className="transaction-summary-count">
          {totalItems.toLocaleString("pt-BR")} {totalItems === 1 ? "transação" : "transações"}
        </p>

        <div id="transaction-summary-inflow" className="transaction-summary-figure transaction-summary-figure-inflow">
          <span className="transaction-summary-figure-label">
            <ArrowUpOutlined aria-hidden="true" /> Entradas
          </span>
          <span className="transaction-summary-figure-value">{formatBRL(brl.inflow)}</span>
        </div>

        <div id="transaction-summary-outflow" className="transaction-summary-figure transaction-summary-figure-outflow">
          <span className="transaction-summary-figure-label">
            <ArrowDownOutlined aria-hidden="true" /> Saídas
          </span>
          <span className="transaction-summary-figure-value">{formatBRL(brl.outflow)}</span>
        </div>

        <div className="transaction-summary-figure transaction-summary-figure-balance">
          <span className="transaction-summary-figure-label">Resultado do período</span>
          <span className={`transaction-summary-figure-value transaction-summary-balance is-${balanceSign(brl.balance)}`}>
            {formatBRL(brl.balance)}
          </span>
        </div>

        <button
          type="button"
          className="transaction-summary-toggle"
          aria-expanded={expanded}
          aria-controls="transaction-summary-inflow transaction-summary-outflow"
          onClick={() => setExpanded((value) => !value)}
        >
          {expanded ? "Ocultar resumo" : "Ver resumo"}
          <RightOutlined aria-hidden="true" />
        </button>
      </div>
    </section>
  );
}
