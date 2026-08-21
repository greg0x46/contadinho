# Configurações / Autenticação

> Referência viva deste contexto — o que existe e funciona hoje. Mantenha
> atualizado ao mudar a feature.

## O que é

Setup inicial (senha de desbloqueio + credenciais Pluggy), autenticação por
sessão, e armazenamento de configurações sensíveis criptografadas em
repouso (AES-256-GCM, chave derivada via Argon2id). A chave existe só na
memória do processo — o app volta a ficar bloqueado a cada reinício.

## Backend

`internal/settings` — `Setup`, `VerifyPassword`, `Session`, `Get/Set`.

## Rotas HTTP

`GET /api/setup/status`, `POST /api/setup`, `POST /api/unlock`,
`GET/PUT /api/preferences`.

## Frontend

- `SetupPage`/`UnlockPage` — fora do roteador principal, controladas por
  `SetupGate`.
- `/configuracoes` — preferências gerais (ex.: dia de fechamento de fatura
  padrão).
