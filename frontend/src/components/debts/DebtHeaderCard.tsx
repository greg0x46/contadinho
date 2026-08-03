import { Button, Card, Popconfirm, Tag, Typography } from "antd";

import type { Debt } from "../../api/contracts";
import { debtStatusColor, debtStatusLabel } from "../../presentation/debtLabels";
import { formatBRL } from "../../presentation/money";

export function DebtHeaderCard({
  debt,
  onEdit,
  onDelete,
}: {
  debt: Debt;
  onEdit: () => void;
  onDelete: () => void;
}) {
  const total = Number(debt.total_amount);
  const paid = Number(debt.paid_amount);
  const paidShare = total > 0 ? Math.min(100, Math.max(0, (paid / total) * 100)) : 0;

  return (
    <Card className="debt-header-card dashboard-widget">
      <div className="debt-header-identity">
        <Typography.Title level={3} className="debt-header-name">
          {debt.name}
        </Typography.Title>
        <Tag color={debtStatusColor[debt.status]}>{debtStatusLabel[debt.status]}</Tag>
      </div>

      <div className="debt-header-figure">
        <p className="dashboard-hero-figure">{formatBRL(debt.remaining_amount)} restantes</p>
        <div className="dashboard-meter" aria-hidden="true">
          <span
            className="dashboard-meter-segment debt-row-meter-paid"
            style={{ width: `${paidShare}%` }}
          />
          <span
            className="dashboard-meter-segment debt-row-meter-remaining"
            style={{ width: `${100 - paidShare}%` }}
          />
        </div>
        <ul className="debt-header-legend-inline">
          <li>
            <span>Pago</span>
            <strong>{formatBRL(debt.paid_amount)}</strong>
          </li>
          <li>
            <span>Total</span>
            <strong>{formatBRL(debt.total_amount)}</strong>
          </li>
        </ul>
      </div>

      <div className="debt-header-actions">
        <Button onClick={onEdit}>Editar</Button>
        <Popconfirm
          title="Excluir dívida"
          description={
            debt.link_count === 0
              ? "Esta ação não pode ser desfeita."
              : `${debt.link_count} transação(ões) vinculada(s) serão desfeitas; as transações em si permanecem inalteradas.`
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
