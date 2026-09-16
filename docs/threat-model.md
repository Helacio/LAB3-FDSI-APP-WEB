# Modelo de amenazas STRIDE — Laboratorio 3

Línea base: sitio estático publicado por HTTP, sin autenticación, dentro del CIDR autorizado del laboratorio.

| ID | Categoría STRIDE | Hipótesis técnica | Validación (evidencia) | Estado |
|---|---|---|---|---|
| H1 | Information Disclosure | HTTP permite observar contenido, rutas y metadatos en tránsito. | PCAP filtrada con `tcpdump`/Wireshark: se ven `GET /` y `GET /public-inventory.txt` en claro. | Confirmada → riesgo aceptado hasta Lab 4 |
| H2 | Information Disclosure | Headers y banners revelan tecnología (servidor y versión). | `curl -I` antes del hardening: `Server: nginx/1.x.x`. ZAP pasivo. | Confirmada → corregida con `server_tokens off` |
| H3 | Repudiation | Sin correlación temporal ni identidad, el equipo no puede atribuir solicitudes. | Comparar timestamp del comando Red Team con `access.log`; log con fecha y User-Agent. | Confirmada → mitigada con línea de tiempo Purple Team |
| H4 | Tampering | Sin TLS, un intermediario podría alterar el tráfico. No se ejecuta MITM. | Ausencia de HTTPS (puerto 80 en claro) + headers que no protegen integridad. | Demostrada (sin explotar) → pendiente Lab 4 |
| H5 | Spoofing | Sin autenticación, cualquier host del CIDR autorizado puede hacerse pasar por operador legítimo. | Solicitudes desde Kali y navegador indistinguibles en `access.log` salvo por User-Agent. | Confirmada → pendiente Lab 4 (identidad/MFA) |
| H6 | Elevation of Privilege | El inventario público se entrega sin control de roles: cualquier visitante accede a la misma información. | `curl $TARGET_URL/public-inventory.txt` devuelve 200 sin credenciales. | Confirmada → pendiente Lab 4 (roles lectura/cambio) |

## Priorización

- **Lab 3 (corregido):** H2 (banner de versión), H3 (parcial: correlación temporal), reducción de exposición de H6 (recortar inventario público).
- **Lab 4 (abierto a propósito):** H1 (HTTPS), H4 (TLS), H5 (autenticación), H6 completo (autorización).
