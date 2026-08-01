type Metrics = {
  accounts_processed: number;
  transactions_inserted: number;
  transactions_updated: number;
};

export function SyncRunMetrics({ run }: { run: Metrics }) {
  return (
    <Row gutter={[16, 16]}>
      <Col xs={24} md={8}>
        <Statistic title="Contas processadas" value={run.accounts_processed} />
      </Col>
      <Col xs={24} md={8}>
        <Statistic title="Transações incluídas" value={run.transactions_inserted} />
      </Col>
      <Col xs={24} md={8}>
        <Statistic title="Transações atualizadas" value={run.transactions_updated} />
      </Col>
    </Row>
  );
}
import { Col, Row, Statistic } from "antd";
