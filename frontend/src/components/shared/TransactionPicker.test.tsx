import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { TransactionPicker, type PickerTransaction } from "./TransactionPicker";

const transactions: PickerTransaction[] = [
  {
    id: "big",
    description: "Resgate CDB - Nu Financeira",
    amount: "1630.04",
    direction: "inflow",
    date: "2026-09-13",
    account: "Nubank",
    category: "Investimentos",
  },
  {
    id: "small",
    description: "Resgate CDB - Nu Financeira",
    amount: "750.00",
    direction: "inflow",
    date: "2026-09-13",
    account: "Nubank",
    category: "Investimentos",
    tag: "Sugestão",
  },
  {
    id: "income",
    description: "Rendimento Ação",
    amount: "123.21",
    direction: "inflow",
    date: "2026-09-08T12:00:00Z",
    account: "Corretora XP",
    category: "Rendimentos",
  },
];

function compactViewport() {
  // Every media query fails → antd reports no breakpoint → compact.
  vi.mocked(window.matchMedia).mockImplementation((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }));
}

function openMenu(): HTMLElement {
  return document.querySelector(".ant-select-dropdown:not(.ant-select-dropdown-hidden)") as HTMLElement;
}

function rowsIn(container: HTMLElement): HTMLElement[] {
  return Array.from(container.querySelectorAll<HTMLElement>(".txn-picker-row"));
}

function renderPicker(props: Partial<Parameters<typeof TransactionPicker>[0]> = {}) {
  const onSelect = vi.fn();
  render(
    <TransactionPicker
      id="picker"
      label="Movimentação"
      value={null}
      transactions={transactions}
      onSelect={onSelect}
      {...props}
    />,
  );
  return { onSelect };
}

describe("TransactionPicker", () => {
  it("renders description, amount and context per row on a wide viewport and picks one", async () => {
    const user = userEvent.setup();
    const { onSelect } = renderPicker();

    await user.click(screen.getByRole("combobox", { name: "Movimentação" }));
    const rows = rowsIn(openMenu());
    expect(rows).toHaveLength(3);
    expect(rows[0]).toHaveTextContent("Resgate CDB - Nu Financeira");
    expect(rows[0]).toHaveTextContent("+R$ 1.630,04");
    expect(rows[0]).toHaveTextContent("13 set. · Nubank · Investimentos");
    expect(rows[1]).toHaveTextContent("+R$ 750,00");
    expect(within(rows[1]!).getByText("Sugestão")).toBeInTheDocument();

    await user.click(within(rows[1]!).getByText("+R$ 750,00"));
    expect(onSelect).toHaveBeenCalledWith("small", transactions[1]);
  });

  it("searches by amount, account and accent-insensitive description", async () => {
    const user = userEvent.setup();
    renderPicker();
    const combobox = screen.getByRole("combobox", { name: "Movimentação" });

    await user.type(combobox, "1630");
    expect(rowsIn(openMenu())).toHaveLength(1);
    expect(rowsIn(openMenu())[0]).toHaveTextContent("1.630,04");

    await user.clear(combobox);
    await user.type(combobox, "corretora");
    expect(rowsIn(openMenu())).toHaveLength(1);
    expect(rowsIn(openMenu())[0]).toHaveTextContent("Rendimento");

    await user.clear(combobox);
    await user.type(combobox, "rendimento acao");
    expect(rowsIn(openMenu())).toHaveLength(1);

    await user.clear(combobox);
    await user.type(combobox, "13/09");
    expect(rowsIn(openMenu())).toHaveLength(2);
  });

  it("tells no search results apart from nothing eligible", async () => {
    const user = userEvent.setup();
    renderPicker({ emptyText: "Nenhuma movimentação compatível." });
    const combobox = screen.getByRole("combobox", { name: "Movimentação" });

    await user.type(combobox, "zzz");
    expect(within(openMenu()).getByText("Nenhuma transação encontrada.")).toBeInTheDocument();
    expect(within(openMenu()).getByText("Tente alterar sua busca.")).toBeInTheDocument();
  });

  it("shows the caller's empty text when nothing is eligible, and a loading state", async () => {
    const user = userEvent.setup();
    renderPicker({ transactions: [], emptyText: "Nenhuma movimentação compatível." });
    await user.click(screen.getByRole("combobox", { name: "Movimentação" }));
    expect(within(openMenu()).getByText("Nenhuma movimentação compatível.")).toBeInTheDocument();
  });

  it("shows the loading state instead of an empty message while searching", async () => {
    const user = userEvent.setup();
    renderPicker({ transactions: [], loading: true });
    await user.click(screen.getByRole("combobox", { name: "Movimentação" }));
    expect(within(openMenu()).getByText(/Buscando transações/)).toBeInTheDocument();
  });

  it("keeps the chosen row in the control after a server search narrows it out", async () => {
    const user = userEvent.setup();
    const search = { value: "", onChange: vi.fn() };
    const { onSelect, rerender } = (() => {
      const onSelect = vi.fn();
      const view = render(
        <TransactionPicker id="p" label="Movimentação" value={null} transactions={transactions} onSelect={onSelect} search={search} />,
      );
      return { onSelect, rerender: view.rerender };
    })();

    await user.click(screen.getByRole("combobox", { name: "Movimentação" }));
    await user.click(within(rowsIn(openMenu())[2]!).getByText("Rendimento Ação"));
    expect(onSelect).toHaveBeenCalledWith("income", transactions[2]);

    rerender(
      <TransactionPicker id="p" label="Movimentação" value="income" transactions={[]} onSelect={onSelect} search={search} />,
    );
    const control = document.querySelector(".ant-select-selector") as HTMLElement;
    expect(control).toHaveTextContent("Rendimento Ação");
    expect(control).toHaveTextContent("+R$ 123,21");
  });

  it("opens a searchable bottom sheet on a compact viewport and picks with the keyboard", async () => {
    compactViewport();
    const user = userEvent.setup();
    const { onSelect } = renderPicker({ value: "big" });

    const trigger = screen.getByRole("button", { name: "Movimentação" });
    expect(trigger).toHaveTextContent("Resgate CDB - Nu Financeira");
    expect(trigger).toHaveTextContent("+R$ 1.630,04");
    await user.click(trigger);

    const sheet = await screen.findByRole("dialog");
    const options = within(sheet).getAllByRole("option");
    expect(options).toHaveLength(3);
    expect(options[0]).toHaveAttribute("aria-selected", "true");

    await user.click(within(sheet).getByRole("combobox", { name: "Buscar em Movimentação" }));
    await user.keyboard("{ArrowDown}{Enter}");
    expect(onSelect).toHaveBeenCalledWith("small", transactions[1]);

    // Search narrows the list; a phone user never has to leave the box.
    await user.click(screen.getByRole("button", { name: "Movimentação" }));
    const reopened = await screen.findByRole("dialog");
    await user.type(within(reopened).getByRole("combobox", { name: "Buscar em Movimentação" }), "123");
    expect(within(reopened).getAllByRole("option")).toHaveLength(1);
    expect(within(reopened).getByRole("option")).toHaveTextContent("Rendimento Ação");
  });
});
