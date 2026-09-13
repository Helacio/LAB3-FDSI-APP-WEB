# LAB3-FDSI-APP-WEB

Aplicación web mínima para el **Laboratorio 3** (Secure Product Challenge · FDSI).
Tema: **Portal de Ejecución Remota Segura** para routers y switches.

## Funcionamiento

Sitio estático servido por **nginx** sobre HTTP (puerto 80), sin autenticación,
para que el Red Team haga reconocimiento y el Blue Team observe tráfico y logs.
El producto es *deliberadamente incompleto*: HTTPS, identidad y roles llegan en
el Laboratorio 4.

## Antes vs Ahora

| Aspecto | Antes (LAN) | Ahora (Azure) |
|---|---|---|
| Infraestructura | Servidor local en red LAN | VM de Azure (`FDSIServidorWeb`) con nginx |
| Despliegue | Manual: `cp` de archivos a `/var/www/muvautomation` y vhost a mano | Automático: GitHub Actions en cada push a `main` |
| Firewall | `ufw` limitado al CIDR del laboratorio | NSG de Azure (puerto 80 abierto) + `ufw` inactivo |
| Acceso | IP privada de la LAN | IP pública de la VM |

El despliegue por GitHub Actions copia `app/*` a `/var/www/muvautomation/`
y recarga nginx (`nginx -t && systemctl reload nginx`), sin intervención manual.

## Cómo ejecutarlo / desplegar

1. Hacer push a la rama `main` del repositorio.
2. El workflow `.github/workflows/deploy.yml` corre solo: copia los archivos por SSH
   y recarga nginx en la VM.
3. Verificar el resultado en la pestaña **Actions** de GitHub.

Comandos útiles dentro de la VM:

```bash
sudo nginx -t                            # validar configuracion
sudo systemctl reload nginx              # recargar nginx
curl -i http://127.0.0.1/                # probar localmente
```

## Dónde está publicado

```
http://68.211.136.226
```
