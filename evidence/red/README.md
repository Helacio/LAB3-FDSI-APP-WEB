# Evidencias Red Team — Laboratorio 3

**Fase:** Fase C — Atacar de manera controlada  
**Alcance autorizado:** `68.211.136.226`  
**IP Origen de pruebas:** `181.63.24.35`  
**Objetivo:** `http://68.211.136.226/` (`TARGET_URL`)  
**Fecha de inicio:** `2026-09-16T22:51:00Z` (verificado en [start.txt](start.txt))  

---

## 1. Registro de Ejecución de Pruebas Ofensivas Autorizadas

Conforme a la Sección 2 de la guía (*"Antes de ejecutar una prueba, documente: amenaza, hipótesis, comando, resultado esperado y evidencia"*):

| ID | Amenaza (STRIDE) | Hipótesis técnica | Comando ejecutado | Timestamp (UTC) | Resultado esperado | Resultado obtenido | Archivo de evidencia |
|---|---|---|---|---|---|---|---|
| **H1** | Information Disclosure | HTTP expone rutas, cabeceras y contenido sin confidencialidad. | `curl "http://68.211.136.226/public-inventory.txt"` | 2026-09-23 00:10:40 | Tráfico y contenido legibles en tránsito. | Payload de routers/switches visible en claro. | [http_cleartext_analysis.md](http_cleartext_analysis.md), [lab3-http.pcap](../blue/lab3-http.pcap) |
| **H2** | Information Disclosure | El servidor revela su tecnología y versión en banners y cabeceras. | `nmap -Pn -sV -p 80 68.211.136.226 -oA evidence/red/nmap_port80` | 2026-09-16 22:51:00 | Identificación de producto y versión. | `nginx 1.24.0 (Ubuntu)` expuesto abiertamente. | [nmap_port80.nmap](nmap_port80.nmap), [nmap_port80.xml](nmap_port80.xml) |
| **H2** | Information Disclosure | Cabeceras de seguridad ausentes permiten fingerprinting y riesgos cliente. | `curl -I "http://68.211.136.226/public-inventory.txt"` | 2026-09-16 22:52:15 | Ausencia de nosniff, framing y CSP. | Cabeceras ausentes confirmadas. | [curl_headers.txt](curl_headers.txt), [curl_home.txt](curl_home.txt) |
| **H2 / H4** | Information Disclosure / Tampering | Inspección pasiva revela fallas de hardening en HTTP sin alterar el servicio. | OWASP ZAP 2.17.0 (Manual Explore pasivo) | 2026-09-16 23:03:34 | Detección de cabeceras ausentes y HTTP claro. | Alertas por `X-Frame-Options`, `X-Content-Type-Options` y server tokens. | [reports/zap-passive/](../../reports/zap-passive/README.md) |
| **H3** | Repudiation | Sin trazabilidad o correlación temporal, las solicitudes no se atribuyen. | Correlación con Blue Team | 2026-09-23 00:10:39 | Registro de IP y User-Agent en logs. | Peticiones registradas en `access.log`. | [access_tail.txt](../blue/access_tail.txt) |

---

## 2. Índice de Archivos de Evidencia en este Directorio

1. **[start.txt](start.txt):** Timestamp UTC de inicio del ejercicio.
2. **[nmap_port80.nmap](nmap_port80.nmap), [nmap_port80.gnmap](nmap_port80.gnmap), [nmap_port80.xml](nmap_port80.xml):** Escaneo de Nmap en los 3 formatos estándar generados por `-oA`.
3. **[curl_home.txt](curl_home.txt):** Petición `curl -i` a la raíz demostrando el código 200 y cabeceras antes del hardening.
4. **[curl_headers.txt](curl_headers.txt):** Petición `curl -I` sobre el recurso público.
5. **[attack_surface.md](attack_surface.md):** Tabla formal de Inventario de Superficie de Ataque (Paso 9 de la guía).
6. **[http_cleartext_analysis.md](http_cleartext_analysis.md):** Análisis y demostración del tráfico HTTP en texto claro (Paso 11 de la guía).
7. **[Reporte OWASP ZAP](../../reports/zap-passive/README.md):** Resultados de la inspección pasiva en `reports/zap-passive/`.
