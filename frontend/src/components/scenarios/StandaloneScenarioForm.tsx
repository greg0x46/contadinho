import { Alert, Button, Drawer, Flex, Input } from "antd";
import { useEffect, useState } from "react";

export function StandaloneScenarioForm({
  open,
  submitting,
  submitError,
  onSubmit,
  onCancel,
}: {
  open: boolean;
  submitting: boolean;
  submitError: string | null;
  onSubmit: (name: string) => void;
  onCancel: () => void;
}) {
  const [name, setName] = useState("");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (open) {
      setName("");
      setError(null);
    }
  }, [open]);

  const submit = () => {
    if (name.trim() === "") {
      setError("Informe um nome para o cenário.");
      return;
    }
    setError(null);
    onSubmit(name.trim());
  };

  return (
    <Drawer
      title="Novo cenário"
      open={open}
      onClose={onCancel}
      width={400}
      destroyOnHidden
      footer={
        <Flex justify="end" gap="small">
          <Button onClick={onCancel}>Cancelar</Button>
          <Button type="primary" loading={submitting} onClick={submit}>
            Salvar
          </Button>
        </Flex>
      }
    >
      <Flex vertical gap="middle">
        {(error ?? submitError) && <Alert type="error" showIcon message={error ?? submitError} />}
        <div className="filter-field">
          <label htmlFor="standalone-scenario-name">Nome</label>
          <Input
            id="standalone-scenario-name"
            value={name}
            onChange={(event) => setName(event.target.value)}
            placeholder="Ex.: Viagem, Novo emprego"
          />
        </div>
      </Flex>
    </Drawer>
  );
}
