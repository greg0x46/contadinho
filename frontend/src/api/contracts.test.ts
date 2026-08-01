import { describe, expect, it } from "vitest";

import {
  parseAutomationRule,
  parseAutomationRuleList,
  parseAutomationRuleWriteResult,
  parseCategory,
  parseCategoryList,
  parseDebt,
  parseDebtDetail,
  parseDebtLink,
  parseDebtList,
  parseDebtTotalOwed,
  parseEligibleTransaction,
  parseEligibleTransactionList,
  parseProblem,
  parseSyncRun,
  parseSyncRunDetail,
  parseTransactionCategoryResult,
  parseTransactionInclusionResult,
  parseTransactionQueryResult,
} from "./contracts";
import { syncRun } from "../test/fixtures";
import {
  categoryId,
  ignoredTransactionResult,
  transactionId,
  transactionResult,
} from "../test/transactionFixtures";

describe("runtime contracts", () => {
  it("accepts valid run and detail payloads", () => {
    expect(parseSyncRun(syncRun)).toEqual(syncRun);
    expect(parseSyncRunDetail({ ...syncRun, failures: [] }).failures).toEqual([]);
  });

  it.each([
    { ...syncRun, status: "unknown" },
    { ...syncRun, id: "invalid" },
    { ...syncRun, accounts_processed: -1 },
    { ...syncRun, started_at: "not-a-date" },
  ])("rejects malformed run fields", (payload) => {
    expect(() => parseSyncRun(payload)).toThrow("inválida");
  });

  it("validates problem payloads and active IDs", () => {
    expect(
      parseProblem({ type: "/problem", title: "Conflict", status: 409, active_sync_run_id: syncRun.id }),
    ).toMatchObject({ status: 409, active_sync_run_id: syncRun.id });
    expect(() =>
      parseProblem({ type: "/problem", title: "Conflict", status: 409, active_sync_run_id: "bad" }),
    ).toThrow();
  });
});

describe("transaction inclusion contracts", () => {
  it("accepts considered and ignored confirmed snapshots", () => {
    expect(parseTransactionQueryResult(transactionResult)).toEqual(transactionResult);
    expect(parseTransactionQueryResult(ignoredTransactionResult)).toEqual(
      ignoredTransactionResult,
    );
    expect(
      parseTransactionInclusionResult({
        transaction_id: transactionId,
        state: "ignored",
        changed_at: "2026-07-30T13:00:00Z",
      }),
    ).toMatchObject({ state: "ignored" });
  });

  it.each([
    {
      ...transactionResult,
      items: [{ ...transactionResult.items[0]!, inclusion: { state: "unknown", changed_at: null } }],
    },
    {
      ...transactionResult,
      items: [
        {
          ...transactionResult.items[0]!,
          inclusion: { state: "considered", changed_at: null, revision: 1 },
        },
      ],
    },
    {
      ...transactionResult,
      items: [
        {
          ...transactionResult.items[0]!,
          totals_eligibility: { included: false, reason: "unknown" },
        },
      ],
    },
    {
      ...transactionResult,
      items: [
        {
          ...transactionResult.items[0]!,
          inclusion: { state: "ignored", changed_at: null },
          totals_eligibility: { included: true, reason: null },
        },
      ],
    },
    {
      ...transactionResult,
      items: [
        {
          ...transactionResult.items[0]!,
          totals_eligibility: { included: false, reason: null },
        },
      ],
    },
  ])("rejects unknown inclusion metadata and extra private fields", (payload) => {
    expect(() => parseTransactionQueryResult(payload)).toThrow();
  });

  it("rejects extra or malformed mutation confirmations", () => {
    expect(() =>
      parseTransactionInclusionResult({
        transaction_id: transactionId,
        state: "ignored",
        changed_at: null,
        revision: 1,
      }),
    ).toThrow();
    expect(() =>
      parseTransactionInclusionResult({
        transaction_id: transactionId,
        state: "other",
        changed_at: null,
      }),
    ).toThrow();
  });

  it("parses the inclusion origin and rule name of a ruled-ignored item", () => {
    const result = parseTransactionQueryResult({
      ...ignoredTransactionResult,
      items: [
        {
          ...ignoredTransactionResult.items[0]!,
          inclusion: {
            state: "ignored",
            changed_at: "2026-07-30T13:00:00Z",
            origin: "rule",
            rule_name: "Ignorar assinaturas",
          },
        },
      ],
    });
    expect(result.items[0]!.inclusion).toEqual({
      state: "ignored",
      changed_at: "2026-07-30T13:00:00Z",
      origin: "rule",
      rule_name: "Ignorar assinaturas",
    });
  });

  it.each([
    { ...transactionResult.items[0]!.inclusion, origin: "unknown" },
    { ...transactionResult.items[0]!.inclusion, rule_name: 42 },
  ])("rejects an invalid inclusion origin or rule name", (inclusion) => {
    expect(() =>
      parseTransactionQueryResult({
        ...transactionResult,
        items: [{ ...transactionResult.items[0]!, inclusion }],
      }),
    ).toThrow();
  });
});

describe("automation rule contracts", () => {
  const rule = {
    id: "33333333-3333-4333-8333-333333333333",
    name: "Ignorar taxas",
    is_active: true,
    logic_operator: "or",
    conditions: [{ field: "description", operator: "contains", value: "taxa" }],
    created_at: "2026-07-30T12:00:00Z",
    updated_at: "2026-07-30T12:00:00Z",
  };

  it("accepts a valid rule and rule list", () => {
    expect(parseAutomationRule(rule)).toEqual(rule);
    expect(parseAutomationRuleList([rule])).toEqual([rule]);
  });

  it("accepts account and card condition options", async () => {
    const { parseAutomationRuleConditionOptions } = await import("./contracts");
    expect(
      parseAutomationRuleConditionOptions({
        accounts: ["ultraviolet-black"],
        cards: ["1139", "2848"],
      }),
    ).toEqual({
      accounts: ["ultraviolet-black"],
      cards: ["1139", "2848"],
    });
  });

  it.each([
    { ...rule, name: "" },
    { ...rule, logic_operator: "xor" },
    { ...rule, conditions: [] },
    { ...rule, conditions: [{ field: "description", operator: "contains", value: "" }] },
    { ...rule, conditions: [{ field: "unknown", operator: "contains", value: "x" }] },
  ])("rejects malformed rule fields", (payload) => {
    expect(() => parseAutomationRule(payload)).toThrow();
  });

  it("parses a write result with and without a retroactive apply outcome", () => {
    expect(parseAutomationRuleWriteResult({ rule, retroactive_apply: null })).toEqual({
      rule,
      retroactive_apply: null,
    });
    expect(
      parseAutomationRuleWriteResult({
        rule,
        retroactive_apply: { matched: 3, ignored: 2 },
      }),
    ).toEqual({ rule, retroactive_apply: { matched: 3, ignored: 2 } });
  });
});

describe("debt contracts", () => {
  const debt = {
    id: "44444444-4444-4444-8444-444444444444",
    name: "Financiamento do carro",
    total_amount: "1000",
    starting_paid_amount: "0",
    paid_amount: "200",
    remaining_amount: "800",
    status: "open",
    link_count: 1,
    created_at: "2026-07-30T12:00:00Z",
    updated_at: "2026-07-30T12:00:00Z",
  };

  const link = {
    id: "66666666-6666-4666-8666-666666666666",
    transaction_id: "77777777-7777-4777-8777-777777777777",
    occurred_at: "2026-07-10T12:00:00Z",
    description: "Parcela 1",
    linked_amount: "200",
    current_amount: "200",
    linked_at: "2026-07-30T12:00:00Z",
  };

  const eligibleTransaction = {
    id: "88888888-8888-4888-8888-888888888888",
    occurred_at: "2026-07-20T12:00:00Z",
    description: "Compra elegível",
    account_name: "Conta Corrente",
    effective_money: { value: "150", currency_code: "BRL" },
  };

  it("accepts a valid debt and debt list", () => {
    expect(parseDebt(debt)).toEqual(debt);
    expect(parseDebtList([debt])).toEqual([debt]);
  });

  it("accepts a debt detail with a nullable link description and nullable occurred_at", () => {
    const detail = { ...debt, links: [link, { ...link, description: null, occurred_at: null }] };
    expect(parseDebtDetail(detail)).toEqual(detail);
  });

  it("accepts a debt link write response", () => {
    expect(
      parseDebtLink({
        id: link.id,
        transaction_id: link.transaction_id,
        linked_amount: link.linked_amount,
        linked_at: link.linked_at,
      }),
    ).toEqual({
      id: link.id,
      transaction_id: link.transaction_id,
      linked_amount: link.linked_amount,
      linked_at: link.linked_at,
    });
  });

  it("accepts an eligible transaction and list, with a nullable description and account", () => {
    expect(parseEligibleTransaction(eligibleTransaction)).toEqual(eligibleTransaction);
    expect(parseEligibleTransactionList([eligibleTransaction])).toEqual([eligibleTransaction]);
    expect(
      parseEligibleTransaction({
        ...eligibleTransaction,
        description: null,
        account_name: null,
        occurred_at: null,
      }),
    ).toMatchObject({ description: null, account_name: null, occurred_at: null });
  });

  it.each([
    { ...debt, id: "not-a-uuid" },
    { ...debt, name: "" },
    { ...debt, status: "closed" },
    { ...debt, link_count: -1 },
    { ...debt, total_amount: "1000.0x" },
    { ...debt, extra: "field" },
  ])("rejects malformed debt fields", (payload) => {
    expect(() => parseDebt(payload)).toThrow();
  });

  const debtTotalOwed = {
    remaining_debts_total: "800",
    future_installments_total: "361.49",
    total_owed: "1161.49",
    currency_code: "BRL",
  };

  it("accepts a valid debt total owed", () => {
    expect(parseDebtTotalOwed(debtTotalOwed)).toEqual(debtTotalOwed);
  });

  it.each([
    { ...debtTotalOwed, currency_code: "" },
    { ...debtTotalOwed, total_owed: "not-a-number" },
    { ...debtTotalOwed, extra: "field" },
  ])("rejects malformed debt total owed fields", (payload) => {
    expect(() => parseDebtTotalOwed(payload)).toThrow();
  });

  it.each([
    { ...eligibleTransaction, effective_money: { value: "150" } },
    { ...eligibleTransaction, effective_money: { value: "150", currency_code: "" } },
    { ...eligibleTransaction, id: "not-a-uuid" },
  ])("rejects malformed eligible transaction fields", (payload) => {
    expect(() => parseEligibleTransaction(payload)).toThrow();
  });
});

describe("category contracts", () => {
  const category = {
    id: categoryId,
    name: "Alimentação",
    kind: "expense" as const,
    is_active: true,
    created_at: "2026-07-31T00:00:00Z",
    updated_at: "2026-07-31T00:00:00Z",
  };

  it("accepts a valid category and category list", () => {
    expect(parseCategory(category)).toEqual(category);
    expect(parseCategoryList([category])).toEqual([category]);
  });

  it.each([
    { ...category, name: "" },
    { ...category, kind: "unknown" },
    { ...category, is_active: "yes" },
    { ...category, created_at: "not-a-date" },
  ])("rejects malformed category fields", (payload) => {
    expect(() => parseCategory(payload)).toThrow();
  });

  it("parses a manual transaction category confirmation", () => {
    expect(
      parseTransactionCategoryResult({
        transaction_id: transactionId,
        category_id: categoryId,
        origin: "manual",
        changed_at: "2026-07-31T00:00:00Z",
      }),
    ).toEqual({
      transaction_id: transactionId,
      category_id: categoryId,
      origin: "manual",
      changed_at: "2026-07-31T00:00:00Z",
    });
  });

  it.each([
    {
      transaction_id: transactionId,
      category_id: categoryId,
      origin: "automatic",
      changed_at: "2026-07-31T00:00:00Z",
    },
    {
      transaction_id: transactionId,
      category_id: "not-a-uuid",
      origin: "manual",
      changed_at: "2026-07-31T00:00:00Z",
    },
  ])("rejects an invalid transaction category confirmation", (payload) => {
    expect(() => parseTransactionCategoryResult(payload)).toThrow();
  });

  it("parses a transaction item without an internal category", () => {
    const result = parseTransactionQueryResult({
      ...transactionResult,
      items: [{ ...transactionResult.items[0]!, internal_category: null }],
    });
    expect(result.items[0]!.internal_category).toBeNull();
  });

  it("rejects a malformed internal category", () => {
    expect(() =>
      parseTransactionQueryResult({
        ...transactionResult,
        items: [
          {
            ...transactionResult.items[0]!,
            internal_category: { ...transactionResult.items[0]!.internal_category, kind: "bad" },
          },
        ],
      }),
    ).toThrow();
  });
});
