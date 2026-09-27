import type { SyncRunDetail } from "../api/contracts";
import { formatDate, formatOptionalDate } from "../presentation/dates";
import { SectionHeader } from "./layout";
import { SyncRunMetrics } from "./SyncRunMetrics";
import { SyncStatusBadge } from "./SyncStatusBadge";

/**
 * The sync run detail page's opening block — status first, then the run's
 * own identity (when it started/finished, which run this is) as quiet
 * metadata, then the volume it moved. A flat bounded section with a
 * definition list, the same "card outside, hairline rows inside" language
 * as the account detail page, instead of the old Card+Descriptions table.
 */
export function SyncRunOverview({ run }: { run: SyncRunDetail }) {
  return (
    <section className="accounts-section" aria-label="Resumo da sincronização">
      <SectionHeader title="Resumo da sincronização" trailing={<SyncStatusBadge status={run.status} />} />
      <div className="sync-run-overview-body">
        <dl className="sync-run-overview-meta">
          <div>
            <dt>Identificador</dt>
            <dd>{run.id}</dd>
          </div>
          <div>
            <dt>Início</dt>
            <dd>
              <time dateTime={run.started_at}>{formatDate(run.started_at)}</time>
            </dd>
          </div>
          <div>
            <dt>Conclusão</dt>
            <dd>
              {run.finished_at === null ? (
                formatOptionalDate(null)
              ) : (
                <time dateTime={run.finished_at}>{formatOptionalDate(run.finished_at)}</time>
              )}
            </dd>
          </div>
        </dl>
        <SyncRunMetrics run={run} />
      </div>
    </section>
  );
}
