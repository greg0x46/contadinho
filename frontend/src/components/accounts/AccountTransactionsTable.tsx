import { SwapOutlined } from "@ant-design/icons";
import { Alert, Skeleton } from "antd";
import { Link } from "react-router-dom";

import type { TransactionItem } from "../../api/contracts";
import { formatCompactDay } from "../../presentation/dates";
import { maskedCardNumber } from "../../presentation/accountLabels";
import { TransactionAmount } from "../transactions/TransactionAmount";
import { TransactionCategory } from "../transactions/TransactionCategory";

function rowDay(value: string | null): string {
  return value ? formatCompactDay(value) : "Sem data";
}

function rowTime(value: string | null): string | undefined {
  if (!value) return undefined;
  const instant = new Date(value);
  if (Number.isNaN(instant.getTime())) return undefined;
  return new Intl.DateTimeFormat("pt-BR", { hour: "2-digit", minute: "2-digit" }).format(instant);
}

/**
 * Recent activity as flat rows, reusing the same category/amount rendering
 * as the Transações list — but the account is fixed here, so unlike that
 * list's meta line, this one drops the account name and shows the card
 * instead. There is no click-to-open panel or "···" actions: deep
 * interaction (editing, ignoring) lives on the full transactions page linked
 * below.
 */
export function AccountTransactionsTable({
  transactions,
  isLoading,
  error,
  accountId,
}: {
  transactions: TransactionItem[];
  isLoading: boolean;
  error: unknown;
  accountId: string;
}) {
  return (
    <section className="accounts-section" aria-label="Últimas transações">
      <header className="accounts-section-header">
        <h2>Últimas transações</h2>
        <Link to={`/transacoes?account_ids=${encodeURIComponent(accountId)}`}>Ver todas</Link>
      </header>
      {error !== null && error !== undefined && (
        <Alert
          type="error"
          showIcon
          message="Não foi possível carregar as transações desta conta."
          style={{ margin: "0 1.25rem 1rem" }}
        />
      )}
      {isLoading ? (
        <div className="accounts-section-loading" role="status" aria-label="Carregando últimas transações">
          <Skeleton active paragraph={{ rows: 3 }} title={false} />
        </div>
      ) : transactions.length === 0 ? (
        <div className="debt-list-empty">
          <SwapOutlined className="debt-list-empty-icon" aria-hidden="true" />
          <span>Nenhuma transação sincronizada para esta conta.</span>
        </div>
      ) : (
        <div className="accounts-list">
          {transactions.map((item) => {
            const card = item.card === null ? null : maskedCardNumber(item.card.number);
            return (
              <div key={item.id} className="account-recent-row">
                <span className="account-recent-row-date" title={rowTime(item.occurred_at)}>
                  {rowDay(item.occurred_at)}
                </span>
                <span className="account-recent-row-identity">
                  <span className="account-recent-row-description" title={item.description ?? undefined}>
                    {item.description ?? "Descrição não informada"}
                  </span>
                  <span className="transaction-meta">
                    <span className="transaction-meta-date">{rowDay(item.occurred_at)}</span>
                    {card !== null && <span>{card}</span>}
                    {item.origin === "manual" && <span className="transaction-meta-flag">Manual</span>}
                    {item.inclusion.state === "ignored" && (
                      <span className="transaction-meta-flag is-ignored">Ignorada</span>
                    )}
                  </span>
                </span>
                <TransactionCategory item={item} />
                <TransactionAmount item={item} />
              </div>
            );
          })}
        </div>
      )}
    </section>
  );
}
