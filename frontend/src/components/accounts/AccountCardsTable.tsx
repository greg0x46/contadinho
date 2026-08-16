import { CreditCardOutlined } from "@ant-design/icons";
import type { ProColumns } from "@ant-design/pro-table";
import ProTable from "@ant-design/pro-table";
import { Alert, Typography } from "antd";

import type { AccountCard } from "../../api/contracts";
import { formatOptionalLocalDay } from "../../presentation/dates";
import { maskedCardNumber } from "../../presentation/accountLabels";

/** The physical cards spending against one credit account. The account is a
 *  single record — this table is the only place the individual card numbers
 *  show up, since they live in transaction metadata rather than in an entity
 *  of their own. */
export function AccountCardsTable({
  cards,
  isLoading,
  error,
}: {
  cards: AccountCard[];
  isLoading: boolean;
  error: unknown;
}) {
  const columns: ProColumns<AccountCard>[] = [
    {
      title: "Cartão",
      dataIndex: "card_number",
      render: (_, card) => maskedCardNumber(card.card_number),
    },
    { title: "Transações", dataIndex: "transaction_count" },
    {
      title: "Último uso",
      dataIndex: "last_transaction_at",
      render: (_, card) => formatOptionalLocalDay(card.last_transaction_at),
    },
  ];

  return (
    <section>
      <Typography.Title level={4}>Cartões desta conta</Typography.Title>
      {error !== null && error !== undefined && (
        <Alert
          type="error"
          showIcon
          message="Não foi possível carregar os cartões desta conta."
          style={{ marginBottom: 16 }}
        />
      )}
      <ProTable<AccountCard>
        aria-label="Cartões desta conta"
        columns={columns}
        dataSource={cards}
        loading={isLoading}
        rowKey="card_number"
        search={false}
        options={false}
        pagination={false}
        cardBordered
        scroll={{ x: "max-content" }}
        locale={{
          emptyText: (
            <div className="debt-list-empty">
              <CreditCardOutlined className="debt-list-empty-icon" aria-hidden="true" />
              <span>Nenhuma transação desta conta identifica um cartão.</span>
            </div>
          ),
        }}
      />
    </section>
  );
}
