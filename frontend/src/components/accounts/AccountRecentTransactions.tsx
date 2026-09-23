import { SwapOutlined } from "@ant-design/icons";
import { Alert, Skeleton } from "antd";
import { Link } from "react-router-dom";

import type { TransactionInclusionState, TransactionItem } from "../../api/contracts";
import { TransactionRow } from "../transactions/TransactionRow";

/**
 * Recent activity on this account, reusing the exact same TransactionRow as
 * the Transações list — same description/amount/category/meta hierarchy,
 * same click-to-open panel — rather than a page-specific row. Deep filtering
 * and pagination still live on the full list; "Ver todas" links there
 * pre-filtered to this account.
 */
export function AccountRecentTransactions({
  transactions,
  isLoading,
  error,
  accountId,
  selectedId,
  onSelect,
  onInclusion,
  pendingTransactionId,
}: {
  transactions: TransactionItem[];
  isLoading: boolean;
  error: unknown;
  accountId: string;
  selectedId: string | null;
  onSelect: (id: string) => void;
  onInclusion?: (id: string, state: TransactionInclusionState) => void;
  pendingTransactionId?: string | null;
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
        <div className="transaction-list">
          {transactions.map((item) => (
            <TransactionRow
              key={item.id}
              item={item}
              selected={item.id === selectedId}
              onSelect={onSelect}
              onInclusion={onInclusion}
              inclusionPending={pendingTransactionId === item.id}
            />
          ))}
        </div>
      )}
    </section>
  );
}
