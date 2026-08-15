import { Button, Card, Popconfirm, Tag, Typography } from "antd";

import type { Payable } from "../../api/contracts";
import { payableStatusColor, payableStatusLabel, payableVocabulary } from "../../presentation/payableLabels";
import { formatBRL } from "../../presentation/money";

export function PayableHeaderCard({
  payable,
  onEdit,
  onDelete,
}: {
  payable: Payable;
  onEdit: () => void;
  onDelete: () => void;
}) {
  const vocab = payableVocabulary[payable.kind];
  const total = Number(payable.total_amount);
  const settled = Number(payable.settled_amount);
  const settledShare = total > 0 ? Math.min(100, Math.max(0, (settled / total) * 100)) : 0;

  return (
    <Card className="debt-header-card dashboard-widget">
      <div className="debt-header-identity">
        <Typography.Title level={3} className="debt-header-name">
          {payable.name}
        </Typography.Title>
        <Tag color={payableStatusColor[payable.status]}>{payableStatusLabel[payable.kind][payable.status]}</Tag>
      </div>

      <div className="debt-header-figure">
        <p className="dashboard-hero-figure">
          {formatBRL(payable.remaining_amount)} {vocab.remainingSuffix}
        </p>
        <div className="dashboard-meter" aria-hidden="true">
          <span
            className="dashboard-meter-segment debt-row-meter-paid"
            style={{ width: `${settledShare}%` }}
          />
          <span
            className="dashboard-meter-segment debt-row-meter-remaining"
            style={{ width: `${100 - settledShare}%` }}
          />
        </div>
        <ul className="debt-header-legend-inline">
          <li>
            <span>{vocab.settledColumnLabel}</span>
            <strong>{formatBRL(payable.settled_amount)}</strong>
          </li>
          <li>
            <span>Total</span>
            <strong>{formatBRL(payable.total_amount)}</strong>
          </li>
        </ul>
      </div>

      <div className="debt-header-actions">
        <Button onClick={onEdit}>Editar</Button>
        <Popconfirm
          title={vocab.deleteTitle}
          description={
            payable.link_count === 0
              ? "Esta ação não pode ser desfeita."
              : `${payable.link_count} transação(ões) vinculada(s) serão desfeitas; as transações em si permanecem inalteradas.`
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
