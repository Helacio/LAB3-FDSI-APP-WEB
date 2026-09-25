# Registro de riesgos — Laboratorio 3

| ID | Riesgo | STRIDE | Evidencia | Mitigación |
|---|---|---|---|---|
| R1 | Tráfico HTTP en claro: confidencialidad e integridad nulas | H1, H4 | `evidence/blue/lab3-http.pcap` (contenido, rutas y headers visibles en claro) | Corregido (Lab 3.2): HTTPS forzado con CA interna, redirect 80→443, HSTS, TLS 1.2/1.3 solo ciphers grado A. Evidencia: `evidence/blue/ca-interna-tls.txt`. Pendiente opcional: certificado de CA pública. |
| R2 | Banner del servidor expone tecnología (fingerprinting) | H2 | `curl -I` → `Server: nginx` | `server_tokens off` en snippet compartido |
| R3 | Headers de seguridad ausentes (nosniff, framing, referrer) | H2 | `curl -I` previo al cambio; reporte ZAP pasivo | `add_header` nosniff, DENY y no-referrer en snippet compartido |
| R4 | Rutas ocultas (`.git`, `.env`) servibles si el archivo existe | H2, H6 | prueba `GET /.git/config` | `location ~ /\. { deny all; return 404; }` |
| R5 | Listado de directorios | H2 | revisión de configuración (`autoindex`) | `autoindex off` |
| R6 | Inventario público con más datos de los necesarios | H6 | `app/public-inventory.txt` (versión git) | reducción del inventario al mínimo; antes/después conservado en git |
| R7 | Sin autenticación ni roles: spoofing de operador | H5 | `docs/threat-model.md` (H5) | Corregido (Lab 3.2): login + MFA TOTP + JWT; roles `lector`/`cambiador` con RBAC; operador de la bitácora sale del token. Evidencia: `evidence/blue/auth-mfa-rbac.txt`. |
| R8 | Sin trazabilidad de identidad: repudio de acciones | H3 | `access.log` sin usuario | Corregido (Lab 3.2): bitácora sellada con hash chain + HMAC y `GET /api/sessions/verify` que detecta modificaciones; operador ligado al token. Evidencia: `evidence/blue/bitacora-hmac.txt`. |
| R9 | Firewall de host deshabilitado (UFW inactivo) | — | `sudo ufw status` → inactive; escaneo `zgrab/0.x` desde Internet en `access.log`; puertos 8080/5672/4369/25672 escuchando en 0.0.0.0 | `ufw default deny incoming`, `allow OpenSSH`, `allow 80/tcp`, `allow 443/tcp` |
| R10 | Detección 404 ciega ante SPA (falsos negativos) | — | `evidence/blue/top_404.txt`: `GET /noexiste-xyz` → 200 | limitación documentada; alternativa: monitorear respuestas `deny all` o estado del backend en 8080 |
| R11 | Software con vulnerabilidades conocidas: nginx 1.24.0 | — | `nginx -v` → `nginx/1.24.0 (Ubuntu)`, paquete `1.24.0-2ubuntu7.18` | Confirmado: los backports de Ubuntu (`1.24.0-2ubuntu7.18`, noble-security) traen parche para todas las CVE listadas. Evidencia: `evidence/blue/nginx-patches.txt`. Sin versión más nueva disponible. |

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
- Ubuntu **backportea parches de seguridad** al paquete `1.24.0-2ubuntu7.18`. **Verificado 2026-09-25**: las 10 CVEs del registro están parcheadas en el changelog (ver `evidence/blue/nginx-patches.txt`); instalada = candidata, no hay actualización pendiente.
- Exposición práctica hoy: solo HTTP/1.1 en puerto 80, proxying a `127.0.0.1:8080`; `mp4`, `http2`, `grpc`, `charset` y `map` no se usan en la configuración actual.
- El banner no revela la versión (`server_tokens off`), por lo que un atacante externo no puede asociar estos CVEs al servidor por fingerprinting básico (ver R2).

## Riesgos a tratar en Laboratorio 3.2

- HTTPS + certificados (R1 parcial, H1/H4; además activa la superficie TLS de CVE-2025-23419 y CVE-2026-40701). → **TRATADO**: CA interna + HTTPS forzado; ambas CVE parcheadas en el paquete de Ubuntu.
- Autenticación con MFA y roles lectura/cambio (R7, H5/H6). → **TRATADO**: MFA TOTP + JWT + RBAC.
- Firma/integridad de la bitácora (R8 completo, H3). → **TRATADO**: hash chain + HMAC + endpoint de verificación.
- Actualización o confirmación de parches de nginx (R11). → **TRATADO**: todas las CVE del registro parcheadas en `1.24.0-2ubuntu7.18`.

## Riesgos residuales documentados (post Lab 3.2)

- Certificado emitido por CA interna (no reconocida globalmente): los clientes deben
  confiar en `MuvAutomation_Lab_CA`. Un certificado de CA pública (Let's Encrypt)
  queda como opción si se consigue dominio.
- JWT en `localStorage` del navegador (exposición a XSS) y token del stream SSE
  visible en `access.log` (query param). Ver `evidence/blue/auth-mfa-rbac.txt`.
- Clave HMAC de la bitácora en el propio servidor: un atacante con acceso total al
  host podría reescribir la cadena completa. Alternativa futura: Azure Key Vault.
- RabbitMQ con usuario por defecto (`guest/guest`) en localhost y semilla TOTP en
  claro en MongoDB: aceptado para entorno de laboratorio.

## Cómo se hizo

el servidor quedo en una maquina ubuntu en azure, con nginx sirviendo la pagina y un backend en el puerto 8080. al principio el sitio andaba solo por http, sin firewall y con varios puertos internos abiertos hacia afuera (rabbitmq y esas cosas). ademas un escaner de internet (zgrab) alcanzo a tocar la pagina al poco tiempo de publicarla, y eso nos confirmo que cualquiera podia entrar.

primero se configuro el firewall del servidor con ufw: se nego toda la entrada por defecto y solo se dejo pasar el ssh y los puertos 80 y 443. con eso quedaron cerrados los puertos internos que no deberian estar expuestos a internet.

despues se genero el certificado tls autofirmado con openssl, metiendo la ip publica del servidor dentro del certificado. quedo guardado en /etc/ssl/certs y la llave en /etc/ssl/private con permisos restringidos. en nginx se agrego un bloque nuevo para el puerto 443 con tls 1.2 y 1.3, y se movieron las reglas de la pagina y del proxy a un archivo aparte (un snippet) para no duplicarlas entre http y https.

el problema es que el deploy automatico de github borraba la config de nginx cada vez que se sube algo a la rama main, porque copiaba la version del repositorio que no tenia https. por eso se arreglo el script de instalacion y el workflow del deploy para que copien tambien el snippet, asi el https no se vuelve a perder en el proximo deploy. todo eso quedo en un commit de la rama lab-n03-parte-ii.

para que el navegador no se queje del certificado se agrego al almacen de confianza del usuario en windows. en http el navegador seguira mostrando que la pagina no es segura, porque http no cifra nada, eso se arregla de forma definitiva en el laboratorio siguiente con un certificado reconocido (lets encrypt o una ca interna).

tambien se revisaron los headers de seguridad (nosniff, frame options y referrer policy), se oculto la version de nginx y se bloqueo el acceso a rutas con punto como .git o .env. todo eso esta versionado en la carpeta nginx del repositorio.

lo que queda pendiente para el lab 3.2: la autenticacion con roles, el https con certificado de confianza y revisar si el paquete de nginx de ubuntu ya trae los parches de las vulnerabilidades que aparecen en r11.
