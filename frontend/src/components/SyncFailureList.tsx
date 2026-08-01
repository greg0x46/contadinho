import type { SyncFailure } from "../api/contracts";
import { formatDate } from "../presentation/dates";
import { getFailureStageLabel } from "../presentation/syncStatus";

export function SyncFailureList({ failures }: { failures: SyncFailure[] }) {
  if (failures.length === 0) {
    return null;
  }
  return (
    <Flex vertical gap="middle">
      <Typography.Title id="failures-title" level={2}>
        Falhas registradas
      </Typography.Title>
      <div className="failure-list">
        {failures.map((failure, index) => (
          <Alert
            key={`${failure.code}-${failure.occurred_at}-${index}`}
            type="error"
            showIcon
            message={getFailureStageLabel(failure.stage)}
            description={
              <Flex vertical gap="small">
                <span>{failure.message}</span>
                <Descriptions column={1} size="small">
                  <Descriptions.Item label="Ocorrida em">
                  <time dateTime={failure.occurred_at}>{formatDate(failure.occurred_at)}</time>
                  </Descriptions.Item>
              {failure.external_account_id !== null && (
                    <Descriptions.Item label="Conta relacionada">
                      {failure.external_account_id}
                    </Descriptions.Item>
              )}
              {failure.external_transaction_id !== null && (
                    <Descriptions.Item label="Transação relacionada">
                      {failure.external_transaction_id}
                    </Descriptions.Item>
              )}
                </Descriptions>
              </Flex>
            }
          />
        ))}
      </div>
    </Flex>
  );
}
import { Alert, Descriptions, Flex, Typography } from "antd";
