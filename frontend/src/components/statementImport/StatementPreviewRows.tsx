import { Button, Tag } from "antd";
import { useState } from "react";

import type { ImportRow } from "../../api/statementImports";
import { formatDate } from "../../presentation/dates";
import { formatMoney } from "../../presentation/money";

const pageSize = 20;

const statusLabel: Record<ImportRow["status"], string> = { new: "Nova", duplicate: "Já importada", invalid: "Inválida" };
const statusColor: Record<ImportRow["status"], string> = { new: "green", duplicate: "default", invalid: "red" };

/**
 * One line of the file: description and where it came from on the left, the
 * amount and the running balance on the right, then its status and review
 * notes. Errors keep the error color; warnings stay plain but visible.
 */
function StatementPreviewRow({ row }: { row: ImportRow }) {
  const currency = row.currency ?? "BRL";

  return (
    <li className={`statement-row is-${row.status}`}>
      <div className="statement-row-identity">
        <span className="statement-row-description">{row.description || "—"}</span>
        <span className="statement-row-meta">
          <span>Linha {row.line_number}</span>
          <span>{row.occurred_at ? formatDate(row.occurred_at) : "—"}</span>
        </span>
      </div>
      <div className="statement-row-figures">
        <span className="statement-row-amount">{row.amount ? formatMoney(row.amount, currency) : "—"}</span>
        <span className="statement-row-balance">Saldo após {row.balance ? formatMoney(row.balance, currency) : "—"}</span>
      </div>
      <div className="statement-row-review">
        <Tag color={statusColor[row.status]}>{statusLabel[row.status]}</Tag>
        {(row.errors.length > 0 || row.warnings.length > 0) && (
          <ul className="statement-row-notes">
            {row.errors.map((value, index) => <li key={`e-${index}`} className="statement-row-error">{value}</li>)}
            {row.warnings.map((value, index) => <li key={`w-${index}`}>{value}</li>)}
          </ul>
        )}
      </div>
    </li>
  );
}

/**
 * The preview's rows as a mobile-first list (a table would scroll sideways on
 * a phone), 20 to a page: a file can carry thousands of lines. The footer is
 * the same Exibindo/Anterior/Próxima one the transactions list uses.
 */
export function StatementPreviewRows({ rows }: { rows: ImportRow[] }) {
  const [page, setPage] = useState(1);

  if (rows.length === 0) return <p className="statement-rows-empty">Nenhuma linha para mostrar</p>;

  const totalPages = Math.ceil(rows.length / pageSize);
  const current = Math.min(page, totalPages);
  const start = (current - 1) * pageSize;
  const visible = rows.slice(start, start + pageSize);

  return (
    <>
      <ul className="statement-row-list" aria-label="Linhas do arquivo">
        {visible.map((row) => <StatementPreviewRow key={row.line_number} row={row} />)}
      </ul>
      {totalPages > 1 && (
        <footer className="statement-import-pagination">
          <span>
            Exibindo {(start + 1).toLocaleString("pt-BR")}–{(start + visible.length).toLocaleString("pt-BR")} de{" "}
            {rows.length.toLocaleString("pt-BR")} linhas
          </span>
          <nav aria-label="Paginação das linhas">
            <Button disabled={current <= 1} onClick={() => setPage(current - 1)}>Anterior</Button>
            <span>Página {current} de {totalPages}</span>
            <Button disabled={current >= totalPages} onClick={() => setPage(current + 1)}>Próxima</Button>
          </nav>
        </footer>
      )}
    </>
  );
}
