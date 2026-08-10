import { useEffect, useState } from "react";
import { Alert, Button, InputNumber, Modal, Tooltip, Typography } from "antd";

import type { Account } from "../../api/contracts";
import { closingDayLabel, closingDaySourceHint } from "../../presentation/accountLabels";

/** Shows a card's closing day and lets the user state it. The manual value
 *  wins over anything the provider reports, and clearing it hands control
 *  back to the provider. */
export function ClosingDayField({
  account,
  onSave,
  saving,
}: {
  account: Account;
  onSave: (closingDay: number | null) => Promise<unknown>;
  saving: boolean;
}) {
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState<number | null>(account.closing_day);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (open) {
      setDraft(account.closing_day);
      setError(null);
    }
  }, [open, account.closing_day]);

  const submit = async (value: number | null) => {
    if (value !== null && (!Number.isInteger(value) || value < 1 || value > 31)) {
      setError("Informe um dia entre 1 e 31.");
      return;
    }
    try {
      await onSave(value);
      setOpen(false);
    } catch {
      setError("Não foi possível salvar o dia de fechamento.");
    }
  };

  return (
    <>
      <li>
        <span>Fechamento</span>
        <Tooltip title={closingDaySourceHint(account.closing_day_source)}>
          <strong>{closingDayLabel(account.closing_day)}</strong>
        </Tooltip>
        <Button size="small" type="link" onClick={() => setOpen(true)}>
          {account.closing_day === null ? "Definir" : "Alterar"}
        </Button>
      </li>

      <Modal
        open={open}
        title="Dia de fechamento"
        onCancel={() => setOpen(false)}
        destroyOnHidden
        footer={[
          account.closing_day_source === "manual" && (
            <Button key="clear" danger loading={saving} onClick={() => void submit(null)}>
              Usar o dado da instituição
            </Button>
          ),
          <Button key="cancel" onClick={() => setOpen(false)}>
            Cancelar
          </Button>,
          // An empty field would submit null, which is what the clear button
          // above already does — and that button only shows when there is
          // something to clear. Saving nothing is never the intent here.
          <Button
            key="save"
            type="primary"
            loading={saving}
            disabled={draft === null}
            onClick={() => void submit(draft)}
          >
            Salvar
          </Button>,
        ]}
      >
        <Typography.Paragraph type="secondary">
          Nem toda instituição informa o fechamento do cartão. Defina o dia aqui para que o
          Contadinho use o seu valor.
        </Typography.Paragraph>
        <div className="filter-field">
          <label htmlFor="closing-day-input">Dia do mês</label>
          <InputNumber
            id="closing-day-input"
            min={1}
            max={31}
            precision={0}
            style={{ width: "100%" }}
            value={draft}
            onChange={setDraft}
          />
        </div>
        {error !== null && (
          <Alert type="error" showIcon message={error} style={{ marginTop: 12 }} />
        )}
      </Modal>
    </>
  );
}
