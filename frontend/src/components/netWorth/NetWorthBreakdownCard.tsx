import { Card, Flex, Statistic } from "antd";

import type { NetWorthSnapshot } from "../../api/contracts";
import { formatBRL, formatSignedBRL } from "../../presentation/money";
import { colors } from "../../theme/tokens";

// A debt payable and a card's current bill-cycle total are summed as
// distinct liabilities, never deduplicated — see internal/networth's
// Breakdown doc comment for why.
//
// Receivables (money owed to the user) have no field here at all: they're a
// future possibility, not a realized asset — see internal/networth's
// Breakdown doc comment.
export function NetWorthBreakdownCard({ snapshot }: { snapshot: NetWorthSnapshot }) {
  return (
    <Card title="Composição do patrimônio" size="small" style={{ marginBottom: 16 }}>
      <Statistic
        title="Patrimônio líquido"
        value={formatBRL(snapshot.net_worth)}
        valueStyle={{ color: colors.textPrimary, fontSize: 32, fontWeight: 600 }}
        style={{ marginBottom: 16 }}
      />
      <Flex gap="middle" wrap>
        <Statistic
          title="Caixa"
          value={formatBRL(snapshot.cash_balance)}
          valueStyle={{ fontSize: 16 }}
        />
        <Statistic
          title="Investimentos"
          value={formatBRL(snapshot.investment_balance)}
          valueStyle={{ fontSize: 16 }}
        />
        <Statistic
          title="Cartão de crédito"
          value={formatSignedBRL(snapshot.credit_card_balance, "outflow")}
          valueStyle={{ fontSize: 16 }}
        />
        <Statistic
          title="Dívidas"
          value={formatSignedBRL(snapshot.payables_debt, "outflow")}
          valueStyle={{ fontSize: 16 }}
        />
      </Flex>
    </Card>
  );
}
