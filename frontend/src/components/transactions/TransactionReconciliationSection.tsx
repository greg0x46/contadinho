import { Alert, Button, Descriptions, Popconfirm, Select, Typography } from "antd";
import { useState } from "react";

import type { ReconciliationOption } from "../../api/contracts";
import { useTransactionReconciliation } from "../../hooks/useTransactionReconciliation";
import { formatDay } from "../../presentation/dates";
import { formatBRL } from "../../presentation/money";
import { reconciliationOriginLabel } from "../../presentation/reconciliationLabels";

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

// The option's value has to carry both halves of an occurrence's address —
// the commitment and the calendar day — because occurrences have no id.
function optionValue(option: ReconciliationOption): string {
  return `${option.commitment_id}|${option.occurrence_date}`;
}

function optionLabel(option: ReconciliationOption): string {
  return `${option.commitment_name} · ${formatDay(option.occurrence_date)} · ${formatBRL(option.expected_amount)}`;
}

/**
 * Reconciliation seen from a transaction: what recurring occurrence it
 * settles, and which nearby occurrences it could settle instead.
 *
 * Only occurrences flowing the same direction as the transaction are offered
 * — an outflow can never settle an income commitment — so an empty list
 * usually means there is simply no matching commitment near this date.
 */
export function TransactionReconciliationSection({
  transactionId,
  ignored = false,
}: {
  transactionId: string;
  ignored?: boolean;
}) {
  // An ignored transaction is out of the totals, so it can settle nothing.
  // Saying that beats letting the empty picker read as "no commitment
  // nearby", which would send the user looking for the wrong problem.
  const reconciliation = useTransactionReconciliation(transactionId, !ignored);
  const [error, setError] = useState<string | null>(null);

  const run = async (action: () => Promise<unknown>, fallback: string) => {
    setError(null);
    try {
      await action();
    } catch (caught) {
      setError(errorMessage(caught, fallback));
    }
  };

  const current = reconciliation.current;

  return (
    <Descriptions title="Conciliação" column={1} size="small" colon={false}>
      <Descriptions.Item label="Recorrência">
        {ignored && (
          <Typography.Text type="secondary">
            Transações ignoradas não entram nos totais, então não conciliam uma recorrência.
          </Typography.Text>
        )}
        {!ignored && error && (
          <Alert
            type="error"
            showIcon
            closable
            style={{ marginBottom: 8 }}
            onClose={() => setError(null)}
            message={error}
          />
        )}
        {ignored ? null : reconciliation.isLoading ? (
          <Typography.Text type="secondary">Carregando…</Typography.Text>
        ) : current !== null ? (
          <>
            <div>
              {current.commitment_name} · {formatDay(current.occurrence_date)} ·{" "}
              {formatBRL(current.expected_amount)}
            </div>
            <div className="transaction-category-origin">
              Conciliação {reconciliationOriginLabel[current.origin]}
            </div>
            <Popconfirm
              title="Desconciliar"
              description="A ocorrência volta a ser projetada no relatório. A transação permanece inalterada."
              onConfirm={() =>
                run(
                  () =>
                    reconciliation.detach({
                      commitmentId: current.commitment_id,
                      occurrenceDate: current.occurrence_date,
                    }),
                  "Não foi possível desconciliar.",
                )
              }
              okText="Desconciliar"
              cancelText="Cancelar"
            >
              <Button type="link" danger size="small" loading={reconciliation.isWriting}>
                Desconciliar
              </Button>
            </Popconfirm>
          </>
        ) : (
          <>
            <Select
              aria-label="Conciliar com uma recorrência"
              style={{ minWidth: 240 }}
              placeholder="Conciliar com…"
              value={null}
              loading={reconciliation.isWriting}
              disabled={reconciliation.options.length === 0}
              notFoundContent="Nenhuma ocorrência compatível por perto"
              options={reconciliation.options.map((option) => ({
                value: optionValue(option),
                label: optionLabel(option),
              }))}
              onChange={(value: string) => {
                const [commitmentId, occurrenceDate] = value.split("|");
                void run(
                  () => reconciliation.reconcile({ commitmentId, occurrenceDate }),
                  "Não foi possível conciliar a transação.",
                );
              }}
            />
            {reconciliation.options.length === 0 && !reconciliation.isLoading && (
              <div className="transaction-category-origin">
                Nenhuma ocorrência de recorrência compatível perto desta data.
              </div>
            )}
          </>
        )}
      </Descriptions.Item>
    </Descriptions>
  );
}
