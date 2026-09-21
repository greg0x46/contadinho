import { PlusOutlined } from "@ant-design/icons";
import { Alert, Button, Skeleton } from "antd";
import { useEffect, useState } from "react";
import { useSearchParams } from "react-router-dom";

import {
  isUuid,
  type ManualTransactionWrite,
  type TransactionFilters,
  type TransactionGrouping,
  type TransactionQueryResult,
} from "../api/contracts";
import { filtersToSearchParams } from "../components/filters/filterUrl";
import { PeriodNavigator } from "../components/filters/PeriodNavigator";
import { periodPresets } from "../components/filters/periodPresets";
import { DataCard, GroupBySelect, Page } from "../components/layout";
import { ManualTransactionForm } from "../components/transactions/ManualTransactionForm";
import { TransactionPanel } from "../components/transactions/TransactionPanel";
import { TransactionFilters as TransactionFilterBar } from "../components/transactions/TransactionFilters";
import { TransactionGroup } from "../components/transactions/TransactionGroup";
import { TransactionSummaryBar } from "../components/transactions/TransactionSummaryBar";
import { useAccounts } from "../hooks/useAccounts";
import { useCategories } from "../hooks/useCategories";
import { useManualTransaction } from "../hooks/useManualTransaction";
import { isValidPeriod, usePeriod, type Period } from "../hooks/usePeriod";
import { currentMonthFilters, useTransactions } from "../hooks/useTransactions";
import { useTransactionCategory } from "../hooks/useTransactionCategory";
import { useTransactionInclusion } from "../hooks/useTransactionInclusion";
import { manualTransactionErrorMessage } from "../presentation/manualTransactionErrors";

type VisibleGrouping = Exclude<TransactionGrouping, "year">;
const visibleGroupings: VisibleGrouping[] = ["none", "day", "week", "month"];
const classifications = ["inflow", "outflow", "unclassified"] as const;

/**
 * A link may carry its own window (`?period=all`, or an explicit
 * `date_from`/`date_to`). When it does, that window wins over the remembered
 * page context; otherwise the page opens on the period the user last chose
 * anywhere in the app.
 */
function periodFromUrl(searchParams: URLSearchParams): Period | null {
  if (searchParams.get("period") === "all") return { from: null, to: null };
  const from = searchParams.get("date_from");
  const to = searchParams.get("date_to");
  if (from === null || to === null) return null;
  if (!/^\d{4}-\d{2}-\d{2}$/.test(from) || !/^\d{4}-\d{2}-\d{2}$/.test(to)) return null;
  const period = { from, to };
  return isValidPeriod(period) ? period : null;
}

function initialState(searchParams: URLSearchParams, period: Period): {
  filters: TransactionFilters;
  groupBy: VisibleGrouping;
  page: number;
} {
  const defaults = currentMonthFilters();
  const value = (key: keyof TransactionFilters) => searchParams.get(key);
  const accountId = value("account_id");
  const categoryId = value("category_id");
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
      card_balance: value("card_balance") === "true" ? true : null,
      credit_card: value("credit_card") === "true" ? true : null,
      date_from: period.from,
      date_to: period.to,
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
      <Skeleton active paragraph={{ rows: 5 }} />
      <span className="visually-hidden">Carregando transações…</span>
    </div>
  );
}

export function TransactionsPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const { period, setPeriod } = usePeriod(() => periodFromUrl(searchParams));
  const [state] = useState(() => initialState(searchParams, period));
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
  const accounts = useAccounts();
  const manualTransaction = useManualTransaction();
  const [manualFormOpen, setManualFormOpen] = useState(false);
  const [manualSaveError, setManualSaveError] = useState<string | null>(null);
  const [manualDeleteError, setManualDeleteError] = useState<string | null>(null);
  const data = query.data;
  const selected = data?.items.find((item) => item.id === selectedId) ?? null;

  const openManualCreate = () => {
    setManualSaveError(null);
    setManualFormOpen(true);
  };
  const closeManualForm = () => setManualFormOpen(false);
  const createManualTransaction = async (write: ManualTransactionWrite) => {
    setManualSaveError(null);
    try {
      await manualTransaction.create(write);
      setManualFormOpen(false);
    } catch (error) {
      setManualSaveError(manualTransactionErrorMessage(error, "save"));
    }
  };
  // Editing happens inside the transaction panel; it shows the message itself.
  const updateManualTransaction = async (transactionId: string, write: ManualTransactionWrite) => {
    try {
      await manualTransaction.update({ transactionId, write });
    } catch (error) {
      throw new Error(manualTransactionErrorMessage(error, "save"));
    }
  };
  const deleteManualTransactionAndClose = async (transactionId: string) => {
    setManualDeleteError(null);
    try {
      await manualTransaction.remove(transactionId);
      setSelectedId(null);
    } catch (error) {
      setManualDeleteError(manualTransactionErrorMessage(error, "delete"));
    }
  };
  const selectTransaction = (transactionId: string | null) => {
    setManualDeleteError(null);
    setSelectedId(transactionId);
  };

  useEffect(() => {
    if (data) setFacets(data.available_filters);
  }, [data]);

  useEffect(() => {
    const params = filtersToSearchParams(filters);
    if (filters.date_from === null && filters.date_to === null) params.set("period", "all");
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
  // The period is page context, not a list filter: it changes the list and
  // the totals together and is remembered for the next page that reads it.
  const applyPeriod = (from: string | null, to: string | null) => {
    setPeriod(from, to);
    apply({ ...filters, date_from: from, date_to: to });
  };
  const clear = () => apply({ ...initialFilters, date_from: filters.date_from, date_to: filters.date_to });
  const hasExtraFilters = Object.entries(filters).some(([key, value]) => !key.startsWith("date_") && value !== null);
  const isInitialMonth =
    filters.date_from === initialFilters.date_from &&
    filters.date_to === initialFilters.date_to &&
    !hasExtraFilters;
  const rangeStart = data ? (data.page.number - 1) * data.page.size + 1 : 0;
  const rangeEnd = data
    ? Math.min(data.page.number * data.page.size, data.page.total_items)
    : 0;

  return (
    <Page
      title="Transações"
      description="Acompanhe suas entradas, saídas e o resultado do período"
      className="transactions-page"
      context={
        <PeriodNavigator
          id="transactions-period"
          value={[filters.date_from, filters.date_to]}
          presets={periodPresets()}
          onChange={applyPeriod}
          reset={{ preset: "this-month", label: "Este mês" }}
          bare
        />
      }
      actions={
        <Button type="primary" icon={<PlusOutlined aria-hidden="true" />} onClick={openManualCreate}>
          Novo lançamento
        </Button>
      }
    >
      <TransactionFilterBar
        applied={filters}
        emptyValues={initialFilters}
        facets={data?.available_filters ?? facets}
        onApply={apply}
        onClear={clear}
        end={
          <GroupBySelect
            id="transaction-grouping"
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
        }
      />

      <DataCard
        className="transactions-card"
        summary={
          data ? (
            <TransactionSummaryBar
              totalItems={data.page.total_items}
              totals={data.page.total_items === 0 ? [] : data.totals}
              busy={query.isFetching}
            />
          ) : query.isPending && query.timezoneValid ? (
            <section className="data-card-summary" aria-label="Carregando resumo financeiro" aria-busy="true">
              <Skeleton active paragraph={{ rows: 1 }} title={false} />
            </section>
          ) : undefined
        }
      >
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
                      : hasExtraFilters
                        ? "Nenhum resultado encontrado para os filtros selecionados."
                        : "Não há transações no período selecionado."
                }
                description={
                  data.stored_total > 0 && hasExtraFilters ? (
                    <Button type="link" onClick={clear}>
                      Limpar filtros
                    </Button>
                  ) : undefined
                }
              />
            ) : (
              <>
                <div className="transaction-groups">
                  {data.groups.map((group) => (
                    <TransactionGroup
                      key={group.key}
                      group={group}
                      items={data.items.filter((item) => item.group_key === group.key)}
                      selectedId={selectedId}
                      onSelect={selectTransaction}
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
      </DataCard>
      <TransactionPanel
        item={selected}
        categories={categories.categories}
        accounts={accounts.accounts}
        onClose={() => selectTransaction(null)}
        onInclusion={(transactionId, target) =>
          inclusion.setInclusion({ transactionId, state: target })
        }
        inclusionPending={inclusion.pendingTarget?.transactionId === selected?.id}
        onCategory={(transactionId, categoryId) => category.setCategory({ transactionId, categoryId })}
        categoryPending={category.pendingTarget?.transactionId === selected?.id}
        onSaveManual={updateManualTransaction}
        saveManualPending={manualTransaction.isUpdating}
        onDeleteManual={deleteManualTransactionAndClose}
        deleteManualPending={manualTransaction.isRemoving}
        deleteManualError={manualDeleteError}
      />
      <ManualTransactionForm
        open={manualFormOpen}
        transaction={null}
        accounts={accounts.accounts}
        categories={categories.categories}
        submitting={manualTransaction.isCreating}
        submitError={manualSaveError}
        onSubmit={createManualTransaction}
        onCancel={closeManualForm}
      />
    </Page>
  );
}
