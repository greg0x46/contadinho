import { ArrowDownOutlined, ArrowUpOutlined, WalletOutlined } from "@ant-design/icons";
import { Card } from "antd";

import type { CurrencyTotals } from "../../api/contracts";
import { formatBRL } from "../../presentation/money";

export function TransactionTotals({ totals }: { totals: CurrencyTotals[] }) {
  const brl = totals.find((total) => total.currency_code === "BRL") ?? {
    currency_code: "BRL",
    inflow: "0",
    outflow: "0",
    balance: "0",
  };
  const cards = [
    {
      key: "inflow",
      label: "Entradas",
      value: brl.inflow,
      icon: <ArrowUpOutlined aria-hidden="true" />,
    },
    {
      key: "outflow",
      label: "Saídas",
      value: brl.outflow,
      icon: <ArrowDownOutlined aria-hidden="true" />,
    },
    {
      key: "balance",
      label: "Saldo do período",
      value: brl.balance,
      icon: <WalletOutlined aria-hidden="true" />,
    },
  ] as const;

  return (
    <section className="transaction-summary" aria-label="Resumo financeiro">
      {cards.map((card) => (
        <Card key={card.key} size="small" className={`summary-card summary-${card.key}`}>
          <div className="summary-label">
            {card.icon}
            <span>{card.label}</span>
          </div>
          <strong>{formatBRL(card.value)}</strong>
        </Card>
      ))}
    </section>
  );
}
