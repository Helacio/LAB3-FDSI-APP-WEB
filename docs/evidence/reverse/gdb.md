# Confirmación dinámica con GDB

**Binario:** `crackme_level2`
**Función:** `validate_key` en `0x401156`

## Comandos

```bash
gdb ./crackme_level2
```

```gdb
set disassembly-flavor intel
break validate_key
run FDSI-REVERSE-2026
info registers
continue
quit
```

## Evidencia

GDB detuvo la ejecución en la función de validación:

```text
Breakpoint 1, validate_key (candidate="FDSI-REVERSE-2026")
```

La comparación de longitud aparece en `0x401176` y la rutina XOR se ejecuta entre `0x401194` y `0x4011e5`. Al continuar:

```text
License accepted.
FLAG{ghidra_plus_gdb}
[Inferior 1 (process ...) exited normally]
```

GDB confirmó que la clave reconstruida por el análisis estático tiene la longitud correcta, atraviesa `validate_key` y produce el retorno de éxito.
