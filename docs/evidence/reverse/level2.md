# Nivel 2: reconstrucción de la validación

**Binario:** `crackme_level2`

## Análisis estático

La función `validate_key` indica una longitud esperada de `0x11` (17 bytes), usa un arreglo cíclico de cuatro bytes en `0x40208b` y compara el resultado contra `expected` en `0x402090`.

```c
int validate_key(const char *candidate) {
    const unsigned char k[4] = {0x23, 0x51, 0x17, 0x6a};
    const unsigned char expected[17] = {
        0x65, 0x15, 0x44, 0x23, 0x0e, 0x03, 0x52, 0x3c,
        0x66, 0x03, 0x44, 0x2f, 0x0e, 0x63, 0x27, 0x58,
        0x15
    };
    unsigned int difference = 0;

    if (strlen(candidate) != 17) return 0;
    for (unsigned int i = 0; i < 17; i++) {
        unsigned char transformed = candidate[i] ^ k[i & 3];
        difference |= expected[i] ^ transformed;
    }
    return difference == 0;
}
```

Como XOR es reversible, `candidate[i] = expected[i] ^ k[i & 3]`. La clave reconstruida es **`FDSI-REVERSE-2026`**, de 17 caracteres.

## Confirmación

```bash
./crackme_level2 FDSI-REVERSE-2026
```

```text
=== FDSI CrackMe Level 2 ===
Hint: static + dynamic analysis.
License accepted.
FLAG{ghidra_plus_gdb}
```

La clave no aparece como texto plano en `strings`; fue necesario recuperar los arreglos desde `.rodata` y revertir el XOR. La confirmación dinámica está en [`gdb.md`](./gdb.md).
