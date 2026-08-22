#!/bin/sh
# Builds the frontend, embeds it into internal/webui/dist, then builds the
# single contadinho binary. See README.md "Rodar a build de produção".
set -eu

cd "$(dirname "$0")"

(cd frontend && npm install && npm run build)

rm -rf internal/webui/dist
mkdir -p internal/webui/dist
cp -r frontend/dist/* internal/webui/dist/

go build -tags embedded_frontend -o contadinho ./cmd/contadinho

echo "build ok: ./contadinho"
