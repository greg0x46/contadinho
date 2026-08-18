import { List, Progress } from "antd";
import { useNavigate } from "react-router-dom";

import type { CategoryImpact } from "../../api/contracts";
import { filtersToSearchParams } from "../filters/filterUrl";
import { formatBRL } from "../../presentation/money";

// CategoryImpactList never lists its own transactions — clicking a row
// navigates to the existing /transacoes screen with the same filters
// ConfigurableFilters already understands, so there's exactly one place in
// the app that lists transactions. onEvolution (optional) is a second,
// explicit action — "ver evolução" — kept separate from the row click so
// the two intents (drill down to transactions vs. see the trend) never
// collide on the same gesture.
export function CategoryImpactList({
  impacts,
  dateFrom,
  dateTo,
  onEvolution,
}: {
  impacts: CategoryImpact[];
  dateFrom: string;
  dateTo: string;
  onEvolution?: (impact: CategoryImpact) => void;
}) {
  const navigate = useNavigate();

  const open = (impact: CategoryImpact) => {
    const params = filtersToSearchParams({
      date_from: dateFrom,
      date_to: dateTo,
      category_id: impact.category_id ?? "",
      uncategorized: impact.category_id === null,
    });
    navigate(`/transacoes?${params.toString()}`);
  };

  return (
    <List
      aria-label="Impacto por categoria"
      dataSource={impacts}
      locale={{ emptyText: "Sem movimentações no período." }}
      renderItem={(impact) => (
        <List.Item
          className="timeline-category-impact-item"
          onClick={() => open(impact)}
          role="button"
          tabIndex={0}
          onKeyDown={(event) => {
            if (event.key === "Enter") open(impact);
          }}
        >
          <div className="timeline-category-impact-row">
            <span>{impact.category_name}</span>
            <span>
              <strong>{formatBRL(impact.amount)}</strong>
              {onEvolution && (
                <button
                  type="button"
                  className="timeline-scenario-clear"
                  style={{ marginInlineStart: 8 }}
                  onClick={(event) => {
                    event.stopPropagation();
                    onEvolution(impact);
                  }}
                >
                  Ver evolução
                </button>
              )}
            </span>
          </div>
          <Progress percent={Number(impact.percentage)} showInfo={false} size="small" />
        </List.Item>
      )}
    />
  );
}
