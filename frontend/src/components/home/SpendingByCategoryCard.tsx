import { PieChartOutlined } from "@ant-design/icons";
import { Card } from "antd";
import { Link } from "react-router-dom";

import type { CategorySpendingItem, SpendingByCategory } from "../../api/contracts";
import { currentMonthFilters } from "../../hooks/useTransactions";
import { useSpendingByCategory } from "../../hooks/useSpendingByCategory";
import { formatBRL } from "../../presentation/money";
import { LoadingState, UnavailableState } from "../AsyncState";
import { filtersToSearchParams } from "../filters/filterUrl";

const MAX_VISIBLE_CATEGORIES = 5;
const SWATCH_CLASSES = [
  "spending-by-category-swatch-1",
  "spending-by-category-swatch-2",
  "spending-by-category-swatch-3",
  "spending-by-category-swatch-4",
  "spending-by-category-swatch-5",
];

const monthFormatter = new Intl.DateTimeFormat("pt-BR", { month: "long", year: "numeric" });

function monthLabel(month: string): string {
  const [year, monthNumber] = month.split("-").map(Number);
  return monthFormatter.format(new Date(year, monthNumber - 1, 1));
}

interface Segment {
  key: string;
  name: string;
  amount: number;
  swatchClass: string;
}

function buildSegments(items: CategorySpendingItem[]): Segment[] {
  const segments = items.slice(0, MAX_VISIBLE_CATEGORIES).map((item, index) => ({
    key: item.category_id ?? "uncategorized",
    name: item.category_name,
    amount: Number(item.amount),
    swatchClass: SWATCH_CLASSES[index],
  }));

  const rest = items.slice(MAX_VISIBLE_CATEGORIES);
  if (rest.length > 0) {
    segments.push({
      key: "other",
      name: "Outras categorias",
      amount: rest.reduce((sum, item) => sum + Number(item.amount), 0),
      swatchClass: "spending-by-category-swatch-other",
    });
  }
  return segments;
}

function SpendingByCategoryBreakdown({ spending }: { spending: SpendingByCategory }) {
  const total = Number(spending.total);

  if (spending.items.length === 0 || total <= 0) {
    return (
      <div className="spending-by-category-body">
        <p className="dashboard-hero-figure">{formatBRL(spending.total)}</p>
        <p className="dashboard-empty">Nenhum gasto categorizado neste mês.</p>
      </div>
    );
  }

  const segments = buildSegments(spending.items);

  return (
    <div className="spending-by-category-body">
      <p className="dashboard-hero-figure">{formatBRL(spending.total)}</p>
      <p className="spending-by-category-subtitle">{monthLabel(spending.month)}</p>
      <div className="dashboard-meter" aria-hidden="true">
        {segments.map((segment) => (
          <span
            key={segment.key}
            className={`dashboard-meter-segment ${segment.swatchClass}`}
            style={{ width: `${(segment.amount / total) * 100}%` }}
          />
        ))}
      </div>
      <ul className="dashboard-legend">
        {segments.map((segment) => (
          <li key={segment.key}>
            <span className={`dashboard-swatch ${segment.swatchClass}`} aria-hidden="true" />
            <span className="dashboard-legend-label">{segment.name}</span>
            <span className="spending-by-category-legend-amount">
              <strong>{formatBRL(segment.amount.toFixed(2))}</strong>
              <span className="spending-by-category-legend-percent">
                {Math.round((segment.amount / total) * 100)}%
              </span>
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}

export function SpendingByCategoryCard() {
  const { spending, isLoading, error, refetch } = useSpendingByCategory();

  const params = filtersToSearchParams({
    ...currentMonthFilters(),
    classification: "outflow",
  });

  return (
    <Card
      className="dashboard-widget"
      title={
        <span className="dashboard-widget-title">
          <PieChartOutlined aria-hidden="true" />
          Gastos por categoria
        </span>
      }
      extra={<Link to={`/transacoes?${params.toString()}`}>Ver transações</Link>}
    >
      {isLoading && <LoadingState>Carregando gastos por categoria…</LoadingState>}
      {!isLoading && (error || !spending) && (
        <UnavailableState onRetry={refetch}>
          Não foi possível carregar os gastos por categoria.
        </UnavailableState>
      )}
      {!isLoading && !error && spending && <SpendingByCategoryBreakdown spending={spending} />}
    </Card>
  );
}
