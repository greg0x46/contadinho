import type { TransactionQuery, TransactionQueryResult } from "../api/contracts";

export const accountId = "11111111-1111-4111-8111-111111111111";
export const transactionId = "22222222-2222-4222-8222-222222222222";
export const categoryId = "33333333-3333-4333-8333-333333333333";

export const transactionQuery: TransactionQuery = {
  timezone: "America/Sao_Paulo",
  group_by: "month",
  page: 1,
  page_size: 50,
  filters: {
    date_from: "2026-07-01",
    date_to: "2026-07-31",
    description: null,
    account_id: null,
    institution: null,
    category_id: null,
    classification: null,
    provider_status: null,
    amount_min: null,
    amount_max: null,
    uncategorized: null,
  },
};

export const transactionResult: TransactionQueryResult = {
  confirmed_at: "2026-07-30T12:00:00Z",
  stored_total: 1,
  page: { number: 1, size: 50, total_items: 1, total_pages: 1 },
  items: [
    {
      id: transactionId,
      external_id: "provider-transaction",
      occurred_at: "2026-07-15T12:00:00Z",
      description: "Mercado",
      account: {
        id: accountId,
        name: "Conta corrente",
        institution: "Banco Teste",
        currency_code: "BRL",
      },
      source_category: "Alimentação",
      internal_category: {
        id: categoryId,
        name: "Alimentação",
        kind: "expense",
        is_active: true,
        icon: "coffee",
        color: "#eb6834",
        origin: "automatic",
        changed_at: "2026-07-15T12:05:00Z",
      },
      movement_type: "DEBIT",
      provider_status: "POSTED",
      classification: "outflow",
      amount: "123.4500",
      currency_code: "BRL",
      amount_in_account_currency: null,
      effective_money: {
        value: "123.4500",
        currency_code: "BRL",
        source: "transaction_currency",
      },
      card: null,
      inclusion: { state: "considered", changed_at: null, origin: "manual", rule_name: null },
      totals_eligibility: { included: true, reason: null },
      group_key: "month:2026-07",
    },
  ],
  totals: [{ currency_code: "BRL", inflow: "0", outflow: "123.4500", balance: "-123.4500" }],
  groups: [
    {
      key: "month:2026-07",
      kind: "month",
      start_date: "2026-07-01",
      end_date: "2026-07-31",
      item_count: 1,
      page_item_count: 1,
      has_items_before: false,
      has_items_after: false,
      totals: [
        { currency_code: "BRL", inflow: "0", outflow: "123.4500", balance: "-123.4500" },
      ],
    },
  ],
  available_filters: {
    accounts: [{ id: accountId, name: "Conta corrente", institution: "Banco Teste" }],
    institutions: ["Banco Teste"],
    categories: [
      { id: categoryId, name: "Alimentação", kind: "expense", is_active: true, icon: "coffee", color: "#eb6834" },
    ],
  },
};

export const ignoredTransactionResult: TransactionQueryResult = {
  ...transactionResult,
  items: [
    {
      ...transactionResult.items[0]!,
      inclusion: {
        state: "ignored",
        changed_at: "2026-07-30T13:00:00Z",
        origin: "manual",
        rule_name: null,
      },
      totals_eligibility: { included: false, reason: "ignored" },
    },
  ],
  totals: [],
  groups: [{ ...transactionResult.groups[0]!, totals: [] }],
};

export const transactionJsonResponse = (
  body: unknown = transactionResult,
  init: ResponseInit = {},
) =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { "content-type": "application/json" },
    ...init,
  });
