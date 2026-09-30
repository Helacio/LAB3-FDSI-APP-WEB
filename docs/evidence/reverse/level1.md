# Nivel 1: Strings are evidence

**Binario:** `crackme_level1`

## Ejecución inicial

```bash
./crackme_level1
./crackme_level1 prueba
```

Resultados:

```text
=== FDSI CrackMe Level 1 ===
Uso: ./crackme_level1 <password>
```

```text
=== FDSI CrackMe Level 1 ===
Access denied.
```

El programa exige un argumento y tiene caminos de éxito y error.

## Hipótesis y evidencia

```bash
strings -n 5 crackme_level1 | less
```

Fragmento relevante:

```text
putchar
strcmp
REDTEAM-101
Uso: %s <password>
Access granted.
Access denied.
putchar@GLIBC_2.2.5
strcmp@GLIBC_2.2.5
```

`putchar` es una función de la biblioteca estándar utilizada para imprimir, no una contraseña. El fragmento `;*3$"` procede de metadatos de depuración y tampoco es un dato de validación. `strcmp` sí es relevante porque compara dos cadenas; `REDTEAM-101` es la candidata porque aparece como dato del programa junto a los mensajes de autenticación.

## Confirmación con ensamblador

```bash
objdump -d -M intel crackme_level1 | sed -n '/<main>:/,/401279/p'
objdump -s -j .rodata crackme_level1
```

La función carga la dirección `0x402004` y la pasa a `strcmp` como segundo argumento:

```asm
4011e7:  lea    rax,[rip+0xe16]        # 402004
4011ee:  mov    QWORD PTR [rbp-0x8],rax
401237:  mov    rdx,QWORD PTR [rbp-0x8]
40123b:  mov    rsi,rdx
40123e:  mov    rdi,rax
401241:  call   401060 <strcmp@plt>
```

En `0x402004` aparecen los bytes `52 45 44 54 45 41 4d 2d 31 30 31 00`, que corresponden a `REDTEAM-101` terminado en byte nulo. La hipótesis queda probada con evidencia estática.

## Resultado

```bash
./crackme_level1 REDTEAM-101
```

```text
=== FDSI CrackMe Level 1 ===
Access granted.
FLAG{strings_are_evidence}
```

**FLAG:** `FLAG{strings_are_evidence}`

## Análisis

`strings` puede revelar secretos porque una contraseña compilada como literal queda almacenada en claro dentro del binario. La herramienta solo genera una hipótesis: la confirmación requiere relacionar la cadena con la referencia que usa el código. En software real, los secretos no deben almacenarse en el cliente; la verificación debe realizarse en el servidor o con un diseño de credenciales adecuado.
