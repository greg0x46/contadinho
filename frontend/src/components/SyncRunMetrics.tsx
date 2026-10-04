type Metrics = {
  accounts_processed: number;
  transactions_inserted: number;
  transactions_updated: number;
};

/**
 * What a run moved, as a flat definition strip (label small, figure 500)
 * instead of three statistic cards — the same shape the summary strips use.
 */
export function SyncRunMetrics({ run }: { run: Metrics }) {
  const items = [
    { label: "Contas processadas", value: run.accounts_processed },
    { label: "Transações incluídas", value: run.transactions_inserted },
    { label: "Transações atualizadas", value: run.transactions_updated },
  ];
  return (
    <dl className="sync-run-metrics">
      {items.map((item) => (
        <div key={item.label}>
          <dt>{item.label}</dt>
          <dd>{item.value}</dd>
        </div>
      ))}
    </dl>
  );
}
