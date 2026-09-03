# LAB3-FDSI-APP-WEB

Aplicación web mínima para el **Laboratorio 3** (Secure Product Challenge · FDSI).
Tema: **Portal de Ejecución Remota Segura** para routers y switches.

El objetivo del laboratorio es publicar un sitio **por HTTP y sin autenticación** en una
instancia autorizada, para que el Red Team haga reconocimiento y el Blue Team observe
tráfico y logs. El producto es *deliberadamente incompleto*: HTTPS, identidad y roles
llegan en el Laboratorio 4.

## Contenido

```
app/
  index.html            Portal estático (datos ficticios)
  public-inventory.txt   Inventario público de demostración
nginx/
  muvautomation.conf     Virtual host + hardening inicial (Fase E)
```

Todo el contenido es ficticio: sin credenciales, sin correos reales, sin direcciones
internas. Las IP usan rangos de documentación (RFC 5737, `192.0.2.0/24`).

## Variables

```bash
export TARGET_IP=IP_ASIGNADA
export TARGET_URL=http://$TARGET_IP
export LAB_CIDR=CIDR_AUTORIZADO
date -u +%Y-%m-%dT%H:%M:%SZ
```

## Despliegue en la instancia autorizada

```bash
sudo apt update && sudo apt install -y nginx

sudo mkdir -p /var/www/muvautomation
sudo cp app/index.html app/public-inventory.txt /var/www/muvautomation/

sudo cp nginx/muvautomation.conf /etc/nginx/sites-available/muvautomation
sudo ln -s /etc/nginx/sites-available/muvautomation /etc/nginx/sites-enabled/muvautomation
sudo rm -f /etc/nginx/sites-enabled/default

sudo nginx -t
sudo systemctl reload nginx
curl -i http://127.0.0.1/
```

Firewall limitado al segmento del laboratorio:

```bash
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow from "$LAB_CIDR" to any port 80 proto tcp
sudo ufw allow OpenSSH
sudo ufw enable
```

## Hardening aplicado (antes / después)

| Control | Antes | Después |
|---|---|---|
| Banner de versión | `Server: nginx/1.x.x` | `Server: nginx` (`server_tokens off`) |
| Listado de directorios | posible | `autoindex off` |
| Sniffing de MIME | permitido | `X-Content-Type-Options: nosniff` |
| Framing | permitido | `X-Frame-Options: DENY` |
| Fuga de Referer | por defecto | `Referrer-Policy: no-referrer` |
| Rutas ocultas (`/.git`, `/.env`) | servidas si existen | `deny all` → 404 |

Verificación:

```bash
curl -I "$TARGET_URL/"
curl -i "$TARGET_URL/.git/config"      # debe responder 404
```

## Límite pedagógico

HTTP sigue exponiendo contenido y metadatos en tránsito. Ese riesgo queda **abierto y
registrado** para corregirse con certificados, HTTPS, identidad y roles en el
Laboratorio 4.
