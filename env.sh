#!/bin/sh

ENV_FILE="$(dirname "$0")/.env"

if [ ! -f "$ENV_FILE" ]; then
    echo "ERREUR : fichier .env introuvable : $ENV_FILE" >&2
    exit 1
fi

set -a
. "$ENV_FILE"
set +a

echo "Environnement epg-vmedia chargé depuis $ENV_FILE"
