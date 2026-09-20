import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { ChoiceField } from "./ChoiceField";

const options = [
  { value: "a", label: "Despesa: Alimentação" },
  { value: "t", label: "Despesa: Transporte", tag: "Sugestão" },
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

describe("ChoiceField", () => {
  it("is a combobox on a wide viewport", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<ChoiceField id="f" label="Categoria" value="a" options={options} onChange={onChange} />);

    await user.click(screen.getByRole("combobox", { name: "Categoria" }));
    await user.click(await screen.findByText("Despesa: Transporte"));
    expect(onChange).toHaveBeenCalledWith("t");
  });

  it("opens a searchable bottom sheet on a compact viewport", async () => {
    compactViewport();
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<ChoiceField id="f" label="Categoria" value="a" options={options} onChange={onChange} />);

    const trigger = screen.getByRole("button", { name: "Categoria" });
    expect(trigger).toHaveTextContent("Despesa: Alimentação");
    await user.click(trigger);

    const sheet = await screen.findByRole("dialog");
    await user.type(within(sheet).getByRole("textbox", { name: "Buscar em Categoria" }), "trans");
    expect(within(sheet).queryByText("Despesa: Alimentação")).not.toBeInTheDocument();
    expect(within(sheet).getByText("Sugestão")).toBeVisible();
    await user.click(within(sheet).getByRole("option", { name: /Transporte/ }));

    expect(onChange).toHaveBeenCalledWith("t");
  });
});
