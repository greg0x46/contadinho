import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import { FilterCombobox } from "./FilterCombobox";

const options = ["Nubank", "Itaú", "Inter"].map((label, index) => ({ value: `acc-${index}`, label }));

function compactViewport() {
  vi.mocked(window.matchMedia).mockImplementation((query: string) => ({
    matches: /max-width/.test(query),
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }));
}

function Harness({ onChange }: { onChange: (value: string[]) => void }) {
  const [value, setValue] = useState<string[]>([]);
  return (
    <>
      <label htmlFor="accounts">Conta</label>
      <FilterCombobox
        multiple
        id="accounts"
        label="Contas"
        placeholder="Todas as contas"
        options={options}
        value={value}
        onChange={(next) => {
          setValue(next);
          onChange(next);
        }}
      />
    </>
  );
}

describe("FilterCombobox", () => {
  it("on a phone opens a sheet whose picks only commit on Confirmar", async () => {
    compactViewport();
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Harness onChange={onChange} />);

    await user.click(screen.getByRole("button", { name: "Conta" }));
    const sheet = await screen.findByRole("dialog");
    expect(sheet).toHaveTextContent("Contas");
    await user.click(screen.getByRole("option", { name: "Nubank" }));
    await user.click(screen.getByRole("option", { name: "Itaú" }));
    expect(screen.getByText("2 selecionadas")).toBeVisible();
    expect(onChange).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Cancelar" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(onChange).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Conta" })).toHaveTextContent("Todas as contas");

    await user.click(screen.getByRole("button", { name: "Conta" }));
    await user.click(await screen.findByRole("option", { name: "Inter" }));
    await user.click(screen.getByRole("button", { name: "Confirmar" }));
    expect(onChange).toHaveBeenLastCalledWith(["acc-2"]);
    await waitFor(() => expect(screen.getByRole("button", { name: "Conta" })).toHaveTextContent("Inter"));
  });

  it("reports loading, errors and an empty search", async () => {
    const user = userEvent.setup();
    const { rerender } = render(
      <FilterCombobox
        multiple
        id="x"
        label="Contas"
        placeholder="Todas"
        options={[]}
        value={[]}
        onChange={vi.fn()}
        loading
      />,
    );
    await user.click(screen.getByRole("button"));
    expect(await screen.findByRole("status")).toHaveTextContent("Carregando");

    const onRetry = vi.fn();
    rerender(
      <FilterCombobox
        multiple
        id="x"
        label="Contas"
        placeholder="Todas"
        options={[]}
        value={[]}
        onChange={vi.fn()}
        error="Falhou"
        onRetry={onRetry}
      />,
    );
    expect(screen.getByRole("alert")).toHaveTextContent("Falhou");
    await user.click(screen.getByRole("button", { name: "Tentar novamente" }));
    expect(onRetry).toHaveBeenCalled();

    rerender(
      <FilterCombobox multiple id="x" label="Contas" placeholder="Todas" options={options} value={[]} onChange={vi.fn()} />,
    );
    await user.type(screen.getByRole("combobox"), "zzz");
    expect(screen.getByText("Nenhum resultado para a busca.")).toBeVisible();
  });
});
