import { CreditCardOutlined } from "@ant-design/icons";
import { Alert, Skeleton } from "antd";

import type { AccountCard } from "../../api/contracts";
import { formatOptionalLocalDay } from "../../presentation/dates";
import { maskedCardNumber } from "../../presentation/accountLabels";
import { SectionHeader } from "../layout";

/** The physical cards spending against one credit account — several PANs can
 *  share the same limit and invoice — as flat rows: card number left, last
 *  use right, matching the bank account list on Contas e cartões. The
 *  account is a single record; this is the only place the individual card
 *  numbers show up, since they live in transaction metadata rather than in
 *  an entity of their own. */
export function AccountCardsTable({
  cards,
  isLoading,
  error,
}: {
  cards: AccountCard[];
  isLoading: boolean;
  error: unknown;
}) {
  return (
    <section className="accounts-section" aria-label="Cartões vinculados">
      <SectionHeader
        title="Cartões vinculados"
        trailing={
          !isLoading && cards.length > 0 ? (
            <small>
              {cards.length} {cards.length === 1 ? "cartão" : "cartões"}
            </small>
          ) : undefined
        }
      />
      {error !== null && error !== undefined && (
        <Alert
          type="error"
          showIcon
          message="Não foi possível carregar os cartões desta conta."
          style={{ margin: "0 1.25rem 1rem" }}
        />
      )}
      {isLoading ? (
        <div className="accounts-section-loading" role="status" aria-label="Carregando cartões vinculados">
          <Skeleton active paragraph={{ rows: 2 }} title={false} />
        </div>
      ) : cards.length === 0 ? (
        <div className="debt-list-empty">
          <CreditCardOutlined className="debt-list-empty-icon" aria-hidden="true" />
          <span>Nenhum cartão vinculado identificado nas transações desta conta.</span>
        </div>
      ) : (
        <div className="accounts-list">
          {cards.map((card) => (
            <div key={card.card_number} className="detail-list-row">
              <span className="detail-list-row-identity">
                <span className="detail-list-row-name">{maskedCardNumber(card.card_number)}</span>
                <span className="detail-list-row-meta">
                  {card.transaction_count} {card.transaction_count === 1 ? "transação" : "transações"}
                </span>
              </span>
              <span className="detail-list-row-figure">
                <span className="detail-list-row-figure-label">Último uso</span>
                <span className="detail-list-row-figure-value">
                  {formatOptionalLocalDay(card.last_transaction_at)}
                </span>
              </span>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}
