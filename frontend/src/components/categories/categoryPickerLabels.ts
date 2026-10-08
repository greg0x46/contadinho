// pt-BR accessible names for the category form's icon and colour pickers. The
// registry keys (`ellipsis`, `line-chart`) are what the API stores, not
// something to show a person: the pickers display the glyph or the swatch
// and name it with these.

export const categoryIconLabel: Record<string, string> = {
  "shopping-cart": "Carrinho de compras",
  coffee: "Café",
  car: "Carro",
  shopping: "Sacola de compras",
  bank: "Banco",
  home: "Casa",
  smile: "Sorriso",
  percentage: "Porcentagem",
  medicine: "Saúde",
  scissor: "Tesoura",
  team: "Pessoas",
  tool: "Ferramenta",
  wifi: "Internet",
  book: "Livro",
  "safety-certificate": "Segurança",
  heart: "Coração",
  global: "Mundo",
  gift: "Presente",
  "line-chart": "Gráfico de linha",
  ellipsis: "Reticências",
  "money-collect": "Dinheiro",
  laptop: "Notebook",
  trophy: "Troféu",
  rise: "Crescimento",
  rollback: "Reembolso",
  wallet: "Carteira",
  "plus-circle": "Mais",
  swap: "Troca",
  phone: "Telefone",
  fund: "Fundo",
  dollar: "Dólar",
  cloud: "Nuvem",
  hourglass: "Ampulheta",
  thunderbolt: "Raio",
  "credit-card": "Cartão de crédito",
  "customer-service": "Atendimento",
  fire: "Fogo",
  bulb: "Lâmpada",
};

export const categoryColorLabel: Record<string, string> = {
  "#2a78d6": "Azul",
  "#eb6834": "Laranja queimado",
  "#17a2b8": "Turquesa",
  "#e64980": "Rosa escuro",
  "#d64545": "Vermelho",
  "#b8860b": "Ocre",
  "#eda100": "Âmbar",
  "#495057": "Grafite",
  "#37b24d": "Verde",
  "#e87ba4": "Rosa",
  "#4c6ef5": "Índigo",
  "#6c757d": "Cinza",
  "#099268": "Verde-água",
  "#7c5cbf": "Roxo",
  "#1baf7a": "Verde Julius",
  "#f76707": "Laranja",
};

/** The accessible name of an icon key, falling back to the key for a legacy one. */
export function iconLabel(key: string): string {
  return categoryIconLabel[key] ?? key;
}

export function colorLabel(hex: string): string {
  return categoryColorLabel[hex] ?? hex;
}
