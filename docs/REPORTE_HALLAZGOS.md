# Informe técnico de hallazgos

**Proyecto:** LAB3-FDSI-APP-WEB (Portal de Ejecución Remota Segura)
**Integrantes:** Tomas Quinceo y Hernán Sánchez
**Repositorio / commit:** https://github.com/Helacio/LAB3-FDSI-APP-WEB · `security/lab-fsi` (4a9f83b) → `main` (a83da2b)
**Entorno autorizado:** https://68.211.136.226 (staging propio, VM Azure del laboratorio)
**Fecha:** 21-10-2026

## Resumen ejecutivo
El análisis de la línea base encontró 21 hallazgos de Semgrep (todos warning), 2 alertas de CodeQL, 16 alertas de Dependabot y 6 tipos de warning en el DAST pasivo (sin fallos). Los tres riesgos principales son: (1) supply chain por actions de GitHub referenciadas con tags mutables, (2) permisos del workflow de despliegue más amplios de lo necesario y (3) dependencias con vulnerabilidades conocidas. Se corrigieron los puntos 1 y 2, el uso del `$host` controlable en nginx y las cabeceras de seguridad faltantes; el uso de `math/rand` se clasificó como falso positivo y las dependencias quedan en seguimiento con Dependabot.

## Metodología
Identificación del alcance → SAST → detección de secretos → SCA → DAST pasivo → validación manual no destructiva → corrección → reescaneo.

## Hallazgo F-01
- **Título:** Actions de GitHub referenciadas con tag mutable (riesgo de supply chain)
- **Categoría OWASP/CWE (si corresponde):** CWE-1357 / A06:2021 Componentes vulnerables
- **Severidad y justificación:** Media. Si el dueño de una action republica el tag, el pipeline ejecutaría código distinto al revisado.
- **Evidencia minimizada, sin credenciales ni datos personales:** `semgrep.json` — 17 ocurrencias de `github-actions-mutable-action-tag` en los 3 workflows (`deploy.yml`, `seguridad-estatica.yml`, `dast-staging.yml`), p. ej. `uses: actions/checkout@v4`.
- **Impacto en confidencialidad, integridad o disponibilidad:** compromiso potencial del pipeline (y del acceso a la VM de despliegue).
- **Pasos de validación segura en entorno propio:** revisión del reporte de Semgrep y de la lista de `uses:` de cada workflow.
- **Corrección (commit / pull request):** PR #2 — todos los `uses` anclados al SHA completo del commit (`actions/checkout@11d5960...`, `appleboy/ssh-action@029f5b4...`, etc.).
- **Resultado del reescaneo:** 0 ocurrencias tras el PR #2 (Semgrep pasó de 21 a 1 hallazgo total).

## Hallazgo F-02
- **Título:** Uso de `math/rand` (falso positivo)
- **Categoría OWASP/CWE (si corresponde):** CWE-330 (uso de aleatoriedad insuficiente)
- **Severidad y justificación:** Baja / informativa. El generador solo se usa para simular latencia y estados ponderados de los dispositivos ficticios; los identificadores usan `crypto/rand`.
- **Evidencia minimizada, sin credenciales ni datos personales:** `backend/status.go:9` importa `math/rand/v2`; `newID()` (misma carpeta) usa `crypto/rand` + `encoding/hex`.
- **Impacto en confidencialidad, integridad o disponibilidad:** ninguno (no produce secretos ni tokens).
- **Pasos de validación segura en entorno propio:** lectura del código y del reporte de Semgrep.
- **Corrección (commit / pull request):** sin cambio de código; se documenta la justificación.
- **Resultado del reescaneo:** la regla seguirá apareciendo (falso positivo aceptado).

## Hallazgo F-03
- **Título:** `$host` controlable por el atacante en nginx
- **Categoría OWASP/CWE (si corresponde):** CWE-601 / host header injection
- **Severidad y justificación:** Baja en este entorno (sitio de un solo host servido por IP), pero es una mala práctica explotable si se añaden dominios.
- **Evidencia minimizada, sin credenciales ni datos personales:** `return 301 https://$host$request_uri;` en `nginx/muvautomation.conf:20` y `proxy_set_header Host $host;` en `nginx/snippets/muvautomation.conf:13,24`.
- **Impacto en confidencialidad, integridad o disponibilidad:** redirecciones abiertas (phishing) y encabezados Host falsos hacia el backend.
- **Pasos de validación segura en entorno propio:** `curl -H "Host: atacante.example"` sobre el puerto 80 y observación del `Location`.
- **Corrección (commit / pull request):** PR #2 — redirect al host canónico fijo y `proxy_set_header Host $proxy_host;` (el backend no depende del `Host`).
- **Resultado del reescaneo:** 0 ocurrencias de `$host`/`$http_host` tras el PR #2 (Semgrep 21 → 1).

## Hallazgo F-04
- **Título:** Workflow de despliegue sin permisos explícitos
- **Categoría OWASP/CWE (si corresponde):** CWE-250 (ejecución con privilegios innecesarios)
- **Severidad y justificación:** Media. El `GITHUB_TOKEN` del job obtenía más permisos de los requeridos (principio de mínimo privilegio).
- **Evidencia minimizada, sin credenciales ni datos personales:** 2 alertas CodeQL "Workflow does not contain permissions" sobre `.github/workflows/deploy.yml`.
- **Impacto en confidencialidad, integridad o disponibilidad:** mayor daño si un step fuera comprometido.
- **Pasos de validación segura en entorno propio:** revisión de la pestaña Security → Code scanning.
- **Corrección (commit / pull request):** PR #2 — `permissions: contents: read` a nivel de workflow.
- **Resultado del reescaneo:** CodeQL sin alertas nuevas en el PR #2; las 2 alertas cierran al reanalizar `main`.

## Hallazgo F-05
- **Título:** Dependencias con vulnerabilidades conocidas
- **Categoría OWASP/CWE (si corresponde):** A06:2021 Componentes vulnerables
- **Severidad y justificación:** Alta (1 critical, 8 high, 6 medium, 1 low según Dependabot).
- **Evidencia minimizada, sin credenciales ni datos personales:** pestaña Security → Dependabot del repositorio.
- **Impacto en confidencialidad, integridad o disponibilidad:** explotación de vulnerabilidades conocidas en librerías de Angular/Go.
- **Pasos de validación segura en entorno propio:** revisión de alertas y de los PR automáticos de actualización.
- **Corrección (commit / pull request):** aceptar los PR de actualización de Dependabot y volver a correr build + SAST.
- **Resultado del reescaneo:** en seguimiento.

## Hallazgo F-06
- **Título:** Cabeceras de seguridad faltantes en las respuestas (DAST pasivo)
- **Categoría OWASP/CWE (si corresponde):** A05:2021 Configuración incorrecta de seguridad
- **Severidad y justificación:** Baja. Sin fallos (0 FAIL), pero ZAP marcó 6 tipos de warning: Cache-control [10015], CSP no seteada [10038], contenido cacheable [10049], Permissions-Policy no seteada [10063], app moderna [10109] y COEP [90004].
- **Evidencia minimizada, sin credenciales ni datos personales:** log del run 39685116213 del workflow "DAST pasivo - staging autorizado" (61 PASS, 6 WARN-NEW, 0 FAIL sobre `https://68.211.136.226`).
- **Impacto en confidencialidad, integridad o disponibilidad:** menor superficie de defensa (XSS, clickjacking ya mitigado, fuga de funcionalidades del navegador).
- **Pasos de validación segura en entorno propio:** reejecutar el baseline tras el cambio y confirmar que CSP/Permissions-Policy desaparecen de los warnings.
- **Corrección (commit / pull request):** PR #2 — `Content-Security-Policy` y `Permissions-Policy` en `nginx/snippets/muvautomation.conf`; además se corrigió el permiso de escritura del contenedor ZAP para que genere el artefacto `informe-zap`.
- **Resultado del reescaneo:** pendiente del re-run del DAST tras el merge (COEP y cacheo quedan como riesgo residual documentado).

## Reflexión final
Las herramientas automatizadas detectaron rápido los problemas de configuración del pipeline (tags mutables, permisos) que una revisión manual pasa por alto, y Dependabot evidenció deuda de dependencias. Sin embargo, Semgrep generó ruido (falso positivo de `math/rand`) y el DAST pasivo por sí solo no valida lógica de negocio: el triage humano sigue siendo necesario. Para el futuro queda integrar el SAST como puerta (fail on error) una vez hecho el triage, y automatizar las actualizaciones de dependencias en el ciclo semanal.
