import { FileTextOutlined } from "@ant-design/icons";
import { Alert, Skeleton } from "antd";

import type { AccountBill } from "../../api/contracts";
import { formatOptionalDay } from "../../presentation/dates";
import { formatAccountMoney } from "../../presentation/accountLabels";

/** Closed invoices as flat rows — the closing/due dates as identity, the
 *  total (with the minimum payment as its hint) as the figure — matching
 *  the bank account list's "identity left, figure right" row. */
export function AccountBillsTable({
  bills,
  isLoading,
  error,
}: {
  bills: AccountBill[];
  isLoading: boolean;
  error: unknown;
}) {
  return (
    <section className="accounts-section" aria-label="Faturas fechadas">
      <header className="accounts-section-header">
        <h2>Faturas fechadas</h2>
        {!isLoading && bills.length > 0 && (
          <small>
            {bills.length} {bills.length === 1 ? "fatura" : "faturas"}
          </small>
        )}
      </header>
      <p className="accounts-section-note">
        A instituição disponibiliza apenas faturas já fechadas — a fatura em aberto aparece no saldo do cartão.
      </p>
      {error !== null && error !== undefined && (
        <Alert
          type="error"
          showIcon
          message="Não foi possível carregar as faturas desta conta."
          style={{ margin: "1rem 1.25rem 0" }}
        />
      )}
      {isLoading ? (
        <div className="accounts-section-loading" role="status" aria-label="Carregando faturas fechadas">
          <Skeleton active paragraph={{ rows: 2 }} title={false} />
        </div>
      ) : bills.length === 0 ? (
        <div className="debt-list-empty">
          <FileTextOutlined className="debt-list-empty-icon" aria-hidden="true" />
          <span>Nenhuma fatura fechada sincronizada ainda.</span>
        </div>
      ) : (
        <div className="accounts-list">
          {bills.map((bill) => (
            <div key={bill.id} className="detail-list-row">
              <span className="detail-list-row-identity">
                <span className="detail-list-row-name">Fechou em {formatOptionalDay(bill.closing_date)}</span>
                <span className="detail-list-row-meta">Vence em {formatOptionalDay(bill.due_date)}</span>
              </span>
              <span className="detail-list-row-figure">
                <span className="detail-list-row-figure-value">
                  {formatAccountMoney(bill.total_amount, bill.currency_code)}
                </span>
                <span className="detail-list-row-figure-hint">
                  Mínimo {formatAccountMoney(bill.minimum_payment_amount, bill.currency_code)}
                </span>
              </span>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}
