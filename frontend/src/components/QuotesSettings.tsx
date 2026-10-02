import { Alert, Button, Card, Flex, Input } from "antd";
import { useState } from "react";
import { apiFetch } from "../api/transport";

// Modeled directly on PluggySettings.tsx: a write-only credential form for
// PUT /api/settings/quotes. Unlike Pluggy's client id/secret, the brapi
// token is optional — the brapi provider (a B3 fallback behind Yahoo Finance)
// sends requests unauthenticated when it is unset — so the field is not
// required here.
export function QuotesSettings() {
  const [token, setToken] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);
  const save = async () => {
    setBusy(true);
    setError(null);
    setSaved(false);
    try {
      const response = await apiFetch("/api/settings/quotes", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ brapi_token: token }),
      });
      if (!response.ok) throw new Error("Não foi possível salvar o token.");
      setToken("");
      setSaved(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Não foi possível salvar.");
    } finally {
      setBusy(false);
    }
  };
  return (
    <Card title="Cotações automáticas — token da brapi (opcional)" style={{ marginBottom: 24 }}>
      <form onSubmit={(event) => { event.preventDefault(); void save(); }}>
        <Flex vertical gap="middle" style={{ maxWidth: 440 }}>
          <p>
            Por padrão, a cotação automática de posições manuais usa o Yahoo Finance como fonte principal;
            a brapi (B3) e a CoinGecko (criptomoedas) são usadas quando ele falha.
          </p>
          <p>
            O token da brapi só importa nesse fallback da B3: sem ele, apenas um conjunto restrito
            de tickers responde. O valor salvo não é exibido novamente.
          </p>
          {error && <Alert type="error" message={error} />}
          {saved && <Alert type="success" message="Token salvo." />}
          <label htmlFor="quotes-brapi-token">Token da brapi</label>
          <Input.Password id="quotes-brapi-token" autoComplete="off" value={token} onChange={(e) => setToken(e.target.value)} />
          <Button htmlType="submit" loading={busy}>Salvar token</Button>
        </Flex>
      </form>
    </Card>
  );
}
