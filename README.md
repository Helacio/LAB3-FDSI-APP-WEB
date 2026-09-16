# LAB3-FDSI-APP-WEB

Aplicación web del **Laboratorio 3** (Secure Product Challenge · FDSI).
Tema: **Portal de Ejecución Remota Segura** para routers y switches.

Stack: **Angular** (frontend) + **Go** (API) + **MongoDB** (NoSQL) + **RabbitMQ** (colas y SSE) sobre **nginx** en HTTP.

## Funcionamiento

El portal permite consultar el inventario y el catálogo de comandos, registrar
órdenes en una **bitácora persistida en MongoDB**, y lanzar una **revisión de estado
de los routers** cuyo progreso llega en vivo por **Server-Sent Events (SSE)**.
Se publica por HTTP (puerto 80), sin autenticación, como línea base del Laboratorio 3
(Red Team + Blue Team). El producto es *deliberadamente incompleto*: HTTPS, identidad
y roles llegan en el Laboratorio 4.

## Arquitectura

```
Navegador ──HTTP:80──> nginx ──/ ────────────────> /var/www/muvautomation (Angular)
                            ├──/api/ ────────────> 127.0.0.1:8080 (Go) ──> MongoDB 127.0.0.1:27017
                            └──/api/status/stream (SSE) ── Go <── RabbitMQ 127.0.0.1:5672
```

### Revisión de estados (RabbitMQ + SSE)

1. `POST /api/status/checks` crea un `runId` y publica un job por dispositivo en la
   cola durable **`router.status.jobs`**.
2. Un pool de workers (mismo binario Go) consume los jobs, ejecuta un chequeo
   **simulado** (los equipos del lab son ficticios), guarda el resultado en
   `status_checks` y actualiza `devices.estado`.
3. Cada resultado se publica en el exchange **fanout `router.status.events`**.
4. El endpoint SSE crea una cola exclusiva ligada al exchange, por lo que cada
   navegador conectado recibe el mismo progreso en vivo.

Además del botón manual, un **scheduler** dispara una revisión periódica
(`STATUS_INTERVAL`, por defecto `60s`; `0` lo desactiva).

## Estructura del repositorio

| Ruta | Descripción |
|---|---|
| `frontend/` | Aplicación Angular (standalone, zoneless). |
| `backend/` | API REST en Go. `main.go`, `status.go` (workers/scheduler), `broker.go` (RabbitMQ) y `sse.go`. |
| `nginx/muvautomation.conf` | vhost de nginx: SPA, proxy `/api` y SSE sin buffering. |
| `deploy/muvautomation.service` | Unit systemd del backend. |
| `deploy/install.sh` | Aprovisiona nginx, MongoDB, RabbitMQ, el binario y el servicio en la VM. |
| `.github/workflows/deploy.yml` | CI/CD: build de Angular y Go, y despliegue a la VM. |
| `evidence/` | Reglas de detección del Blue Team. |

## Requisitos

- Node.js 24+ y npm
- Go 1.27+
- MongoDB 8.0
- RabbitMQ 3.x (o 4.x)

## Arranque local

1. **MongoDB** (con una carpeta de datos local):

   ```bash
   mkdir -p /tmp/muv-mongo
   mongod --dbpath /tmp/muv-mongo --bind_ip 127.0.0.1 --port 27017
   ```

2. **RabbitMQ** (servicio del sistema):

   ```bash
   sudo apt-get install -y rabbitmq-server
   sudo systemctl enable --now rabbitmq-server
   ```

3. **Backend Go** (siembra la base de datos al primer arranque si está vacía):

   ```bash
   cd backend
   go run .
   # API en http://127.0.0.1:8080
   ```

   Variables opcionales: `PORT` (8080), `MONGO_URI` (`mongodb://127.0.0.1:27017`),
   `MONGO_DB` (`muvautomation`), `BROKER_URL`
   (`amqp://guest:guest@127.0.0.1:5672/`), `STATUS_WORKERS` (4) y
   `STATUS_INTERVAL` (`60s`, `0` desactiva el periódico).

4. **Frontend Angular** (proxy de `/api` hacia el backend):

   ```bash
   cd frontend
   npm install
   npm start          # http://localhost:4200
   ```

   La configuración del proxy está en `frontend/proxy.conf.json`.

## API

| Método | Ruta | Descripción |
|---|---|---|
| GET | `/api/healthz` | Estado del servicio y de MongoDB. |
| GET | `/api/status` | Entorno, propietario y aviso (colección `settings`). |
| GET | `/api/kpis` | Indicadores calculados desde MongoDB. |
| GET | `/api/inventory` | Dispositivos (colección `devices`). |
| GET | `/api/inventory.txt` | Mismo inventario en texto plano. |
| GET | `/api/commands` | Catálogo de comandos permitidos (colección `commands`). |
| GET | `/api/sessions?limit=N` | Últimas órdenes de la bitácora (colección `sessions`). |
| POST | `/api/sessions` | Registra una orden: `{ "dispositivo", "comando", "operador" }`. |
| POST | `/api/status/checks` | Inicia una revisión: encola un job por dispositivo. Devuelve `{ runId, total }`. |
| GET | `/api/status/stream?runId=` | Stream SSE con el progreso del run (`run.started`, `check.result`, `run.finished`). |
| GET | `/api/status/runs?limit=N` | Historial de revisiones. |
| GET | `/api/status/runs/{runId}` | Estado del run y sus checks (fallback sin SSE). |
| GET | `/api/status/latest` | Último estado conocido por dispositivo. |

Ejemplo:

```bash
curl -s http://127.0.0.1:8080/api/kpis
curl -s -X POST http://127.0.0.1:8080/api/sessions \
  -H 'Content-Type: application/json' \
  -d '{"dispositivo":"RTR-LAB-01","comando":"show version","operador":"blue-tec"}'
```

Revisión de estados y stream SSE:

```bash
RUN=$(curl -s -X POST http://127.0.0.1:8080/api/status/checks | python3 -c 'import json,sys;print(json.load(sys.stdin)["runId"])')
curl -N "http://127.0.0.1:8080/api/status/stream?runId=$RUN"
```

## Despliegue

El workflow `.github/workflows/deploy.yml` corre en cada push a `main`:

1. Compila Angular (`ng build --configuration production`) y el binario Go (`linux/amd64`).
2. Copia los artefactos a la VM.
3. Ejecuta `deploy/install.sh`, que instala/actualiza nginx, MongoDB, el binario,
   el vhost y el servicio systemd, y recarga nginx.

Comandos útiles en la VM:

```bash
sudo nginx -t                              # validar configuracion
sudo systemctl reload nginx                # recargar nginx
sudo systemctl status muvautomation        # estado del backend
sudo journalctl -u muvautomation -f        # logs del backend
curl -i http://127.0.0.1/api/healthz       # probar localmente
```

Para desplegar manualmente los artefactos a la VM:

```bash
# desde el directorio staging con frontend/, muvbackend, *.service y *.conf
sudo ./install.sh /ruta/al/staging
```

## Antes vs Ahora

| Aspecto | Antes (HTML estático) | Ahora (Angular + Go + MongoDB) |
|---|---|---|
| Frontend | `app/index.html` con contenido fijo | SPA Angular que consume la API |
| Datos | Hardcodeados en el HTML | Persistidos en MongoDB (`devices`, `commands`, `sessions`) |
| Backend | No existía (solo nginx) | API Go en `127.0.0.1:8080` |
| Bitácora | No existía | Colección `sessions` con registro de órdenes |
| Despliegue | `scp` de `app/*` | CI compila Angular + Go y ejecuta `install.sh` |

## Dónde está publicado

```
http://68.211.136.226
```

## Créditos

MuvAutomation Secure Product Challenge · Laboratorio 3 · Uso académico autorizado.
Datos ficticios (rangos IP de documentación, RFC 5737). Propietario: Blue Team.
Por: Tomas Quinceo y Hernán Sánchez.
