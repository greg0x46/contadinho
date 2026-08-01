export function movementTypeLabel(type: string | null): string {
  if (!type) return "Tipo não informado";
  if (type === "CREDIT") return "Crédito";
  if (type === "DEBIT") return "Débito";
  return type;
}
