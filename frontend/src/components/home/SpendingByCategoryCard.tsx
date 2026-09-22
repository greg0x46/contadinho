import { PieChartOutlined } from "@ant-design/icons";
import { Tabs } from "antd";
import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Cell, Pie, PieChart, ResponsiveContainer, Tooltip } from "recharts";

import type { CategoryBreakdown, CategoryDirection, CategorySpendingItem } from "../../api/contracts";
import { useCategoryBreakdown } from "../../hooks/useCategoryBreakdown";
import type { Period } from "../../hooks/usePeriod";
import { renderCategoryIcon } from "../../presentation/categoryLabels";
import { formatBRL } from "../../presentation/money";
import { LoadingState, UnavailableState } from "../AsyncState";
import { filtersToSearchParams } from "../filters/filterUrl";
import { WidgetCard } from "../shared/WidgetCard";

const percentFormatter = new Intl.NumberFormat("pt-BR", { minimumFractionDigits: 1, maximumFractionDigits: 1 });
const palette = ["#2a78d6", "#eb6834", "#17a2b8", "#b54d81", "#8b6cc2", "#8a731b"];

function categoryColor(item: CategorySpendingItem) {
  if (!item.category_id) return "#898781";
  let hash = 0;
  for (const char of item.category_id) hash = (hash * 31 + char.charCodeAt(0)) >>> 0;
  return item.category_color || palette[hash % palette.length];
}
function percent(amount: number, total: number) {
  const value = total > 0 ? amount / total * 100 : 0;
  return value > 0 && value < 0.1 ? "<0,1%" : `${percentFormatter.format(value)}%`;
}
function transactionLink(period: Period, classification: CategoryDirection, item?: CategorySpendingItem) {
  const params = filtersToSearchParams({
    ...(period.from === null ? { period: "all" } : { date_from: period.from, date_to: period.to }),
    classification,
    category_ids: item?.category_id ? [item.category_id] : [],
    uncategorized: item ? item.category_id === null : null,
  });
  return `/transacoes?${params}`;
}

function Breakdown({ data, period, direction }: { data: CategoryBreakdown; period: Period; direction: CategoryDirection }) {
  const [active, setActive] = useState<string | null>(null);
  const navigate = useNavigate();
  const total = Number(data.total);
  const label = direction === "outflow" ? "saídas" : "entradas";
  const items = [...data.items].sort((a, b) => Number(b.amount) - Number(a.amount));
  const segments = items.map((item) => ({ ...item, value: Number(item.amount), key: item.category_id ?? "uncategorized" }));

  if (total <= 0 || items.length === 0) {
    return <div className="category-empty"><strong>{formatBRL("0.00")}</strong><p>Nenhuma {direction === "outflow" ? "saída" : "entrada"} neste período.</p></div>;
  }

  return <div className="category-breakdown">
    <div className="category-donut">
      <div className="category-donut-total">
        <span>Total de {label}</span>
        <strong title={formatBRL(data.total)}>{formatBRL(data.total)}</strong>
      </div>
      <ResponsiveContainer width="100%" height={220}>
        <PieChart>
          <Pie data={segments} dataKey="value" nameKey="category_name" innerRadius="72%" outerRadius="95%"
            isAnimationActive={false} strokeWidth={1}
            onMouseEnter={(_, index) => setActive(segments[index].key)}
            onMouseLeave={() => setActive(null)}
            onClick={(_, index) => navigate(transactionLink(period, direction, segments[index]))}>
            {segments.map((item) => <Cell key={item.key} fill={categoryColor(item)}
              opacity={active === null || active === item.key ? 1 : 0.35} style={{ cursor: "pointer" }} />)}
          </Pie>
          <Tooltip content={({ active: visible, payload }) => {
            const item = payload?.[0]?.payload as (typeof segments)[number] | undefined;
            return visible && item ? <div className="timeline-chart-tooltip">
              <strong>{item.category_name}</strong>
              <div>{formatBRL(item.amount)} · {percent(item.value, total)}</div>
            </div> : null;
          }} />
        </PieChart>
      </ResponsiveContainer>
    </div>
    <ul className="category-breakdown-list" aria-label={`Categorias de ${label}`}>
      {segments.map((item) => <li key={item.key} data-active={active === item.key}>
        <Link to={transactionLink(period, direction, item)}
          onFocus={() => setActive(item.key)} onBlur={() => setActive(null)}
          onMouseEnter={() => setActive(item.key)} onMouseLeave={() => setActive(null)}>
          <span className="category-breakdown-icon" style={{ color: categoryColor(item) }} aria-hidden="true">
            {item.category_icon ? renderCategoryIcon(item.category_icon) : <span style={{ background: categoryColor(item) }} />}
          </span>
          <span className="category-breakdown-name">{item.category_name}</span>
          <span className="category-breakdown-amount"><strong>{formatBRL(item.amount)}</strong><span>{percent(item.value, total)}</span></span>
        </Link>
      </li>)}
    </ul>
  </div>;
}

export function SpendingByCategoryCard({ period }: { period: Period }) {
  const [direction, setDirection] = useState<CategoryDirection>("outflow");
  const { data, isLoading, error, refetch } = useCategoryBreakdown(period, direction);

  return <WidgetCard icon={<PieChartOutlined aria-hidden="true" />} title="Por categoria">
    <Tabs activeKey={direction} onChange={(key) => setDirection(key as CategoryDirection)}
      items={[{ key: "outflow", label: "Saídas" }, { key: "inflow", label: "Entradas" }]} />
    <div aria-live="polite" aria-busy={isLoading}>
      {isLoading ? <LoadingState>Carregando categorias…</LoadingState>
        : error || !data ? <UnavailableState onRetry={() => void refetch()}>Não foi possível carregar as categorias.</UnavailableState>
        : <Breakdown key={`${period.from}-${period.to}-${direction}`} data={data} period={period} direction={direction} />}
    </div>
    <Link className="category-transactions-link" to={transactionLink(period, direction)}>Ver transações</Link>
  </WidgetCard>;
}
