# Demostración de Visibilidad de HTTP en Tránsito (Paso 11)

**Objetivo:** Demostrar por qué el protocolo HTTP expone metadatos y contenido en tránsito (Hipótesis STRIDE H1 y H4).

## 1. Procedimiento de la Prueba
1. En el servidor (Blue Team), se inició una captura de paquetes limitada a 60 segundos sobre el puerto 80:
   ```bash
   sudo timeout 60 tcpdump -i any -nn -s0 -w /tmp/lab3-http.pcap 'tcp port 80'
   ```
2. Desde la estación de Red Team (`181.63.24.35`), se enviaron las peticiones HTTP autorizadas:
   ```bash
   curl "http://68.211.136.226/"
   curl "http://68.211.136.226/public-inventory.txt"
   ```

## 2. Evidencia Capturada

- **Archivo de captura PCAP:** [evidence/blue/lab3-http.pcap](../blue/lab3-http.pcap)
- **Captura Wireshark:** [evidence/blue/Captura desde 2026-09-16 18-00-01.png](../blue/Captura%20desde%202026-09-16%2018-00-01.png)

## 3. Datos Observados en Tránsito sin Cifrado

Al inspeccionar los paquetes TCP en Wireshark aplicando el filtro `http`:
1. **Petición del cliente expuesta en claro:**
   - Método: `GET / HTTP/1.1`
   - Host destino: `Host: 68.211.136.226`
   - User-Agent del cliente: `User-Agent: curl/8.21.0`
   - Recurso solicitado: `GET /public-inventory.txt HTTP/1.1`
2. **Respuesta del servidor expuesta en claro:**
   - Código de estado: `HTTP/1.1 200 OK`
   - Cabeceras del servidor: `Server: nginx`, tipos de contenido, fecha y longitud.
   - Cuerpo de la respuesta (Payload): El contenido completo del archivo de inventario con todos los nombres de routers y switches ficticios es completamente legible en texto plano en la capa de aplicación.

## 4. Conclusión de Seguridad
- **H1 (Information Disclosure):** Confirmada. Cualquier observador en el camino de red (ISP, Wi-Fi intermediario, router de tránsito) puede leer sin esfuerzo el contenido transmitido y las rutas solicitadas.
- **H4 (Tampering):** Demostrada. Al no contar con verificación de integridad criptográfica (TLS MAC / HMAC), un intermediario (Man-in-the-Middle) tiene la capacidad técnica de alterar las respuestas HTTP antes de que lleguen al cliente.
- **Riesgo:** Permanece como riesgo aceptado en el Laboratorio 3 y será resuelto mediante TLS/HTTPS y certificados en el Laboratorio 4.
