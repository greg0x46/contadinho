const formatter = new Intl.DateTimeFormat("pt-BR", {
  dateStyle: "short",
  timeStyle: "medium",
});

export function formatDate(value: string): string {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "Data inválida" : formatter.format(date);
}

export function formatOptionalDate(value: string | null): string {
  return value === null ? "Ainda não disponível" : formatDate(value);
}
