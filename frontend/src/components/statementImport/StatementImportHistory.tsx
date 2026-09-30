import { Link } from "react-router-dom";

import type { ImportHistoryItem } from "../../api/statementImports";
import { formatDate } from "../../presentation/dates";
import { DataCardSummary } from "../layout/DataCard";

/** The summary strip of the import history DataCard: how many imports are listed. */
export function StatementImportHistorySummary({ items }: { items: ImportHistoryItem[] }) {
  return (
    <DataCardSummary label="Resumo das importações">
      <p className="statement-history-summary-count">
        {items.length} {items.length === 1 ? "importação" : "importações"}
      </p>
    </DataCardSummary>
  );
}

/**
 * Past imports as flat rows: account and file on the left, what the file
 * contained beside a link to the account. Like the sync history it lists
 * runs, but as rows so a phone never scrolls it sideways.
 */
export function StatementImportHistory({ items }: { items: ImportHistoryItem[] }) {
  return (
    <ul className="statement-history-list" aria-label="Extratos importados">
      {items.map((item) => (
        <li key={item.run_id} className="statement-history-row">
          <div className="statement-history-identity">
            <span className="statement-history-account">{item.account_name}</span>
            <span className="statement-history-meta">{item.filename} · {formatDate(item.created_at)}</span>
          </div>
          <span className="statement-history-counts">
            {item.counts.new} novos · {item.counts.duplicate} já importados · {item.counts.invalid} inválidos
          </span>
          <Link
            className="statement-history-link"
            aria-label={`Ver conta ${item.account_name}`}
            to={`/contas-e-cartoes/${item.account_id}`}
          >
            Ver conta
          </Link>
        </li>
      ))}
    </ul>
  );
}
