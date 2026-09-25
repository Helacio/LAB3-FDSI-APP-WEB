# Resumen Lab 3.2 — Purple Team (2026-09-25)

Ciclo completo: Diseñar → Construir → Atacar → Detectar → Corregir → Verificar.

## Antes (Lab 3) vs Después (Lab 3.2)

| Aspecto | Antes | Después |
|---|---|---|
| Transporte | HTTP plano en 80 | HTTPS forzado (301 + HSTS), TLS 1.2/1.3 solo ciphers A, CA interna + CRL |
| Identidad | Operador por texto libre (body) | Login + MFA TOTP + JWT 15 min; operador sale del token |
| Autorización | Ninguna | RBAC: `lector` (GET) vs `cambiador` (POST) |
| Bitácora | Texto modificable sin detección | Hash chain + HMAC-SHA256 + `/verify` detecta alteración |
| Login abusivo | Sin límite | 10 intentos/5 min por IP → 429 |
| Banner/headers | Versión expuesta, sin headers | server_tokens off + nosniff/DENY/no-referrer |
| Firewall | UFW inactivo | deny incoming, solo 22/80/443 |
| CVEs nginx | sin verificar | 10/10 parcheadas en 1.24.0-2ubuntu7.18 |
| CORS | `*` | eliminado (mismo origen) |

## Retest externo (Purple)

- `https://68.211.136.226/` con validación completa (revocación por CRL) → 200
- `http://...` → 301 a HTTPS; HSTS `max-age=86400`
- nmap `-sV -p 443`: `ssl/http nginx` sin versión
- ssl-enum-ciphers: todo grado A (ver `evidence/retest/ssl_enum_ciphers.txt`)
- Auth: 401 sin token / 403 lector en POST / 201 cambiador / 429 tras 10 fallos
- Bitácora: alteración en Mongo detectada (`primerRompimiento` exacto)

## Checklist de cierre

- [x] Solo se tocó el objetivo autorizado (68.211.136.226)
- [x] Datos ficticios (usuarios demo, dispositivos RFC 5737)
- [x] Evidencia antes/después en `evidence/blue/` y `evidence/retest/`
- [x] Riesgos actualizados en `risk-register.md` (R1, R7, R8, R11 cerrados)
- [x] Secretos fuera del repo (`/etc/muvautomation/secrets.env`)
- [x] Deploy conserva la seguridad (install.sh + workflow + snippet nginx)

## Riesgos residuales

CA interna (clientes deben confiar), JWT en localStorage, token SSE en logs,
clave HMAC en el host, RabbitMQ guest/guest en localhost. Detalle en
`risk-register.md`.
