# Inventario de Superficie de Ataque — Laboratorio 3 (Paso 9)

**Fecha de evaluación:** 2026-09-16  
**Objetivo autorizado:** `http://68.211.136.226/` (`TARGET_IP=68.211.136.226`)  
**Rol evaluador:** Red Team  

## 1. Tabla de Inventario de Superficie de Ataque

| Elemento | Dato observado | Riesgo / Pregunta analítica | Mapeo STRIDE |
|---|---|---|---|
| **Host / IP** | `68.211.136.226` (instancia en la nube, interfaz pública) | ¿Está limitado al CIDR autorizado? Durante la prueba inicial no había filtrado UFW activo, permitiendo visibilidad desde cualquier IP pública (corregido en Paso 5). | Information Disclosure |
| **Puerto** | `80/TCP` (servicio HTTP en texto claro) | ¿Por qué el tráfico no tiene confidencialidad? Porque HTTP transmite peticiones, cabeceras, rutas y respuestas sin cifrado TLS en la capa de transporte. | H1 (Information Disclosure), H4 (Tampering) |
| **Servidor Web** | `nginx 1.24.0 (Ubuntu)` | ¿Se revela la versión? Sí, el banner `Server` expone el software y la versión exacta, facilitando fingerprinting y correlación con CVEs conocidos (ej. CVE-2026-9256, CVE-2024-7347). | H2 (Information Disclosure) |
| **Ruta `/`** | Código `200 OK`, HTML retornado | ¿Expone información innecesaria? Expone título del portal, entorno (`LAB`), propietario (`Blue Team`) y enlaces a recursos internos sin requerir autenticación. | H6 (Elevation of Privilege) |
| **Archivo público** | `/public-inventory.txt` (código `200 OK`, texto plano) | ¿Qué metadatos entrega? Revela nombres de host (`RTR-LAB-01`, `SW-LAB-01`, etc.), tipos de dispositivo, roles en la topología, direcciones IP de gestión y comandos permitidos. | H1, H6 (Information Disclosure) |

## 2. Interpretación de Resultados de Reconocimiento (Paso 8)

1. **Protocolo y Puerto:**
   - El puerto `80/tcp` está en estado `open`. Al no emplear TLS (`https`), cualquier dispositivo intermediario en el canal de red puede inspeccionar y alterar los paquetes en tránsito.
2. **Identificación de Tecnología:**
   - La cabecera `Server: nginx/1.24.0 (Ubuntu)` permite a un atacante identificar de inmediato la versión del software base del servidor sin necesidad de recurrir a técnicas avanzadas de banner grabbing.
3. **Ausencia de Cabeceras Defensivas:**
   - En la respuesta inicial no existen cabeceras como `X-Content-Type-Options`, `X-Frame-Options` ni `Referrer-Policy`.
