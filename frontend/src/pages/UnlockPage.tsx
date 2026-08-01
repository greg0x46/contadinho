import { Alert, Button, Card, Flex, Input, Typography } from "antd";
import { useState } from "react";

import { unlock } from "../api/setup";
import type { SetupStatus } from "../api/contracts";

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "Não foi possível desbloquear.";
}

export function UnlockPage({ onDone }: { onDone: (status: SetupStatus) => void }) {
  const [password, setPassword] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = async () => {
    if (password === "") {
      setError("Informe a senha.");
      return;
    }
    setError(null);
    setSubmitting(true);
    try {
      const status = await unlock({ password });
      onDone(status);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Flex justify="center" align="center" style={{ minHeight: "100vh", padding: 24 }}>
      <Card title="Desbloquear o Contadinho" style={{ maxWidth: 380, width: "100%" }}>
        <Flex vertical gap="middle">
          <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
            O servidor foi reiniciado. Informe a senha definida na configuração para acessar as
            credenciais do Pluggy novamente.
          </Typography.Paragraph>
          {error && <Alert type="error" showIcon message={error} />}

          <div className="filter-field">
            <label htmlFor="unlock-password">Senha</label>
            <Input.Password
              id="unlock-password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              autoComplete="current-password"
              autoFocus
              onPressEnter={submit}
            />
          </div>

          <Button type="primary" block loading={submitting} onClick={submit}>
            Desbloquear
          </Button>
        </Flex>
      </Card>
    </Flex>
  );
}
