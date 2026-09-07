import { LeftOutlined, PieChartOutlined, RightOutlined } from "@ant-design/icons";
import { Button, Tabs } from "antd";
import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Cell, Pie, PieChart, ResponsiveContainer, Tooltip } from "recharts";

import type { CategoryBreakdown, CategoryDirection, CategorySpendingItem } from "../../api/contracts";
import { useCategoryBreakdown } from "../../hooks/useCategoryBreakdown";
import { currentMonthFilters } from "../../hooks/useTransactions";
import { renderCategoryIcon } from "../../presentation/categoryLabels";
import { formatBRL } from "../../presentation/money";
import { LoadingState, UnavailableState } from "../AsyncState";
import { filtersToSearchParams } from "../filters/filterUrl";
import { WidgetCard } from "../shared/WidgetCard";

const monthFormatter = new Intl.DateTimeFormat("pt-BR", { month: "long", year: "numeric" });
const percentFormatter = new Intl.NumberFormat("pt-BR", { minimumFractionDigits: 1, maximumFractionDigits: 1 });
const palette = ["#2a78d6", "#eb6834", "#17a2b8", "#b54d81", "#8b6cc2", "#8a731b"];

function monthText(date: Date) {
  return `${String(date.getFullYear()).padStart(4, "0")}-${String(date.getMonth() + 1).padStart(2, "0")}`;
}
function monthDate(month: string) {
  const [year, number] = month.split("-").map(Number);
  const date = new Date(0);
  date.setFullYear(year, number - 1, 1);
  date.setHours(12, 0, 0, 0);
  return date;
}
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
function transactionLink(month: string, classification: CategoryDirection, item?: CategorySpendingItem) {
  const params = filtersToSearchParams({
    ...currentMonthFilters(monthDate(month)),
    classification,
    category_id: item?.category_id ?? null,
    uncategorized: item ? item.category_id === null : null,
  });
  return `/transacoes?${params}`;
}

function Breakdown({ data, month, direction }: { data: CategoryBreakdown; month: string; direction: CategoryDirection }) {
  const [active, setActive] = useState<string | null>(null);
  const navigate = useNavigate();
  const total = Number(data.total);
  const label = direction === "outflow" ? "saídas" : "entradas";
  const items = [...data.items].sort((a, b) => Number(b.amount) - Number(a.amount));
  const segments = items.map((item) => ({ ...item, value: Number(item.amount), key: item.category_id ?? "uncategorized" }));

  if (total <= 0 || items.length === 0) {
    return <div className="category-empty"><strong>{formatBRL("0.00")}</strong><p>Nenhuma {direction === "outflow" ? "saída" : "entrada"} neste mês.</p></div>;
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
            onClick={(_, index) => navigate(transactionLink(month, direction, segments[index]))}>
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
        <Link to={transactionLink(month, direction, item)}
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

export function SpendingByCategoryCard() {
  const currentMonth = monthText(new Date());
  const [month, setMonth] = useState(currentMonth);
  const [direction, setDirection] = useState<CategoryDirection>("outflow");
  const { data, isLoading, error, refetch } = useCategoryBreakdown(month, direction);
  function moveMonth(delta: number) {
    const date = monthDate(month);
    date.setMonth(date.getMonth() + delta);
    setMonth(monthText(date));
  }

  return <WidgetCard icon={<PieChartOutlined aria-hidden="true" />} title="Por categoria">
    <Tabs activeKey={direction} onChange={(key) => setDirection(key as CategoryDirection)}
      items={[{ key: "outflow", label: "Saídas" }, { key: "inflow", label: "Entradas" }]} />
    <div className="category-month-navigation">
      <Button type="text" icon={<LeftOutlined />} aria-label="Mês anterior" onClick={() => moveMonth(-1)} disabled={month === "0001-01"} />
      <span aria-live="polite">{monthFormatter.format(monthDate(month))}</span>
      <Button type="text" icon={<RightOutlined />} aria-label="Próximo mês" onClick={() => moveMonth(1)} disabled={month >= currentMonth} />
    </div>
    {month !== currentMonth && <Button className="category-current-month" type="link" size="small" onClick={() => setMonth(currentMonth)}>Voltar ao mês atual</Button>}
    <div aria-live="polite" aria-busy={isLoading}>
      {isLoading ? <LoadingState>Carregando categorias…</LoadingState>
        : error || !data ? <UnavailableState onRetry={() => void refetch()}>Não foi possível carregar as categorias.</UnavailableState>
        : <Breakdown key={`${month}-${direction}`} data={data} month={month} direction={direction} />}
    </div>
    <Link className="category-transactions-link" to={transactionLink(month, direction)}>Ver transações</Link>
  </WidgetCard>;
}
