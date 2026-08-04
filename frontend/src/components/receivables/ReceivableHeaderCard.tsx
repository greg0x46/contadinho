import { Button, Card, Popconfirm, Tag, Typography } from "antd";

import type { Receivable } from "../../api/contracts";
import { receivableStatusColor, receivableStatusLabel } from "../../presentation/receivableLabels";
import { formatBRL } from "../../presentation/money";

export function ReceivableHeaderCard({
  receivable,
  onEdit,
  onDelete,
}: {
  receivable: Receivable;
  onEdit: () => void;
  onDelete: () => void;
}) {
  const total = Number(receivable.total_amount);
  const received = Number(receivable.received_amount);
  const receivedShare = total > 0 ? Math.min(100, Math.max(0, (received / total) * 100)) : 0;

  return (
    <Card className="debt-header-card dashboard-widget">
      <div className="debt-header-identity">
        <Typography.Title level={3} className="debt-header-name">
          {receivable.name}
        </Typography.Title>
        <Tag color={receivableStatusColor[receivable.status]}>{receivableStatusLabel[receivable.status]}</Tag>
      </div>

      <div className="debt-header-figure">
        <p className="dashboard-hero-figure">{formatBRL(receivable.remaining_amount)} a receber</p>
        <div className="dashboard-meter" aria-hidden="true">
          <span
            className="dashboard-meter-segment debt-row-meter-paid"
            style={{ width: `${receivedShare}%` }}
          />
          <span
            className="dashboard-meter-segment debt-row-meter-remaining"
            style={{ width: `${100 - receivedShare}%` }}
          />
        </div>
        <ul className="debt-header-legend-inline">
          <li>
            <span>Recebido</span>
            <strong>{formatBRL(receivable.received_amount)}</strong>
          </li>
          <li>
            <span>Total</span>
            <strong>{formatBRL(receivable.total_amount)}</strong>
          </li>
        </ul>
      </div>

      <div className="debt-header-actions">
        <Button onClick={onEdit}>Editar</Button>
        <Popconfirm
          title="Excluir conta a receber"
          description={
            receivable.link_count === 0
              ? "Esta ação não pode ser desfeita."
              : `${receivable.link_count} transação(ões) vinculada(s) serão desfeitas; as transações em si permanecem inalteradas.`
          }
          onConfirm={onDelete}
          okText="Excluir"
          cancelText="Cancelar"
        >
          <Button danger>Excluir</Button>
        </Popconfirm>
      </div>
    </Card>
  );
}
