import { ReloadOutlined } from "@ant-design/icons";
import { PageContainer } from "@ant-design/pro-layout";
import { Alert, Button, Card, Select, Skeleton } from "antd";
import { useEffect, useState } from "react";
import { useSearchParams } from "react-router-dom";

import {
  isUuid,
  type TransactionFilters,
  type TransactionGrouping,
  type TransactionQueryResult,
} from "../api/contracts";
import { filtersToSearchParams } from "../components/filters/filterUrl";
import { TransactionDetailDrawer } from "../components/transactions/TransactionDetailDrawer";
import { TransactionFilters as TransactionFilterBar } from "../components/transactions/TransactionFilters";
import { TransactionGroup } from "../components/transactions/TransactionGroup";
import { TransactionTotals } from "../components/transactions/TransactionTotals";
import { useCategories } from "../hooks/useCategories";
import { currentMonthFilters, useTransactions } from "../hooks/useTransactions";
import { useTransactionCategory } from "../hooks/useTransactionCategory";
import { useTransactionInclusion } from "../hooks/useTransactionInclusion";

type VisibleGrouping = Exclude<TransactionGrouping, "year">;
const visibleGroupings: VisibleGrouping[] = ["none", "day", "week", "month"];
const classifications = ["inflow", "outflow", "unclassified"] as const;

function initialState(searchParams: URLSearchParams): {
  filters: TransactionFilters;
  groupBy: VisibleGrouping;
  page: number;
} {
  const defaults = currentMonthFilters();
  const value = (key: keyof TransactionFilters) => searchParams.get(key);
  const dateFrom = value("date_from");
  const dateTo = value("date_to");
  const accountId = value("account_id");
  const categoryId = value("category_id");
  const hasValidDateRange =
    dateFrom !== null &&
    dateTo !== null &&
    /^\d{4}-\d{2}-\d{2}$/.test(dateFrom) &&
    /^\d{4}-\d{2}-\d{2}$/.test(dateTo) &&
    dateFrom <= dateTo;
  const classification = value("classification");
  const status = value("provider_status");
  const amountPattern = /^(0|[1-9]\d*)(\.\d+)?$/;
  const amountMin = value("amount_min");
  const amountMax = value("amount_max");
  const group = searchParams.get("group");
  const requestedPage = Number(searchParams.get("page"));
  return {
    filters: {
      ...defaults,
      date_from: hasValidDateRange ? dateFrom : defaults.date_from,
      date_to: hasValidDateRange ? dateTo : defaults.date_to,
      description: value("description"),
      account_id: accountId && isUuid(accountId) ? accountId : null,
      institution: value("institution"),
      category_id: categoryId && isUuid(categoryId) ? categoryId : null,
      classification: classifications.includes(
        classification as (typeof classifications)[number],
      )
        ? (classification as TransactionFilters["classification"])
        : null,
      provider_status:
        status === "POSTED" || status === "PENDING" ? status : null,
      amount_min: amountMin && amountPattern.test(amountMin) ? amountMin : null,
      amount_max: amountMax && amountPattern.test(amountMax) ? amountMax : null,
      uncategorized: value("uncategorized") === "true" ? true : null,
    },
    groupBy: visibleGroupings.includes(group as VisibleGrouping)
      ? (group as VisibleGrouping)
      : "week",
    page: Number.isInteger(requestedPage) && requestedPage > 0 ? requestedPage : 1,
  };
}

function ResultsSkeleton() {
  return (
    <div className="transaction-results-skeleton" role="status" aria-label="Carregando transações">
      <section className="transaction-summary" aria-hidden="true">
        {[0, 1, 2].map((key) => (
          <Card key={key} size="small">
            <Skeleton active paragraph={false} title={{ width: "70%" }} />
          </Card>
        ))}
      </section>
      <Skeleton active paragraph={{ rows: 5 }} />
      <span className="visually-hidden">Carregando transações…</span>
    </div>
  );
}

export function TransactionsPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [state] = useState(() => initialState(searchParams));
  const [initialFilters] = useState(() => currentMonthFilters());
  const [filters, setFilters] = useState<TransactionFilters>(state.filters);
  const [groupBy, setGroupBy] = useState<VisibleGrouping>(state.groupBy);
  const [page, setPage] = useState(state.page);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [facets, setFacets] =
    useState<TransactionQueryResult["available_filters"]>();
  const query = useTransactions(filters, groupBy, page);
  const inclusion = useTransactionInclusion();
  const category = useTransactionCategory();
  const categories = useCategories();
  const data = query.data;
  const selected = data?.items.find((item) => item.id === selectedId) ?? null;

  useEffect(() => {
    if (data) setFacets(data.available_filters);
  }, [data]);

  useEffect(() => {
    const params = filtersToSearchParams(filters);
    if (groupBy !== "week") params.set("group", groupBy);
    if (page > 1) params.set("page", String(page));
    if (params.toString() !== searchParams.toString()) {
      setSearchParams(params, { replace: true });
    }
  }, [filters, groupBy, page, searchParams, setSearchParams]);

  const apply = (next: TransactionFilters) => {
    setFilters(next);
    setPage(1);
    setSelectedId(null);
  };
  const clear = () => apply(initialFilters);
  const isInitialMonth =
    filters.date_from === initialFilters.date_from &&
    filters.date_to === initialFilters.date_to &&
    Object.entries(filters).every(
      ([key, value]) => key.startsWith("date_") || value === null,
    );
  const rangeStart = data ? (data.page.number - 1) * data.page.size + 1 : 0;
  const rangeEnd = data
    ? Math.min(data.page.number * data.page.size, data.page.total_items)
    : 0;

  return (
    <PageContainer title={false} className="transactions-page">
      <header className="transactions-header">
        <div>
          <h1>Transações</h1>
          <p>Acompanhe suas movimentações e o saldo do período.</p>
        </div>
        <Button
          icon={<ReloadOutlined aria-hidden="true" />}
          loading={query.isFetching}
          disabled={!query.timezoneValid}
          onClick={() => query.refetch()}
        >
          Atualizar
        </Button>
      </header>

      <TransactionFilterBar
        applied={filters}
        emptyValues={initialFilters}
        facets={data?.available_filters ?? facets}
        onApply={apply}
        onClear={clear}
      />

      {!query.timezoneValid && (
        <Alert
          type="error"
          showIcon
          message="Não foi possível identificar um fuso horário IANA válido."
        />
      )}
      {query.isPending && query.timezoneValid && <ResultsSkeleton />}
      {query.isError && !data && (
        <Alert
          type="error"
          showIcon
          message="Não foi possível carregar as transações"
          description={<Button onClick={() => query.refetch()}>Tentar novamente</Button>}
        />
      )}
      {query.isError && data && (
        <Alert
          type="warning"
          showIcon
          message="Dados possivelmente desatualizados"
          description={<Button onClick={() => query.refetch()}>Tentar novamente</Button>}
        />
      )}
      {inclusion.writeError && (
        <Alert
          type="error"
          showIcon
          message="Não foi possível salvar a decisão"
          description={
            <>
              <p>O último estado confirmado foi mantido. {inclusion.writeError}</p>
              <Button onClick={inclusion.retryWrite}>Tentar novamente</Button>
            </>
          }
        />
      )}
      {inclusion.refreshError && (
        <Alert
          type="warning"
          showIcon
          message="Alteração salva, atualização pendente"
          description={
            <>
              <p>Os resultados anteriores foram preservados. {inclusion.refreshError}</p>
              <Button onClick={inclusion.retryRefresh}>Atualizar resultados</Button>
            </>
          }
        />
      )}
      {category.writeError && (
        <Alert
          type="error"
          showIcon
          message="Não foi possível salvar a categoria"
          description={
            <>
              <p>A última categoria confirmada foi mantida. {category.writeError}</p>
              <Button onClick={category.retryWrite}>Tentar novamente</Button>
            </>
          }
        />
      )}
      {category.refreshError && (
        <Alert
          type="warning"
          showIcon
          message="Categoria salva, atualização pendente"
          description={
            <>
              <p>Os resultados anteriores foram preservados. {category.refreshError}</p>
              <Button onClick={category.retryRefresh}>Atualizar resultados</Button>
            </>
          }
        />
      )}
      <div className="visually-hidden" aria-live="polite" aria-atomic="true">
        {inclusion.announcement}
      </div>
      <div className="visually-hidden" aria-live="polite" aria-atomic="true">
        {category.announcement}
      </div>

      {data && (
        <div
          className={`transaction-results ${query.isFetching ? "is-updating" : ""}`}
          aria-busy={query.isFetching}
        >
          {query.isFetching && !query.isPending && (
            <span className="visually-hidden" role="status">
              Atualizando resultados…
            </span>
          )}
          {data.page.total_items === 0 ? (
            <Alert
              type="info"
              message={
                data.stored_total === 0
                  ? "Ainda não há transações armazenadas."
                  : isInitialMonth
                    ? "Não há transações no mês atual."
                    : "Nenhum resultado encontrado para os filtros selecionados."
              }
              description={
                data.stored_total > 0 && !isInitialMonth ? (
                  <Button type="link" onClick={clear}>
                    Limpar filtros
                  </Button>
                ) : undefined
              }
            />
          ) : (
            <>
              <TransactionTotals totals={data.totals} />
              <p className="transaction-inclusion-explanation">
                Transações ignoradas permanecem na lista e não entram nos totais.
              </p>
              <div className="transaction-list-controls">
                <strong>
                  {data.page.total_items.toLocaleString("pt-BR")}{" "}
                  {data.page.total_items === 1 ? "transação" : "transações"}
                </strong>
                <label htmlFor="transaction-grouping">Agrupar por:</label>
                <Select
                  id="transaction-grouping"
                  aria-label="Agrupar por"
                  size="small"
                  value={groupBy}
                  options={[
                    { value: "none", label: "Sem agrupamento" },
                    { value: "day", label: "Dia" },
                    { value: "week", label: "Semana" },
                    { value: "month", label: "Mês" },
                  ]}
                  onChange={(value: VisibleGrouping) => {
                    setGroupBy(value);
                    setPage(1);
                  }}
                />
              </div>
              <div className="transaction-groups">
                {data.groups.map((group) => (
                  <TransactionGroup
                    key={group.key}
                    group={group}
                    items={data.items.filter((item) => item.group_key === group.key)}
                    onSelect={setSelectedId}
                    onInclusion={(transactionId, target) =>
                      inclusion.setInclusion({ transactionId, state: target })
                    }
                    pendingTransactionId={inclusion.pendingTarget?.transactionId}
                  />
                ))}
              </div>
              <footer className="transaction-pagination">
                <span>
                  Exibindo {rangeStart.toLocaleString("pt-BR")}–
                  {rangeEnd.toLocaleString("pt-BR")} de{" "}
                  {data.page.total_items.toLocaleString("pt-BR")} transações
                </span>
                <nav aria-label="Paginação das transações">
                  <Button
                    disabled={data.page.number <= 1}
                    onClick={() => setPage((current) => current - 1)}
                  >
                    Anterior
                  </Button>
                  <span>
                    Página {data.page.number} de {data.page.total_pages}
                  </span>
                  <Button
                    disabled={data.page.number >= data.page.total_pages}
                    onClick={() => setPage((current) => current + 1)}
                  >
                    Próxima
                  </Button>
                </nav>
              </footer>
            </>
          )}
        </div>
      )}
      <TransactionDetailDrawer
        item={selected}
        categories={categories.categories}
        onClose={() => setSelectedId(null)}
        onInclusion={(transactionId, target) =>
          inclusion.setInclusion({ transactionId, state: target })
        }
        inclusionPending={inclusion.pendingTarget?.transactionId === selected?.id}
        onCategory={(transactionId, categoryId) => category.setCategory({ transactionId, categoryId })}
        categoryPending={category.pendingTarget?.transactionId === selected?.id}
      />
    </PageContainer>
  );
}
