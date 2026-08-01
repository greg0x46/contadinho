import { useQuery } from "@tanstack/react-query";

import { queryTransactions } from "../api/transactions";
import type {
  TransactionFilters,
  TransactionGrouping,
  TransactionQuery,
} from "../api/contracts";

export const DEFAULT_PAGE_SIZE = 50;

function dateText(date: Date): string {
  const year = String(date.getFullYear()).padStart(4, "0");
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

export function currentMonthFilters(now = new Date()): TransactionFilters {
  const start = new Date(now.getFullYear(), now.getMonth(), 1);
  const end = new Date(now.getFullYear(), now.getMonth() + 1, 0);
  return {
    date_from: dateText(start),
    date_to: dateText(end),
    description: null,
    account_id: null,
    institution: null,
    category_id: null,
    classification: null,
    provider_status: null,
    amount_min: null,
    amount_max: null,
    uncategorized: null,
  };
}

export function browserTimezone(): string | null {
  const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone;
  return timezone?.trim() ? timezone : null;
}

export function transactionQueryKey(query: TransactionQuery) {
  return ["transactions", query] as const;
}

export function useTransactions(
  filters: TransactionFilters,
  groupBy: TransactionGrouping,
  page: number,
  pageSize = DEFAULT_PAGE_SIZE,
) {
  const timezone = browserTimezone();
  const query: TransactionQuery | null =
    timezone === null
      ? null
      : { timezone, group_by: groupBy, page, page_size: pageSize, filters };
  const result = useQuery({
    queryKey: query === null ? ["transactions", "invalid-timezone"] : transactionQueryKey(query),
    queryFn: ({ signal }) => queryTransactions(query!, signal),
    enabled: query !== null,
    retry: false,
  });
  return { ...result, query, timezoneValid: timezone !== null };
}
