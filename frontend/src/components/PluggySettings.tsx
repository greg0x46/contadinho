import { Alert, Button, Card, Flex, Input } from "antd";
import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "../api/transport";

export function PluggySettings() {
  const client = useQueryClient();
  const [id, setID] = useState("");
  const [secret, setSecret] = useState("");
  const [item, setItem] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);
  const save = async () => {
    setBusy(true); setError(null); setSaved(false);
    try {
      const response = await apiFetch("/api/settings/pluggy", { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ pluggy_client_id: id, pluggy_client_secret: secret, pluggy_item_id: item }) });
      if (!response.ok) throw new Error("Não foi possível salvar as credenciais.");
      setID(""); setSecret(""); setItem(""); setSaved(true);
      void client.invalidateQueries({ queryKey: ["data-sources"] });
    } catch (err) { setError(err instanceof Error ? err.message : "Não foi possível salvar."); }
    finally { setBusy(false); }
  };
  return <Card title="Credenciais da Pluggy" style={{ marginBottom: 24 }}>
    <form onSubmit={(event) => { event.preventDefault(); void save(); }}>
      <Flex vertical gap="middle" style={{ maxWidth: 440 }}>
        <p>Preencha para configurar ou substituir as credenciais. Os valores salvos não são exibidos.</p>
        {error && <Alert type="error" message={error} />}
        {saved && <Alert type="success" message="Credenciais salvas." />}
        <label htmlFor="pluggy-client-id">Client ID</label>
        <Input id="pluggy-client-id" required value={id} onChange={(e) => setID(e.target.value)} />
        <label htmlFor="pluggy-client-secret">Client secret</label>
        <Input.Password id="pluggy-client-secret" required autoComplete="off" value={secret} onChange={(e) => setSecret(e.target.value)} />
        <label htmlFor="pluggy-item">Item ID de uma nova conexão (opcional)</label>
        <Input id="pluggy-item" value={item} onChange={(e) => setItem(e.target.value)} />
        <Button htmlType="submit" loading={busy}>Salvar credenciais</Button>
      </Flex>
    </form>
  </Card>;
}
