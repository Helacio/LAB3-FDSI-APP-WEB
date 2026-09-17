#!/usr/bin/env bash
# install.sh — Aprovisiona el stack MuvAutomation en la VM (Ubuntu/Debian).
#
# Ejecutar como root (o con sudo) desde el directorio que contiene los artefactos:
#   frontend/            build de Angular (contenido de dist/frontend/browser)
#   muvbackend           binario Go (linux/amd64)
#   muvautomation.service
#   muvautomation.conf   vhost de nginx
#
# Uso:
#   sudo ./install.sh [STAGE_DIR]   # STAGE_DIR por defecto: directorio del script
#
# Es idempotente: se puede correr varias veces sin efectos adversos.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STAGE="${1:-$SCRIPT_DIR}"

WEB_ROOT="/var/www/muvautomation"
APP_DIR="/opt/muvautomation"
APP_USER="muvautomation"
MONGO_MAJOR="8.0"

log() { echo "[install] $*"; }

if [ "$(id -u)" -ne 0 ]; then
  echo "Este script debe ejecutarse como root (usa sudo)." >&2
  exit 1
fi

# --- Dependencias base ---
export DEBIAN_FRONTEND=noninteractive
if ! command -v nginx >/dev/null 2>&1; then
  log "instalando nginx"
  apt-get update
  apt-get install -y nginx
fi

# --- MongoDB (repositorio oficial) ---
if ! command -v mongod >/dev/null 2>&1; then
  log "instalando MongoDB ${MONGO_MAJOR}"
  apt-get update
  apt-get install -y gnupg curl ca-certificates
  curl -fsSL "https://www.mongodb.org/static/pgp/server-${MONGO_MAJOR}.asc" \
    | gpg --dearmor -o "/usr/share/keyrings/mongodb-server-${MONGO_MAJOR}.gpg"
  CODENAME="$(. /etc/os-release && echo "${UBUNTU_CODENAME:-${VERSION_CODENAME}}")"
  echo "deb [ signed-by=/usr/share/keyrings/mongodb-server-${MONGO_MAJOR}.gpg ] https://repo.mongodb.org/apt/ubuntu ${CODENAME}/mongodb-org/${MONGO_MAJOR} multiverse" \
    > "/etc/apt/sources.list.d/mongodb-org-${MONGO_MAJOR}.list"
  apt-get update
  apt-get install -y mongodb-org
fi
log "habilitando mongod"
systemctl enable --now mongod

# --- Usuario de sistema y directorios ---
if ! id "$APP_USER" >/dev/null 2>&1; then
  log "creando usuario $APP_USER"
  useradd --system --home "$APP_DIR" --shell /usr/sbin/nologin "$APP_USER"
fi
mkdir -p "$APP_DIR" "$WEB_ROOT"

# --- Frontend Angular ---
log "desplegando frontend en $WEB_ROOT"
find "$WEB_ROOT" -mindepth 1 -delete
cp -r "$STAGE/frontend/." "$WEB_ROOT/"

# --- Backend Go ---
log "desplegando backend en $APP_DIR"
install -m 0755 "$STAGE/muvbackend" "$APP_DIR/muvbackend"
chown -R "$APP_USER:$APP_USER" "$APP_DIR"

# --- vhost nginx ---
log "instalando vhost de nginx"
install -m 0644 "$STAGE/muvautomation.conf" /etc/nginx/sites-available/muvautomation
ln -sf /etc/nginx/sites-available/muvautomation /etc/nginx/sites-enabled/muvautomation
rm -f /etc/nginx/sites-enabled/default
nginx -t
systemctl reload nginx 2>/dev/null || systemctl restart nginx

# --- Servicio systemd del backend ---
log "instalando servicio systemd"
install -m 0644 "$STAGE/muvautomation.service" /etc/systemd/system/muvautomation.service
systemctl daemon-reload
systemctl enable muvautomation
systemctl restart muvautomation

sleep 2
systemctl --no-pager --full status muvautomation || true
log "listo. Verifica: curl -i http://127.0.0.1/api/healthz"
