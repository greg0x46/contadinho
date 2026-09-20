import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { CategoryChoice } from "../../presentation/transactionDetail";
import { CategoryField } from "./CategoryField";
import { categorySections } from "./categorySections";

const choice = (value: string, name: string, kind: CategoryChoice["kind"] = "expense"): CategoryChoice => ({
  value,
  name,
  kind,
  label: `${kind}: ${name}`,
  isActive: true,
  icon: "coffee",
  color: "#eb6834",
});

const options = [
  choice("compras", "Compras"),
  choice("educacao", "Educação"),
  choice("lazer", "Lazer"),
  choice("familia", "Família"),
  choice("transf", "Transferências", "transfer"),
];

function compactViewport() {
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

function sectionTitles(root: HTMLElement) {
  return within(root).getAllByRole("heading", { level: 4 }).map((heading) => heading.textContent);
}

describe("categorySections", () => {
  it("puts the suggestion first, recents next and groups the rest by nature only when needed", () => {
    const sections = categorySections({ options, suggestedId: "transf", recentIds: ["lazer", "transf", "compras"], search: "" });
    expect(sections.map((section) => section.title)).toEqual(["Sugeridas", "Recentes", "Despesas", "Transferências"]);
    expect(sections[1]!.options.map((option) => option.name)).toEqual(["Lazer", "Compras"]);

    const single = categorySections({ options: options.slice(0, 3), suggestedId: null, recentIds: [], search: "" });
    expect(single.map((section) => section.title)).toEqual(["Todas"]);
  });

  it("searches by name ignoring accents and case, keeping the section structure", () => {
    const sections = categorySections({ options, suggestedId: "transf", recentIds: ["lazer"], search: "EDUCA" });
    expect(sections.map((section) => section.title)).toEqual(["Todas"]);
    expect(sections[0]!.options.map((option) => option.name)).toEqual(["Educação"]);
    expect(categorySections({ options, suggestedId: null, recentIds: [], search: "familia" })[0]!.options[0]!.name).toBe("Família");
  });
});

describe("CategoryField", () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  it("shows only the category name on the trigger and picks from a popover on a wide viewport", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<CategoryField id="c" value="lazer" options={options} onChange={onChange} />);

    const trigger = screen.getByRole("button", { name: "Categoria" });
    expect(trigger).toHaveTextContent("Lazer");
    expect(trigger).not.toHaveTextContent("Despesa");

    await user.click(trigger);
    const list = await screen.findByRole("listbox", { name: "Categorias" });
    expect(within(list).getByRole("option", { name: "Lazer" })).toHaveAttribute("aria-selected", "true");
    expect(within(list).getByRole("option", { name: "Compras" })).toHaveAttribute("aria-selected", "false");

    await user.click(within(list).getByRole("option", { name: "Compras" }));
    expect(onChange).toHaveBeenCalledWith("compras");
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
  });

  it("offers the suggestion under the field and as the first section, and Usar applies it", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<CategoryField id="c" value="lazer" options={options} suggested={options[4]!} onChange={onChange} />);

    expect(screen.getByText(/Sugestão:/)).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Categoria" }));
    const list = await screen.findByRole("listbox", { name: "Categorias" });
    expect(sectionTitles(list)[0]).toBe("Sugeridas");

    await user.click(screen.getByRole("button", { name: "Usar" }));
    expect(onChange).toHaveBeenCalledWith("transf");
  });

  it("hides Usar once the suggestion is the current category", () => {
    render(<CategoryField id="c" value="transf" options={options} suggested={options[4]!} onChange={vi.fn()} />);
    expect(screen.queryByRole("button", { name: "Usar" })).not.toBeInTheDocument();
  });

  it("remembers picks as Recentes across mounts", async () => {
    const user = userEvent.setup();
    const { unmount } = render(<CategoryField id="c" value={null} options={options} onChange={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Categoria" }));
    await user.click(within(await screen.findByRole("listbox")).getByRole("option", { name: "Educação" }));
    unmount();

    render(<CategoryField id="c" value={null} options={options} onChange={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Categoria" }));
    const list = await screen.findByRole("listbox", { name: "Categorias" });
    expect(sectionTitles(list)[0]).toBe("Recentes");
    const recent = within(list).getAllByRole("heading", { level: 4 })[0]!.parentElement!;
    expect(within(recent).getByRole("option", { name: "Educação" })).toBeInTheDocument();
  });

  it("opens a bottom sheet with sticky search on a compact viewport and resets the search on close", async () => {
    compactViewport();
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<CategoryField id="c" value="lazer" options={options} onChange={onChange} />);

    await user.click(screen.getByRole("button", { name: "Categoria" }));
    const sheet = await screen.findByRole("dialog");
    expect(within(sheet).getByText("Selecionar categoria")).toBeVisible();

    await user.type(within(sheet).getByRole("textbox", { name: "Buscar categoria" }), "compr");
    expect(within(sheet).queryByRole("option", { name: "Lazer" })).not.toBeInTheDocument();
    await user.click(within(sheet).getByRole("option", { name: "Compras" }));
    expect(onChange).toHaveBeenCalledWith("compras");

    await user.click(screen.getByRole("button", { name: "Categoria" }));
    const reopened = await screen.findByRole("dialog");
    expect(within(reopened).getByRole("textbox", { name: "Buscar categoria" })).toHaveValue("");
    expect(within(reopened).getByRole("option", { name: "Lazer" })).toHaveAttribute("aria-selected", "true");
  });
});
