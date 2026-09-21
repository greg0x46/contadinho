import type { TransactionItem } from "../../api/contracts";
import { formatSignedBRL } from "../../presentation/money";

function transactionAmountText(item: TransactionItem): string {
  if (!item.effective_money || item.effective_money.currency_code !== "BRL") {
    return "Valor indisponível";
  }
  return formatSignedBRL(item.effective_money.value, item.classification);
}

/** The signed BRL amount, coloured by direction; the anchor of every row scan. */
export function TransactionAmount({ item, className = "" }: { item: TransactionItem; className?: string }) {
  return (
    <span className={`transaction-amount amount-${item.classification} ${className}`}>
      {transactionAmountText(item)}
    </span>
  );
}
