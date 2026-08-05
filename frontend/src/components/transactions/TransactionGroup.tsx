import { Button, Tag } from "antd";

import type {
  TransactionGroup as Group,
  TransactionInclusionState,
  TransactionItem,
} from "../../api/contracts";
import {
  internalCategoryName,
  internalCategoryOriginLabel,
  renderCategoryIcon,
} from "../../presentation/categoryLabels";
import { formatBRL, formatSignedBRL } from "../../presentation/money";
import { inclusionOriginLabel, statusLabel } from "../../presentation/transactionStatus";

const shortDate = new Intl.DateTimeFormat("pt-BR", {
  day: "numeric",
  month: "short",
  timeZone: "UTC",
});

function dateOnly(value: string): Date {
  return new Date(`${value}T12:00:00Z`);
}

function groupLabel(group: Group): string {
  if (group.kind === "none") return "";
  if (group.kind === "undated") return "Sem data";
  if (group.kind === "day") return shortDate.format(dateOnly(group.start_date!));
  if (group.start_date && group.end_date) {
    return `${shortDate.format(dateOnly(group.start_date))} – ${shortDate.format(
      dateOnly(group.end_date),
    )}`;
  }
  return group.key;
}

function rowDate(value: string | null): { date: string; time: string } {
  if (!value) return { date: "Sem data", time: "" };
  const instant = new Date(value);
  return {
    date: new Intl.DateTimeFormat("pt-BR", {
      day: "2-digit",
      month: "short",
    }).format(instant),
    time: new Intl.DateTimeFormat("pt-BR", {
      hour: "2-digit",
      minute: "2-digit",
    }).format(instant),
  };
}

function shownValue(item: TransactionItem): string {
  if (!item.effective_money || item.effective_money.currency_code !== "BRL") {
    return "Valor indisponível";
  }
  return formatSignedBRL(item.effective_money.value, item.classification);
}

function TransactionRow({
  item,
  onSelect,
  onInclusion,
  inclusionPending,
}: {
  item: TransactionItem;
  onSelect: (id: string) => void;
  onInclusion?: (id: string, state: TransactionInclusionState) => void;
  inclusionPending?: boolean;
}) {
  const occurred = rowDate(item.occurred_at);
  const status = statusLabel(item.provider_status);
  const ignored = item.inclusion.state === "ignored";
  const target = ignored ? "considered" : "ignored";
  return (
    <div className={`transaction-row ${ignored ? "transaction-row-ignored" : ""}`}>
    <button
      type="button"
      className="transaction-detail-action"
      onClick={() => onSelect(item.id)}
      aria-label={`Ver detalhes de ${item.description ?? "transação sem descrição"}`}
    >
      <span className="transaction-date">
        <strong>{occurred.date}</strong>
        <small>{occurred.time}</small>
      </span>
      <span className="transaction-identity">
        <strong>
          {item.description ?? "Descrição não informada"}
          {ignored && (
            <Tag
              className="transaction-ignored-tag"
              title={item.inclusion.changed_at ? inclusionOriginLabel(item.inclusion) : undefined}
            >
              {item.inclusion.origin === "rule" ? "Ignorada (regra)" : "Ignorada"}
            </Tag>
          )}
        </strong>
        <small>
          {[item.account.name, item.account.institution].filter(Boolean).join(" · ") ||
            "Conta não informada"}
        </small>
      </span>
      <span className="transaction-badges">
        <Tag
          color={item.internal_category?.color}
          title={item.internal_category ? internalCategoryOriginLabel(item.internal_category) : undefined}
        >
          {item.internal_category && renderCategoryIcon(item.internal_category.icon)} {internalCategoryName(item)}
        </Tag>
        <Tag className={`status-${item.provider_status?.toLocaleLowerCase() ?? "unknown"}`}>
          {status}
        </Tag>
      </span>
      <strong className={`transaction-amount amount-${item.classification}`}>
        {shownValue(item)}
      </strong>
    </button>
      <Button
        type="link"
        className="transaction-inclusion-action"
        loading={inclusionPending}
        disabled={!onInclusion}
        onClick={() => onInclusion?.(item.id, target)}
        aria-label={`${ignored ? "Restaurar" : "Ignorar"} ${item.description ?? "transação sem descrição"}`}
      >
        {ignored ? "Restaurar" : "Ignorar"}
      </Button>
    </div>
  );
}

export function TransactionGroup({
  group,
  items,
  onSelect,
  onInclusion,
  pendingTransactionId,
}: {
  group: Group;
  items: TransactionItem[];
  onSelect?: (id: string) => void;
  onInclusion?: (id: string, state: TransactionInclusionState) => void;
  pendingTransactionId?: string | null;
}) {
  const brl = group.totals.find((total) => total.currency_code === "BRL");
  const split = group.has_items_before || group.has_items_after;
  return (
    <section
      className={`transaction-group ${group.kind === "none" ? "transaction-group-flat" : ""}`}
      aria-labelledby={group.kind === "none" ? undefined : `group-${group.key}`}
    >
      {group.kind !== "none" && (
        <header className="transaction-group-header">
          <div>
            <h2 id={`group-${group.key}`}>{groupLabel(group)}</h2>
            {split && <small>Período continua em outra página</small>}
          </div>
          <dl>
            <div>
              <dt>Entradas</dt>
              <dd>{formatBRL(brl?.inflow ?? "0")}</dd>
            </div>
            <div>
              <dt>Saídas</dt>
              <dd>{formatBRL(brl?.outflow ?? "0")}</dd>
            </div>
            <div>
              <dt>Saldo</dt>
              <dd>{formatBRL(brl?.balance ?? "0")}</dd>
            </div>
          </dl>
        </header>
      )}
      <div className="transaction-list">
        {items.map((item) => (
          <TransactionRow
            key={item.id}
            item={item}
            onSelect={onSelect ?? (() => undefined)}
            onInclusion={onInclusion}
            inclusionPending={pendingTransactionId === item.id}
          />
        ))}
      </div>
    </section>
  );
}
