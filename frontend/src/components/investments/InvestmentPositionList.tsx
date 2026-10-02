import { Select, Tag, Tooltip, Typography } from "antd";
import type { ReactNode } from "react";
import { Link } from "react-router-dom";

import type { InvestmentPortfolio, InvestmentPosition } from "../../api/contracts";
import { investmentValuationBasisLabel } from "../../presentation/investmentWorkspaceLabels";
import { formatBRL, formatMoney } from "../../presentation/money";
import { ActionsMenu, type ActionsMenuItem } from "../shared/ActionsMenu";
import { useCompactScreen } from "../shared/useCompactScreen";
import { InvestmentYieldValue } from "./InvestmentFigures";
import { formatDate, formatQuantity, isZeroBRL, positionYield, type LinkedInvestments } from "./investmentFigures";

function YieldCell({
  position,
  linked,
  cost,
}: {
  position: InvestmentPosition;
  linked: LinkedInvestments;
  cost: string;
}) {
  const estimate = positionYield(position, linked, cost);
  if (!estimate.known) {
    return (
      <Tooltip title={estimate.reason}>
        <Typography.Text type="secondary" tabIndex={0}>
          Indisponível
        </Typography.Text>
      </Tooltip>
    );
  }
  return (
    <Tooltip title={`Líquido de IR, IOF e taxas · bruto ${formatBRL(estimate.gross)}`}>
      <span tabIndex={0}>
        <InvestmentYieldValue gain={estimate.value} percent={estimate.percent} />
      </span>
    </Tooltip>
  );
}

function Figure({ label, children, note }: { label: string; children: ReactNode; note?: string }) {
  return (
    <div className="investment-position-figure">
      <dt className="investment-position-figure-label">{label}</dt>
      <dd className="investment-position-figure-value">{children}</dd>
      {note && <dd className="investment-position-figure-note">{note}</dd>}
    </div>
  );
}

/**
 * The positions of an account or goal as flat rows — identity and its "···"
 * on top, the figures under it, the goal picker last — the same recipe as the
 * rows on Contas e cartões. One column on a phone, one dense line from `md`
 * up; nothing scrolls sideways. Figures that are empty for a position
 * (quantity of a fixed-income title, say) are left out instead of shown as
 * "—", so a row only carries what it has.
 */
export function InvestmentPositionList({
  positions,
  portfolios,
  costs,
  linked,
  accountNameOf,
  showAccount = false,
  onAssignGoal,
  onEdit,
  onDelete,
  busy,
  emptyText,
}: {
  positions: InvestmentPosition[];
  portfolios: InvestmentPortfolio[];
  /** position_id -> taxas e impostos lançados nas movimentações dessa posição. */
  costs: Record<string, string>;
  linked: LinkedInvestments;
  accountNameOf?: (accountId: string) => string;
  showAccount?: boolean;
  onAssignGoal: (position: InvestmentPosition, portfolioId: string | null) => void;
  onEdit?: (position: InvestmentPosition) => void;
  onDelete?: (position: InvestmentPosition) => void;
  busy: boolean;
  emptyText: string;
}) {
  const compact = useCompactScreen();
  const goalOptions = portfolios.map((portfolio) => ({ value: portfolio.id, label: portfolio.name }));

  if (positions.length === 0) {
    return <p className="investment-list-empty">{emptyText}</p>;
  }

  return (
    <ul className="investment-position-list" aria-label="Posições">
      {positions.map((position) => {
        const cost = costs[position.id] ?? null;
        // An imported position is the institution's record: only the goal,
        // which is a local grouping, is ours to change.
        const editable = position.source !== "synced";
        const menuItems: ActionsMenuItem[] = [
          ...(editable && onEdit
            ? [{ key: "edit", label: "Editar posição", onClick: () => onEdit(position) }]
            : []),
          ...(editable && onDelete
            ? [
                {
                  key: "delete",
                  label: "Remover posição",
                  danger: true,
                  onClick: () => onDelete(position),
                  confirm: {
                    title: "Remover posição",
                    description: "Só é possível remover uma posição sem movimentações.",
                  },
                },
              ]
            : []),
        ];
        const meta = [
          position.ticker,
          position.asset_type,
          showAccount ? accountNameOf?.(position.account_id) ?? "Conta não encontrada" : null,
          editable ? null : "Informada pela instituição",
        ].filter(Boolean);

        return (
          <li key={position.id} className="investment-position-row">
            <div className="investment-position-identity">
              <span className="investment-position-name">
                {position.linked_investment_id ? (
                  <Link to={`/investimentos/${position.linked_investment_id}`}>{position.name}</Link>
                ) : (
                  position.name
                )}
                {position.closed && <Tag>Encerrada</Tag>}
              </span>
              {meta.length > 0 && <span className="investment-position-meta">{meta.join(" · ")}</span>}
            </div>

            <ActionsMenu
              className="investment-position-menu"
              label={`Ações de ${position.name}`}
              items={menuItems}
            />

            <dl className="investment-position-figures">
              <Figure
                label="Valor atual"
                note={[investmentValuationBasisLabel[position.valuation_basis], position.valued_on && formatDate(position.valued_on)]
                  .filter(Boolean)
                  .join(" · ")}
              >
                {position.currency_code === "BRL"
                  ? formatBRL(position.current_value)
                  : formatMoney(position.current_value, position.currency_code)}
              </Figure>
              <Figure label="Rendimento líquido">
                <YieldCell position={position} linked={linked} cost={cost ?? "0"} />
              </Figure>
              {!isZeroBRL(position.quantity) && <Figure label="Quantidade">{formatQuantity(position.quantity)}</Figure>}
              {!isZeroBRL(position.average_cost) && (
                <Figure label="Custo médio">{formatBRL(position.average_cost as string)}</Figure>
              )}
              {!isZeroBRL(cost) && <Figure label="IR/Taxas">{formatBRL(cost as string)}</Figure>}
            </dl>

            <div className="investment-position-goal">
              <span className="investment-position-goal-label" aria-hidden="true">
                Objetivo
              </span>
              <Select
                aria-label={`Objetivo de ${position.name}`}
                size={compact ? "large" : "middle"}
                value={position.portfolio_id ?? undefined}
                options={goalOptions}
                placeholder="Sem objetivo"
                allowClear
                showSearch
                optionFilterProp="label"
                disabled={busy}
                onChange={(value: string | undefined) => onAssignGoal(position, value ?? null)}
              />
            </div>
          </li>
        );
      })}
    </ul>
  );
}
