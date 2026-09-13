#!/usr/bin/env bash
# detect-404.sh — Regla de detección del Laboratorio 3 (Blue Team)
#
# Señal: una misma IP produce >= THRESHOLD respuestas 404 en una ventana de 5 minutos.
# Implementación: agrupa por IP + minuto UTC (columna $4 del access.log) y marca las
# IPs que superan el umbral. Es una aproximación de la ventana deslizante de 5 min.
#
# Limitaciones conocidas (documentarlas en el reporte):
#   - Ventana por minuto, no deslizante: actividad repartida en 5 minutos distintos
#     puede quedar por debajo del umbral (falso negativo).
#   - Falsos positivos: typos legítimos de usuarios, escáneres autorizados del lab
#     (Nmap/ZAP), robots de indexación.
#   - No bloquea nada: solo genera la señal para análisis humano.
#
# Uso: sudo ./detect-404.sh [/var/log/nginx/access.log]
# Salida: listado de IPs señaladas + top de 404 por IP.

set -u
LOG="${1:-/var/log/nginx/access.log}"
THRESHOLD=5

if [ ! -r "$LOG" ]; then
  echo "No se puede leer $LOG (usa sudo o pasa otra ruta)." >&2
  exit 1
fi

echo "[*] Regla: IP con >= $THRESHOLD respuestas 404 en la misma ventana (minuto UTC)"
echo "-------------------------------------------------------------------------------"
grep ' 404 ' "$LOG" \
  | awk '{print $1, substr($4, 1, 15)}' \
  | sort | uniq -c | sort -rn \
  | awk -v t="$THRESHOLD" '$1 >= t {printf "SENAL  %-15s %s  ->  %s x 404\n", $2, $3, $1}'

echo
echo "[*] Contexto: total de 404 por IP (histórico)"
echo "----------------------------------------------"
grep ' 404 ' "$LOG" | awk '{print $1}' | sort | uniq -c | sort -rn | head -n 10

echo
echo "[*] Contexto: rutas más pedidas que devolvieron 404"
echo "-----------------------------------------------------"
grep ' 404 ' "$LOG" | awk '{print $7}' | sort | uniq -c | sort -rn | head -n 10
