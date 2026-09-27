import { Alert } from "antd";

import type { SyncFailure } from "../api/contracts";
import { formatDate } from "../presentation/dates";
import { getFailureStageLabel } from "../presentation/syncStatus";
import { SectionHeader } from "./layout";

/**
 * Every failure the run recorded, as one bounded section instead of a
 * page-level heading — each entry keeps its own error Alert (the severity
 * coloring earns its keep here, unlike a plain row) but the related
 * account/transaction ids are now a quiet meta line instead of a nested
 * Descriptions table.
 */
export function SyncFailureList({ failures }: { failures: SyncFailure[] }) {
  if (failures.length === 0) {
    return null;
  }
  return (
    <section className="accounts-section" aria-label="Falhas registradas">
      <SectionHeader
        title="Falhas registradas"
        trailing={
          <small>
            {failures.length} {failures.length === 1 ? "falha" : "falhas"}
          </small>
        }
      />
      <div className="accounts-list sync-failure-list">
        {failures.map((failure, index) => (
          <div key={`${failure.code}-${failure.occurred_at}-${index}`} className="sync-failure-row">
            <Alert
              type="error"
              showIcon
              message={getFailureStageLabel(failure.stage)}
              description={
                <div className="sync-failure-details">
                  <span>{failure.message}</span>
                  <span className="sync-failure-meta">
                    Ocorrida em <time dateTime={failure.occurred_at}>{formatDate(failure.occurred_at)}</time>
                  </span>
                  {failure.external_account_id !== null && (
                    <span className="sync-failure-meta">Conta relacionada: {failure.external_account_id}</span>
                  )}
                  {failure.external_transaction_id !== null && (
                    <span className="sync-failure-meta">
                      Transação relacionada: {failure.external_transaction_id}
                    </span>
                  )}
                </div>
              }
            />
          </div>
        ))}
      </div>
    </section>
  );
}
