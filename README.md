# LAB3-FDSI-APP-WEB

Aplicación web del **Laboratorio 3** (Secure Product Challenge · FDSI).
Tema: **Portal de Ejecución Remota Segura** para routers y switches.

Stack: **Angular** (frontend) + **Go** (API) + **MongoDB** (NoSQL) sobre **nginx** en HTTP.

## Funcionamiento

El portal permite consultar el inventario y el catálogo de comandos, y registrar
órdenes en una **bitácora persistida en MongoDB**. Se publica por HTTP (puerto 80),
sin autenticación, como línea base del Laboratorio 3 (Red Team + Blue Team).
El producto es *deliberadamente incompleto*: HTTPS, identidad y roles llegan en el
Laboratorio 4.

## Arquitectura

```
Navegador ──HTTP:80──> nginx ──/ ────────────> /var/www/muvautomation (Angular)
                            └──/api/────────> 127.0.0.1:8080 (Go) ──> MongoDB 127.0.0.1:27017
```

## Estructura del repositorio

| Ruta | Descripción |
|---|---|
| `frontend/` | Aplicación Angular (standalone, zoneless). |
| `backend/` | API REST en Go (solo `net/http` + driver oficial de MongoDB). |
| `nginx/muvautomation.conf` | vhost de nginx: SPA + proxy `/api`. |
| `deploy/muvautomation.service` | Unit systemd del backend. |
| `deploy/install.sh` | Aprovisiona nginx, MongoDB, el binario y el servicio en la VM. |
| `.github/workflows/deploy.yml` | CI/CD: build de Angular y Go, y despliegue a la VM. |
| `evidence/` | Reglas de detección del Blue Team. |

## Requisitos

- Node.js 24+ y npm
- Go 1.27+
- MongoDB 8.0

## Arranque local

1. **MongoDB** (con una carpeta de datos local):

   ```bash
   mkdir -p /tmp/muv-mongo
   mongod --dbpath /tmp/muv-mongo --bind_ip 127.0.0.1 --port 27017
   ```

2. **Backend Go** (siembra la base de datos al primer arranque si está vacía):

   ```bash
   cd backend
   go run .
   # API en http://127.0.0.1:8080
   ```

   Variables opcionales: `PORT` (8080), `MONGO_URI` (`mongodb://127.0.0.1:27017`),
   `MONGO_DB` (`muvautomation`).

3. **Frontend Angular** (proxy de `/api` hacia el backend):

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

Ejemplo:

```bash
curl -s http://127.0.0.1:8080/api/kpis
curl -s -X POST http://127.0.0.1:8080/api/sessions \
  -H 'Content-Type: application/json' \
  -d '{"dispositivo":"RTR-LAB-01","comando":"show version","operador":"blue-tec"}'
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
