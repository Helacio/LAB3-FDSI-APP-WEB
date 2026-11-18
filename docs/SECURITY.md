# Seguridad del proyecto — evidencia del laboratorio

## 1. Alcance autorizado
- Repositorio: https://github.com/Helacio/LAB3-FDSI-APP-WEB
- Rama evaluada y commit: `security/lab-fsi` (4a9f83b), integrada a `main` (a83da2b) vía PR #1
- URL de staging propio (si aplica): https://68.211.136.226
- Responsable / autorización: Tomas Quinceo y Hernán Sánchez / 21-10-2026
- Fuera de alcance: producción, sistemas de terceros, datos reales y pruebas destructivas.

## 2. Línea base y herramientas
| Herramienta | Tipo | Fecha | Resultado | Enlace a evidencia |
|---|---|---|---|---|
| CodeQL (Default setup) | SAST | 21-10-2026 | 2 alertas abiertas (warning): permisos de workflow sin restringir en `deploy.yml` | Pestaña Security → Code scanning |
| Semgrep | SAST | 21-10-2026 | 21 hallazgos (warning): 17 tags mutables de actions, 1 `math/rand`, 3 `$host` en nginx | Artefacto `resultado-semgrep` del run 39683754980 |
| Gitleaks / GitHub secret scanning | Secretos | 21-10-2026 | 0 secretos detectados | Job Gitleaks (run 39683754980) y pestaña Security |
| Dependabot | SCA | 21-10-2026 | 16 alertas: 1 critical, 8 high, 6 medium, 1 low | Pestaña Security → Dependabot |
| OWASP ZAP Baseline | DAST pasivo | 21-10-2026 | Antes: 0 FAIL, 6 WARN-NEW, 61 PASS. Después del PR #2: 0 FAIL, 7 alertas (CSP solo por `style-src unsafe-inline`, COOP/CORP/COEP, cache y app moderna) | Runs 39685116213 y 39686269263 (artefacto `informe-zap`) |

## 3. Hallazgos y gestión
| ID | Categoría | Descripción sin secretos | Riesgo | Archivo/ruta | Cambio propuesto | Estado |
|---|---|---|---|---|---|---|
| F-01 | Supply chain | Actions referenciadas con tag mutable (`@v4`, `@v1.0.3`, `@v3`...): el dueño puede republicar el tag | Medio | `.github/workflows/*.yml` | Anclar cada action al SHA completo del commit | Corregido (PR #2) |
| F-02 | Criptografía (falso positivo) | `math/rand/v2` en `status.go` | Bajo | `backend/status.go:9` | Ninguno: solo simula latencia/estados ponderados; los IDs usan `crypto/rand` | Aceptado como falso positivo |
| F-03 | Host header injection | `$host` (controlable por el atacante) en el redirect HTTP→HTTPS y en `proxy_set_header Host` | Bajo | `nginx/muvautomation.conf:20`, `nginx/snippets/muvautomation.conf:13,24` | Redirect al host canónico fijo y `Host $proxy_host` en el proxy | Corregido (PR #2) |
| F-04 | Permisos de workflow | `deploy.yml` sin bloque `permissions`: el token del job tiene más permisos de los necesarios (lo detecta CodeQL) | Medio | `.github/workflows/deploy.yml` | `permissions: contents: read` | Corregido (PR #2) |
| F-05 | Dependencias vulnerables | npm audit: 14 (2 critical, 7 high: `piscina`, `vite`, `@angular/router`…); govulncheck: 12 alcanzables (stdlib Go 1.27.1 `net/http`, `crypto/tls`, `crypto/x509` y `klauspost/compress`) | Alto | `frontend/package-lock.json`, `backend/go.mod` | Angular 22.1 → 22.2.7, lockfile regenerado; Go 1.27.1 → 1.27.4 y dependencias actualizadas; Dependabot semanal | Corregido (0 en npm audit y govulncheck) |
| F-06 | Cabeceras de seguridad faltantes | ZAP: sin CSP, sin Permissions-Policy, sin COEP y contenido cacheable | Bajo | respuestas nginx (SPA y estáticos) | CSP, Permissions-Policy, COOP/CORP/COEP, `expires -1` en la SPA y `Cache-Control: no-store` en la API | Corregido (resta `style-src 'unsafe-inline'`) |
| F-07 | A07 Fallas de autenticación | El rate limit de login usaba el primer valor de `X-Forwarded-For`, controlado por el cliente: rotándolo se evadía el bloqueo | Alto | `backend/auth.go` (`ipCliente`) | Usar `RemoteAddr`; confiar en `X-Real-IP` solo si la conexión viene de nginx local; `limit_req` en nginx para `/api/auth/` | Corregido |
| F-08 | A07 Fallas de autenticación | `/api/auth/verify` sin límite de intentos ni anti-replay: fuerza bruta del TOTP (6 dígitos) durante los 2 min del ticket y reutilización del mismo código | Alto | `backend/auth.go` (`handleVerify`) | 5 intentos/5 min por usuario; consumo atómico del paso TOTP (`ultimoPasoTotp`); ticket `mfa` no sirve si el usuario no enroló y viceversa | Corregido |
| F-09 | A02/A07 Secretos embebidos | `JWT_SECRET` y `BITACORA_KEY` tenían valores por defecto en el código: si faltaba la variable, cualquiera podía firmar JWT válidos y re-sellar la bitácora | Crítico | `backend/main.go` | Fail closed: el backend no arranca sin secretos de ≥ 32 caracteres | Corregido |
| F-10 | A07 Credenciales por defecto | Contraseñas de los usuarios semilla escritas en el código y en el README | Alto | `backend/auth.go` (`seedUsers`), `README.md` | Contraseñas desde `SEED_*_PASSWORD`, generadas al azar por `install.sh`; eliminadas del README | Corregido (cambiar las contraseñas existentes en la VM: siguen en el historial de git) |
| F-11 | A01/A09 Token en la URL | `requireAuth` aceptaba `?token=` en **todas** las rutas: el JWT de acceso quedaba en logs de nginx, historial y `Referer` | Medio | `backend/auth.go`, `frontend/src/app/api.service.ts` | Solo cabecera `Authorization`; el SSE usa un ticket `tipo=sse` de 1 minuto | Corregido |
| F-12 | A03 Inyección / validación | `POST /api/sessions` aceptaba cualquier texto como comando y dispositivo (el catálogo no se aplicaba en el servidor); cuerpos JSON sin límite | Medio | `backend/main.go` | Allowlist contra `commands` y `devices`; `MaxBytesReader` 16 KiB, `DisallowUnknownFields`, `client_max_body_size` en nginx | Corregido |
| F-13 | A01 Control de acceso | `/api/inventory.txt` público (H6 del modelo de amenazas, pendiente desde Lab 3) | Medio | `backend/main.go` | Requiere token | Corregido |
| F-14 | A05/A09 Configuración y registro | Backend escuchando en todas las interfaces; URI con credenciales (`guest:guest`) en logs; `err.Error()` devuelto al cliente; sin registro de eventos de autenticación | Medio | `backend/main.go`, `backend/status.go`, `deploy/muvautomation.service` | Bind a `127.0.0.1`; `redactarURL`; mensajes genéricos; log `audit evento=...` (login/MFA/403/comando rechazado); hardening systemd | Corregido |
| F-15 | Pipeline | Semgrep y ZAP con `\|\| true`: el pipeline siempre quedaba en verde | Medio | `.github/workflows/*.yml` | Semgrep `--error` (versión fijada), govulncheck, `npm audit --audit-level=high`, `go test`, `nginx -t`; ZAP con `.zap/rules.tsv` y puerta por código de salida; imagen ZAP por digest; el deploy también exige tests y SCA | Corregido |

## 4. Validación antes y después
- Evidencia inicial (captura o artefacto): `resultado-semgrep` (21 hallazgos) del run 39683754980; 2 alertas CodeQL; 16 alertas Dependabot.
- Pull request con corrección: PR #2 `security/remediacion-fsi` (pin de SHAs, permisos del workflow, `$host` de nginx, cabeceras CSP/Permissions-Policy y fix del artefacto ZAP).
- Evidencia de reescaneo: Semgrep 21 → 1 hallazgo (solo el falso positivo F-02); CodeQL y Gitleaks en verde; DAST: Permissions-Policy resuelta y CSP enviada (resta `style-src unsafe-inline`); sin FAIL en ningún re-run. Las 2 alertas de CodeQL cierran al reanalizar `main` tras el merge.
- Riesgo residual y decisión: F-02 aceptado (uso no criptográfico de `math/rand`); F-05 en seguimiento con los PR automáticos de Dependabot; F-06 residual: `style-src 'unsafe-inline'` (compatibilidad con la SPA Angular), COOP/CORP/COEP no aplicados (podrían romper la carga de recursos) y cacheo de estáticos en revisión.

## 4.1 Segunda iteración (rama `security/owasp-sast-dast`)
- Antes: Semgrep 1 hallazgo (F-02) con pipeline sin puerta; npm audit 14 vulnerabilidades (2 critical); govulncheck 12 alcanzables; revisión manual OWASP: F-07 a F-14.
- Después (local, mismas reglas que el CI): Semgrep **0** (F-02 suprimido con `nosemgrep` justificado), npm audit **0**, govulncheck **0**, `go test` con pruebas de regresión de cada control (`backend/security_test.go`).
- Pendiente de evidencia en GitHub: ejecutar el workflow DAST tras el despliegue y adjuntar `informe-zap`.
- Riesgo residual: `style-src 'unsafe-inline'` (Angular inyecta estilos de componentes); MongoDB y RabbitMQ sin autenticación propia pero solo en `127.0.0.1`; el token de acceso se guarda en `localStorage` (mitigado por CSP estricta de scripts y TTL de 15 min); las contraseñas semilla antiguas siguen en el historial de git y deben rotarse.

## 5. Pentest autorizado
- Hipótesis: la superficie expuesta (nginx + API Go) mantiene configuraciones heredadas de la línea base sin endurecer.
- Entorno de pruebas aislado: staging propio `https://68.211.136.226`, escaneo pasivo ZAP baseline (sin ataques activos, `-m 1`, `-I`).
- Solicitudes inocuas y respuestas observadas (sin datos personales): crawl pasivo del sitio (/, /robots.txt, /sitemap.xml, JS/CSS). Antes: 61 PASS, 0 FAIL, 6 WARN-NEW (cache, CSP, cacheable, Permissions-Policy, app moderna, COEP). Después del PR #2: 0 FAIL y Permissions-Policy resuelta; CSP presente (aviso solo por `style-src unsafe-inline`).
- Resultado: validado (sin fallos) / requiere revisión: cabeceras de aislamiento (COOP/CORP/COEP) y cacheo quedan como riesgo residual documentado.
- Recomendación y prueba de regresión: reejecutar el baseline en cada cambio de nginx; evaluar quitar `'unsafe-inline'` de `style-src` y habilitar COOP/CORP tras validar que la SPA carga correctamente.
