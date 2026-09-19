import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { InvestmentOperation, InvestmentOperationWrite } from "../../api/contracts";
import { buyOperation, manualAccount, manualPosition } from "../../test/investmentWorkspaceFixtures";
import { InvestmentOperationForm } from "./InvestmentOperationForm";

type Props = Partial<Parameters<typeof InvestmentOperationForm>[0]>;

function show(props: Props = {}) {
  const onSubmit = vi.fn<(write: InvestmentOperationWrite) => void>();
  const build = (overrides: Props) => (
    <InvestmentOperationForm
      open
      operation={null}
      accounts={[manualAccount]}
      positions={[manualPosition]}
      submitting={false}
      submitError={null}
      onSubmit={onSubmit}
      onCancel={() => undefined}
      {...props}
      {...overrides}
    />
  );
  const view = render(build({}));
  return { onSubmit, rerender: (overrides: Props) => view.rerender(build(overrides)) };
}

const tenAtNinetyNine: InvestmentOperation = {
  ...buyOperation,
  quantity: "10",
  unit_price: "99",
  amount: "990.00",
};

describe("InvestmentOperationForm", () => {
  it("keeps what was typed when the caller re-renders with a new initial literal", async () => {
    const user = userEvent.setup();
    const { rerender } = show({ initial: { account_id: manualAccount.id } });

    await user.type(screen.getByLabelText("Valor"), "250");
    await user.type(screen.getByLabelText("Observações (opcional)"), "Aporte de setembro");

    // Same values, fresh objects: what a parent does on every render, and on
    // every workspace refetch — including the one that reports a save error.
    rerender({ initial: { account_id: manualAccount.id }, accounts: [{ ...manualAccount }], submitError: "Caixa ficaria negativo" });

    expect(screen.getByLabelText("Valor")).toHaveValue("250,00");
    expect(screen.getByLabelText("Observações (opcional)")).toHaveValue("Aporte de setembro");
    expect(screen.getByText("Caixa ficaria negativo")).toBeVisible();
  });

  it("omits the amount of a corrected trade so the server prices it again", async () => {
    const user = userEvent.setup();
    const { onSubmit } = show({ operation: tenAtNinetyNine });

    // 990 is exactly 10 × 99, so it was never typed by hand.
    expect(screen.getByLabelText("Valor bruto (opcional)")).toHaveValue("");
    await user.clear(screen.getByLabelText("Quantidade"));
    await user.type(screen.getByLabelText("Quantidade"), "20");
    await user.click(screen.getByRole("button", { name: "Salvar" }));

    expect(onSubmit).toHaveBeenCalledTimes(1);
    const write = onSubmit.mock.calls[0]![0];
    expect(write).toMatchObject({ kind: "buy", quantity: "20", unit_price: "99" });
    expect(write.amount).toBeUndefined();
  });

  it("clears a hand-typed amount when quantity changes, unless the amount was edited this time", async () => {
    const user = userEvent.setup();
    const { onSubmit } = show({ operation: { ...tenAtNinetyNine, amount: "1000.00" } });

    // 1000 differs from 10 × 99, so it is shown as the person's own figure...
    expect(screen.getByLabelText("Valor bruto (opcional)")).toHaveValue("1000,00");
    // ...but a new quantity makes that old total meaningless.
    await user.clear(screen.getByLabelText("Quantidade"));
    await user.type(screen.getByLabelText("Quantidade"), "20");
    expect(screen.getByLabelText("Valor bruto (opcional)")).toHaveValue("");

    await user.type(screen.getByLabelText("Valor bruto (opcional)"), "1995");
    await user.clear(screen.getByLabelText("Preço unitário"));
    await user.type(screen.getByLabelText("Preço unitário"), "100");
    await user.click(screen.getByRole("button", { name: "Salvar" }));

    expect(onSubmit.mock.calls[0]![0]).toMatchObject({ quantity: "20", unit_price: "100", amount: "1995" });
  });
});
