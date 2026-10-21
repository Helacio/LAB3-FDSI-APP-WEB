# Informe técnico de hallazgos

**Proyecto:** LAB3-FDSI-APP-WEB (Portal de Ejecución Remota Segura)
**Integrantes:** Tomas Quinceo y Hernán Sánchez
**Repositorio / commit:** https://github.com/Helacio/LAB3-FDSI-APP-WEB · `security/lab-fsi` (4a9f83b) → `main` (a83da2b)
**Entorno autorizado:** https://68.211.136.226 (staging propio, VM Azure del laboratorio)
**Fecha:** 21-10-2026

## Resumen ejecutivo
El análisis de la línea base encontró 21 hallazgos de Semgrep (todos warning), 2 alertas de CodeQL y 16 alertas de Dependabot. Los tres riesgos principales son: (1) supply chain por actions de GitHub referenciadas con tags mutables, (2) permisos del workflow de despliegue más amplios de lo necesario y (3) dependencias con vulnerabilidades conocidas. Se corrigieron los puntos 1 y 2, junto con el uso del `$host` controlable en nginx; el uso de `math/rand` se clasificó como falso positivo y las dependencias quedan en seguimiento con Dependabot.

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
- **Resultado del reescaneo:** pendiente del run del PR #2.

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
- **Resultado del reescaneo:** pendiente del run del PR #2.

## Hallazgo F-04
- **Título:** Workflow de despliegue sin permisos explícitos
- **Categoría OWASP/CWE (si corresponde):** CWE-250 (ejecución con privilegios innecesarios)
- **Severidad y justificación:** Media. El `GITHUB_TOKEN` del job obtenía más permisos de los requeridos (principio de mínimo privilegio).
- **Evidencia minimizada, sin credenciales ni datos personales:** 2 alertas CodeQL "Workflow does not contain permissions" sobre `.github/workflows/deploy.yml`.
- **Impacto en confidencialidad, integridad o disponibilidad:** mayor daño si un step fuera comprometido.
- **Pasos de validación segura en entorno propio:** revisión de la pestaña Security → Code scanning.
- **Corrección (commit / pull request):** PR #2 — `permissions: contents: read` a nivel de workflow.
- **Resultado del reescaneo:** pendiente del run del PR #2.

## Hallazgo F-05
- **Título:** Dependencias con vulnerabilidades conocidas
- **Categoría OWASP/CWE (si corresponde):** A06:2021 Componentes vulnerables
- **Severidad y justificación:** Alta (1 critical, 8 high, 6 medium, 1 low según Dependabot).
- **Evidencia minimizada, sin credenciales ni datos personales:** pestaña Security → Dependabot del repositorio.
- **Impacto en confidencialidad, integridad o disponibilidad:** explotación de vulnerabilidades conocidas en librerías de Angular/Go.
- **Pasos de validación segura en entorno propio:** revisión de alertas y de los PR automáticos de actualización.
- **Corrección (commit / pull request):** aceptar los PR de actualización de Dependabot y volver a correr build + SAST.
- **Resultado del reescaneo:** en seguimiento.

## Reflexión final
Las herramientas automatizadas detectaron rápido los problemas de configuración del pipeline (tags mutables, permisos) que una revisión manual pasa por alto, y Dependabot evidenció deuda de dependencias. Sin embargo, Semgrep generó ruido (falso positivo de `math/rand`) y el DAST pasivo por sí solo no valida lógica de negocio: el triage humano sigue siendo necesario. Para el futuro queda integrar el SAST como puerta (fail on error) una vez hecho el triage, y automatizar las actualizaciones de dependencias en el ciclo semanal.
