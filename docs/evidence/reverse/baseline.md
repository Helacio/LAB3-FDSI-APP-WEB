# Puntos 0 y 1: preparación y baseline forense

**Fecha:** 2026-09-30
**Integrantes:** Tomas Quiceno y Hernan Sanchez
**Binarios:** `crackme_level1`, `crackme_level2`, `crackme_level2_stripped`
**Salida cruda:** [`baseline.txt`](./baseline.txt)

## 0. Preparación

| Herramienta | Estado | Uso |
|---|---|---|
| `file` | Instalada, 5.47-4 | Identificar formato y arquitectura |
| `binutils` | Actualizada a 2.47-6 | `strings`, `objdump`, `readelf` y `nm` |
| `gdb` | Instalada, 17.2-1 | Confirmar dinámicamente la validación |
| Ghidra | Instalada | Análisis estático y decompilación |

```bash
sudo apt update
sudo apt install -y binutils gdb file
mkdir -p ~/fdsi-reverse/evidence
cd ~/fdsi-reverse
```

`file` y `gdb` ya estaban instalados. `apt install` actualizó `binutils` de `2.47-2` a `2.47-6`.

### Checklist

- [x] Copiamos los tres binarios al directorio de trabajo.
- [x] Confirmamos el formato y la arquitectura con `file`.
- [x] Calculamos y registramos los SHA-256.
- [x] Creamos `evidence/` para guardar las evidencias.
- [x] Instalamos Ghidra para el análisis del Nivel 2.
- [x] Trabajamos únicamente con los binarios autorizados del laboratorio.

## 1. Baseline forense

Los tres archivos son ejecutables **ELF de 64 bits**, arquitectura **x86-64**, enlazados dinámicamente y con intérprete `/lib64/ld-linux-x86-64.so.2`.

| Archivo | Resultado relevante |
|---|---|
| `crackme_level1` | `with debug_info, not stripped` |
| `crackme_level2` | `with debug_info, not stripped` |
| `crackme_level2_stripped` | `stripped` |

Los dos primeros conservan símbolos e información de depuración. El tercero no conserva los símbolos de depuración, por lo que sus funciones deben localizarse por referencias, comportamiento y flujo de control.

### Cabecera ELF

| Campo | Valor |
|---|---|
| Clase | ELF64 |
| Datos | Little endian |
| Tipo | `EXEC` |
| Máquina | AMD x86-64 |
| Entry point | `0x401070` |
| Program headers | 14 |
| Section headers | 36 |

### Integridad SHA-256

Los hashes calculados coinciden exactamente con los publicados en la guía.

| Archivo | SHA-256 | Estado |
|---|---|---|
| `crackme_level1` | `61e980febe84b1003b5a3b641468e915b984f7fdd835be9828af54233f88c68c` | Coincide |
| `crackme_level2` | `8dc5931dfbf74d7371de9ca9ed8cc57bfe0af4521346202dcd1c701dd8b6f4e5` | Coincide |
| `crackme_level2_stripped` | `c8e638741272a87ee3b30fe8878898c1aa977e6a879a1ec0b271034b5bb9aed3` | Coincide |

## Análisis

`file`, `readelf` y `sha256sum` permiten conocer la arquitectura, el formato ELF, el enlazado dinámico, el punto de entrada, la presencia o ausencia de símbolos y la integridad del archivo sin ejecutarlo.

SHA-256 genera una huella de 256 bits que cambia si se modifica el archivo. Comparar el hash calculado con el publicado demuestra que el archivo no fue alterado, corresponde al material del docente y permite reproducir los resultados entre equipos.
