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
| F-05 | Dependencias vulnerables | 16 alertas de Dependabot (1 critical, 8 high) | Alto | `frontend/package-lock.json`, `backend/go.sum` | Actualizar dependencias con los PR de Dependabot | Abierto (seguimiento) |
| F-06 | Cabeceras de seguridad faltantes | ZAP: sin CSP, sin Permissions-Policy, sin COEP y contenido cacheable | Bajo | respuestas nginx (SPA y estáticos) | Agregar CSP y Permissions-Policy; COEP y cache documentados como riesgo residual | Corregido parcialmente (PR #2) |

## 4. Validación antes y después
- Evidencia inicial (captura o artefacto): `resultado-semgrep` (21 hallazgos) del run 39683754980; 2 alertas CodeQL; 16 alertas Dependabot.
- Pull request con corrección: PR #2 `security/remediacion-fsi` (pin de SHAs, permisos del workflow, `$host` de nginx, cabeceras CSP/Permissions-Policy y fix del artefacto ZAP).
- Evidencia de reescaneo: Semgrep 21 → 1 hallazgo (solo el falso positivo F-02); CodeQL y Gitleaks en verde; DAST: Permissions-Policy resuelta y CSP enviada (resta `style-src unsafe-inline`); sin FAIL en ningún re-run. Las 2 alertas de CodeQL cierran al reanalizar `main` tras el merge.
- Riesgo residual y decisión: F-02 aceptado (uso no criptográfico de `math/rand`); F-05 en seguimiento con los PR automáticos de Dependabot; F-06 residual: `style-src 'unsafe-inline'` (compatibilidad con la SPA Angular), COOP/CORP/COEP no aplicados (podrían romper la carga de recursos) y cacheo de estáticos en revisión.

## 5. Pentest autorizado
- Hipótesis: la superficie expuesta (nginx + API Go) mantiene configuraciones heredadas de la línea base sin endurecer.
- Entorno de pruebas aislado: staging propio `https://68.211.136.226`, escaneo pasivo ZAP baseline (sin ataques activos, `-m 1`, `-I`).
- Solicitudes inocuas y respuestas observadas (sin datos personales): crawl pasivo del sitio (/, /robots.txt, /sitemap.xml, JS/CSS). Antes: 61 PASS, 0 FAIL, 6 WARN-NEW (cache, CSP, cacheable, Permissions-Policy, app moderna, COEP). Después del PR #2: 0 FAIL y Permissions-Policy resuelta; CSP presente (aviso solo por `style-src unsafe-inline`).
- Resultado: validado (sin fallos) / requiere revisión: cabeceras de aislamiento (COOP/CORP/COEP) y cacheo quedan como riesgo residual documentado.
- Recomendación y prueba de regresión: reejecutar el baseline en cada cambio de nginx; evaluar quitar `'unsafe-inline'` de `style-src` y habilitar COOP/CORP tras validar que la SPA carga correctamente.
