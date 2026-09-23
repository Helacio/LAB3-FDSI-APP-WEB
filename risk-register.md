# Registro de riesgos — Laboratorio 3

Estado: `corregido` | `mitigado` | `aceptado` | `pendiente Lab 3.2`

| ID | Riesgo | STRIDE | Evidencia | Corrección | Antes | Después | Estado |
|---|---|---|---|---|---|---|---|
| R1 | Trafico HTTP en claro: confidencialidad e integridad nulas | H1, H4 | `evidence/red/curl_home.txt`, `evidence/blue/lab3-http.pcap` | Ninguna en Lab 3 (límite pedagógico) | contenido visible en PCAP| sin cambio | Aceptado → pendiente Lab 3.2 (HTTPS) |
| R2 | Banner revela versión de Nginx | H2 | `evidence/red/nmap_port80.nmap`, `curl_headers.txt` | `server_tokens off` | `Server: nginx/1.x.x` | `Server: nginx` | Corregido (retest: `evidence/retest/headers_after.txt`) |
| R3 | Headers de seguridad ausentes (nosniff, framing, referrer) | H2 | `evidence/red/curl_headers.txt`, reporte ZAP pasivo | `add_header X-Content-Type-Options nosniff; X-Frame-Options DENY; Referrer-Policy no-referrer` | headers ausentes | headers presentes | Corregido |
| R4 | Rutas ocultas (`.git`, `.env`) servidas si existen | H2, H6 | `curl -i $TARGET_URL/.git/config` → 404 tras fix | `location ~ /\. { deny all; }` | 404 o 200 según archivo | siempre 404 | Corregido |
| R5 | Listado de directorios | H2 | revisión manual | `autoindex off` | listado posible | deshabilitado | Corregido |
| R6 | Inventario público con más datos de los necesarios | H6 | `app/public-inventory.txt` (versión git) | reducción de exposición, antes/después en Git | versión amplia | versión mínima | Mitigado |
| R7 | Sin autenticación ni roles: spoofing de operador | H5 | H5 en `docs/threat-model.md` | fuera de alcance Lab 3 | — | — | Pendiente Lab 4 (identidad, MFA, roles) |
| R8 | Sin trazabilidad de identidad: repudio de acciones | H3 | `access.log` sin usuario | línea de tiempo Purple Team con timestamp UTC | sin correlación | eventos correlacionados | Mitigado (parcial) |
| R9 | Firewall de host deshabilitado (UFW inactivo) | — | `sudo ufw status` → inactive; escaneo zgrab/0.x desde Internet en access.log | Paso 5: `ufw default deny incoming`, `allow OpenSSH`, `allow 80/tcp` | puerto 80 expuesto a todo Internet; 8080/5672/4369/25672 escuchando en 0.0.0.0 | UFW active, solo 22 y 80 abiertos; retest en `evidence/blue/ufw_status.txt` | Corregido (80 abierto a todos: aceptado) |
| R10 | Detección 404 ciega ante SPA (falsos negativos) | — | `evidence/blue/top_404.txt`: `/noexiste-xyz` → 200 | documentar limitación; alternativa: monitorear status del backend en 8080 o respuestas `deny all` | regla de 404 inefectiva en SPA | limitación documentada | Aceptado |

## Riesgos a mitigar para Laboratorio 3.2

- HTTPS + certificados (cierra R1 parcialmente, H1/H4).
- Autenticación con MFA y roles lectura/cambio (R7, H5/H6).
- Firma/integridad de la bitácora (R8 completo, H3).
