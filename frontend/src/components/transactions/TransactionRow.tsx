import type { TransactionInclusionState, TransactionItem } from "../../api/contracts";
import { formatCompactDay } from "../../presentation/dates";
import { inclusionOriginLabel } from "../../presentation/transactionStatus";
import { TransactionActions } from "./TransactionActions";
import { TransactionAmount } from "./TransactionAmount";
import { TransactionCategory } from "./TransactionCategory";

function rowTime(value: string | null): string | undefined {
  if (!value) return undefined;
  const instant = new Date(value);
  if (Number.isNaN(instant.getTime())) return undefined;
  return new Intl.DateTimeFormat("pt-BR", { hour: "2-digit", minute: "2-digit" }).format(instant);
}

function rowDay(value: string | null): string {
  return value ? formatCompactDay(value) : "Sem data";
}

/**
 * The second line: where the money moved and anything unusual about the
 * line (manual entry, ignored). On a compact screen the date joins it, since
 * there is no date column there.
 */
export function TransactionMeta({ item }: { item: TransactionItem }) {
  const ignored = item.inclusion.state === "ignored";
  const where =
    [item.account.name, item.account.institution].filter(Boolean).join(" · ") || "Conta não informada";
  return (
    <span className="transaction-meta">
      <span className="transaction-meta-date">{rowDay(item.occurred_at)}</span>
      <span className="transaction-meta-where">{where}</span>
      {item.origin === "manual" && <span className="transaction-meta-flag">Manual</span>}
      {ignored && (
        <span
          className="transaction-meta-flag is-ignored"
          title={item.inclusion.changed_at ? inclusionOriginLabel(item.inclusion) : undefined}
        >
          {item.inclusion.origin === "rule" ? "Ignorada (regra)" : "Ignorada"}
        </span>
      )}
    </span>
  );
}

/**
 * One transaction in a list: date | description + meta | category | amount |
 * actions on a wide screen; description/amount → meta → category on a compact
 * one. The row is flat — a hairline below, a hover wash — and the whole line
 * opens the panel: the description is the real button and its hit area is
 * stretched over the row, so nothing interactive nests inside anything else.
 * Secondary actions sit behind "···".
 */
export function TransactionRow({
  item,
  selected = false,
  onSelect,
  onInclusion,
  inclusionPending = false,
}: {
  item: TransactionItem;
  /** Whether this line's panel is open. */
  selected?: boolean;
  onSelect: (id: string) => void;
  onInclusion?: (id: string, state: TransactionInclusionState) => void;
  inclusionPending?: boolean;
}) {
  const ignored = item.inclusion.state === "ignored";
  const name = item.description ?? "transação sem descrição";
  const states = [
    ignored ? "is-ignored" : "",
    selected ? "is-selected" : "",
    inclusionPending ? "is-pending" : "",
  ].join(" ");

  return (
    <div className={`transaction-row ${states}`} aria-current={selected || undefined} aria-busy={inclusionPending || undefined}>
      <span className="transaction-row-date" title={rowTime(item.occurred_at)}>
        {rowDay(item.occurred_at)}
      </span>
      <span className="transaction-row-identity">
        <button
          type="button"
          className="transaction-row-open"
          aria-label={`Ver detalhes de ${name}`}
          title={item.description ?? undefined}
          onClick={() => onSelect(item.id)}
        >
          {item.description ?? "Descrição não informada"}
        </button>
        <TransactionMeta item={item} />
      </span>
      <TransactionCategory item={item} />
      <TransactionAmount item={item} className="transaction-row-amount" />
      <TransactionActions item={item} onSelect={onSelect} onInclusion={onInclusion} pending={inclusionPending} />
    </div>
  );
}
