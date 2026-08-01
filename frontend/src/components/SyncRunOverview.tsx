import type { SyncRunDetail } from "../api/contracts";
import { formatDate, formatOptionalDate } from "../presentation/dates";
import { SyncRunMetrics } from "./SyncRunMetrics";
import { SyncStatusBadge } from "./SyncStatusBadge";

export function SyncRunOverview({ run }: { run: SyncRunDetail }) {
  return (
    <Card>
      <Flex vertical gap="large">
        <Flex justify="space-between" align="center" wrap>
          <Typography.Title id="result-title" level={2}>
            Resumo da sincronização
          </Typography.Title>
        <SyncStatusBadge status={run.status} />
        </Flex>
        <Descriptions column={{ xs: 1, md: 3 }} bordered size="small">
          <Descriptions.Item label="Identificador">{run.id}</Descriptions.Item>
          <Descriptions.Item label="Início">
            <time dateTime={run.started_at}>{formatDate(run.started_at)}</time>
          </Descriptions.Item>
          <Descriptions.Item label="Conclusão">
            {run.finished_at === null ? (
              formatOptionalDate(null)
            ) : (
              <time dateTime={run.finished_at}>{formatOptionalDate(run.finished_at)}</time>
            )}
          </Descriptions.Item>
        </Descriptions>
        <SyncRunMetrics run={run} />
      </Flex>
    </Card>
  );
}
import { Card, Descriptions, Flex, Typography } from "antd";
