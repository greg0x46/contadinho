import { Tag } from "antd";

import type { InvestmentOperation, InvestmentPosition } from "../../api/contracts";
import { investmentOperationKindLabel } from "../../presentation/investmentWorkspaceLabels";
import { formatBRL } from "../../presentation/money";
import { ActionsMenu, type ActionsMenuItem } from "../shared/ActionsMenu";
import { formatDate } from "./investmentFigures";

/**
 * Movimentações as flat rows: what happened and when on the left, the amount
 * beside its "···" on the right, notes under them. Mobile-first — the row is
 * already the phone layout, wider screens just get more room for the notes.
 */
export function InvestmentOperationList({
  operations,
  positions,
  onEdit,
  onDelete,
  busy,
}: {
  operations: InvestmentOperation[];
  positions: InvestmentPosition[];
  onEdit: (operation: InvestmentOperation) => void;
  onDelete: (operation: InvestmentOperation) => void;
  busy: boolean;
}) {
  const positionName = new Map(positions.map((position) => [position.id, position.name]));

  if (operations.length === 0) {
    return <p className="investment-list-empty">Nenhuma movimentação registrada nesta conta.</p>;
  }

  return (
    <ul className="investment-operation-list" aria-label="Movimentações">
      {operations.map((operation) => {
        const menuItems: ActionsMenuItem[] = operation.is_editable
          ? [
              { key: "edit", label: "Corrigir movimentação", disabled: busy, onClick: () => onEdit(operation) },
              {
                key: "delete",
                label: "Excluir movimentação",
                danger: true,
                disabled: busy,
                onClick: () => onDelete(operation),
                confirm: {
                  title: "Excluir movimentação",
                  description: "Os saldos e custos da conta serão recalculados.",
                  okText: "Excluir",
                },
              },
            ]
          : [];
        const where = operation.position_id
          ? positionName.get(operation.position_id) ?? "Posição removida"
          : "Só caixa";

        return (
          <li key={operation.id} className="investment-operation-row">
            <div className="investment-operation-main">
              <span className="investment-operation-kind">
                {investmentOperationKindLabel[operation.kind]}
                {operation.source === "synced" && <Tag color="blue">Informado pela instituição</Tag>}
              </span>
              <span className="investment-operation-meta">
                {[formatDate(operation.occurred_on), where, operation.is_editable ? null : "Somente leitura"]
                  .filter(Boolean)
                  .join(" · ")}
              </span>
              {operation.notes && <span className="investment-operation-notes">{operation.notes}</span>}
            </div>
            <span className="investment-operation-amount">{formatBRL(operation.amount)}</span>
            {/* Reserves the "···" column on read-only rows too, so amounts line up. */}
            <span className="investment-operation-menu-slot">
              <ActionsMenu
                label={`Ações da movimentação de ${formatDate(operation.occurred_on)}`}
                items={menuItems}
              />
            </span>
          </li>
        );
      })}
    </ul>
  );
}
