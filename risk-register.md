# Registro de riesgos — Laboratorio 3

| ID | Riesgo | STRIDE | Evidencia |
|---|---|---|---|
| R1 | Tráfico HTTP en claro: confidencialidad e integridad nulas | H1, H4 | `evidence/blue/lab3-http.pcap` (contenido, rutas y headers visibles en claro) |
| R2 | Banner del servidor expone tecnología (fingerprinting) | H2 | `curl -I` → `Server: nginx` |
| R3 | Headers de seguridad ausentes (nosniff, framing, referrer) | H2 | `curl -I` previo al cambio; reporte ZAP pasivo |
| R4 | Rutas ocultas (`.git`, `.env`) servibles si el archivo existe | H2, H6 | prueba `GET /.git/config` |
| R5 | Listado de directorios | H2 | revisión de configuración (`autoindex`) |
| R6 | Inventario público con más datos de los necesarios | H6 | `app/public-inventory.txt` (versión git) |
| R7 | Sin autenticación ni roles: spoofing de operador | H5 | `docs/threat-model.md` (H5) |
| R8 | Sin trazabilidad de identidad: repudio de acciones | H3 | `access.log` sin usuario |
| R9 | Firewall de host deshabilitado (UFW inactivo) | — | `sudo ufw status` → inactive; escaneo `zgrab/0.x` desde Internet en `access.log`; puertos 8080/5672/4369/25672 escuchando en 0.0.0.0 |
| R10 | Detección 404 ciega ante SPA (falsos negativos) | — | `evidence/blue/top_404.txt`: `GET /noexiste-xyz` → 200 |
| R11 | Software con vulnerabilidades conocidas: nginx 1.24.0 | — | `nginx -v` → `nginx/1.24.0 (Ubuntu)`, paquete `1.24.0-2ubuntu7.18` |

## R11 — Versión instalada y vulnerabilidades conocidas

- Binario: `nginx/1.24.0 (Ubuntu)`, paquete `1.24.0-2ubuntu7.18` (amd64).
- Módulos compilados verificados con `nginx -V`: `ngx_http_mp4_module` ✓, `ngx_http_v2_module` ✓.

### CVEs cuyos rangos afectados incluyen 1.24.0 (según advisory oficial de nginx)

| CVE | Severidad | Módulo/condición | Aplicable en la config actual |
|---|---|---|---|
| CVE-2026-42533 | major | `map` + regex | No (no se usa `map`) |
| CVE-2025-23419 | medium | Reutilización de sesión SSL con certificados de cliente | No hoy (sin HTTPS); relevante Lab 3.2 |
| CVE-2026-9256 | medium | `ngx_http_rewrite_module` | Posible (módulo built-in) |
| CVE-2026-42945 | medium | `ngx_http_rewrite_module` | Posible (módulo built-in) |
| CVE-2026-42055 | medium | `proxy_v2` / `grpc` | No (no se usan) |
| CVE-2026-40701 | medium | resolver + OCSP (TLS) | No hoy; relevante Lab 3.2 |
| CVE-2024-7347 | low | `ngx_http_mp4_module` (overread) | No en la práctica (módulo compilado, pero sin directiva `mp4` en config) |
| CVE-2026-48142 | low | `ngx_http_charset_module` | No (no se usa charset) |
| CVE-2026-42934 | low | `ngx_http_charset_module` | No (no se usa charset) |
| CVE-2023-44487 | — (DoS) | HTTP/2 Rapid Reset (nginx < 1.25.3) | No hoy (módulo http2 compilado, pero `http2` no habilitado en `listen`); relevante si se activa HTTP/2 |

### Notas de análisis

- Los CVE de HTTP/3 (CVE-2024-31079, 32760, 35200, 34161, 24989, 24990) **no aplican**: nginx 1.24.0 (stable) no incluye HTTP/3/QUIC.
- Ubuntu **backportea parches de seguridad** al paquete `1.24.0-2ubuntu7.18`; la exposición real debe validarse contra el changelog del paquete (USN), no solo contra la versión upstream: `apt-get changelog nginx` o `ubuntu-security-status`.
- Exposición práctica hoy: solo HTTP/1.1 en puerto 80, proxying a `127.0.0.1:8080`; `mp4`, `http2`, `grpc`, `charset` y `map` no se usan en la configuración actual.
- El banner no revela la versión (`server_tokens off`), por lo que un atacante externo no puede asociar estos CVEs al servidor por fingerprinting básico (ver R2).

## Riesgos a tratar en Laboratorio 3.2

- HTTPS + certificados (R1 parcial, H1/H4; además activa la superficie TLS de CVE-2025-23419 y CVE-2026-40701).
- Autenticación con MFA y roles lectura/cambio (R7, H5/H6).
- Firma/integridad de la bitácora (R8 completo, H3).
- Actualización o confirmación de parches de nginx (R11).
