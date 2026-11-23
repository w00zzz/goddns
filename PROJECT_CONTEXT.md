# GoDDNS — Contexto para continuar el desarrollo

Este archivo resume el estado técnico actual del proyecto para poder continuar en otra iteración o sesión sin perder decisiones ni trabajo realizado.

## Objetivo del proyecto

GoDDNS detectará cambios en la IPv4 pública de un equipo y actualizará un registro DNS de Simply.com. Debe funcionar como CLI y como proceso residente, inicialmente con soporte principal para FreeBSD y sin dependencias externas.

Versión objetivo del MVP: `v0.1.0`.

## Decisiones tomadas

- Lenguaje: Go.
- Dependencias: únicamente biblioteca estándar.
- Configuración: JSON.
- IPv6: fuera del alcance funcional inicial, aunque existen campos de configuración para dejar preparada la evolución.
- Proveedor inicial: Simply.com.
- El monitor debe depender de `provider.DNSProvider`, nunca de detalles de Simply.
- Los intervalos se escriben como texto (`"10m"`) y se convierten a `time.Duration` mediante `GeneralConfig.UnmarshalJSON`.
- El campo DNS elegido es `host`, no `hosts`.
- Se usa consenso entre fuentes antes de permitir cualquier actualización DNS.
- Cada cambio de archivo se ha confirmado en un commit independiente cuando se ha trabajado en etapas recientes.

## Estado implementado

### Configuración — `internal/config`

Implementado:

- `Config`, `GeneralConfig`, `DNSConfig`, `DiscoveryConfig` y `StateConfig`.
- `GeneralConfig.Interval time.Duration`.
- Parseo de duraciones como `"10m"`.
- `Load(path)` mediante `os.ReadFile` y `encoding/json`.
- Rechazo de campos desconocidos.
- Rechazo de múltiples valores JSON o JSON adicional.
- `Config.Validate()`.
- Validación de intervalo, nivel de log, proveedor, modo, dominio, host, credenciales, fuentes, consenso y ruta de estado.
- Modos admitidos: `dyndns2` y `rest`.
- En modo REST se exige `dns.product`.

Tests:

- `internal/config/loader_test.go`
- `internal/config/validation_test.go`

### Descubrimiento — `internal/discovery`

Implementado:

- Interfaz `IPSource`.
- `HTTPSource`.
- `NewHTTPSource` y `NewSources`.
- Timeout HTTP por defecto de 10 segundos.
- Uso de `context.Context`.
- Respuestas limitadas a 4 KB.
- Validación de status HTTP y de IPv4.
- `Discover(ctx, sources, minimumConsensus)`.
- Consultas concurrentes.
- Ignora fuentes fallidas y solo devuelve una IP con consenso suficiente.

Tests:

- `sources_test.go` con `httptest`.
- `discovery_test.go` con fuentes falsas.

### Proveedores — `internal/provider`

Implementado:

- `DNSProvider`.
- `UpdateRequest` con dominio, host e IP.
- `Registry` con registro y búsqueda por nombre.
- Tests del registro.

### Simply — `internal/provider/simply`

Implementado:

- Cliente HTTP reutilizable.
- Modo DynDNS2:
  - Endpoint `https://ddns.simply.com/nic/update`.
  - Basic Authentication.
  - Parámetros `hostname`, `domain` y `myip`.
  - Respuestas exitosas `good` y `nochg`.
- Modo REST:
  - API base `https://api.simply.com/2`.
  - Bearer API key.
  - Consulta de registros cuando `record_id == 0`.
  - Búsqueda del registro `A` por hostname.
  - Actualización mediante `PUT`.
  - Payload con tipo, nombre, IP y TTL.
- Timeout, context cancellation y límite de respuestas.
- Tests con `httptest` para ambos modos.

La construcción configurada se realiza mediante `simply.NewConfigured(simply.Config{...})`. Todavía no está conectada al loader ni al monitor.

### Verificación DNS — `internal/dnscheck`

Implementado:

- `Verify` usando `net.Resolver`.
- `WaitForIP` para reintentar durante la propagación DNS.
- Comparación de la IP resuelta con la IP esperada.
- Resolver inyectable para tests.

Limitación: utiliza el resolver configurado en el sistema. No consulta directamente los servidores autoritativos y puede observar caché o propagación incompleta.

## Estado pendiente

### Persistencia — `internal/storage`

Implementado:

- `Load(path)` con `ErrNotFound` para archivos inexistentes.
- `Save(path, state)` con creación de directorio.
- Escritura atómica mediante archivo temporal, `Sync`, `Rename` y sincronización del directorio.
- Permisos restrictivos `0600` para el archivo y `0750` para el directorio.
- Rechazo de JSON inválido, campos desconocidos, JSON adicional y estados inválidos.
- Validación de IPv4, IPv6 y timestamp RFC3339.
- `State.UpdateIPv4` con timestamp UTC.
- Tests de lectura, escritura, reemplazo, corrupción, permisos y preservación del estado anterior.

El estado mínimo esperado es:

```json
{
  "last_ipv4": "",
  "last_ipv6": "",
  "last_update": ""
}
```

### Integración del proveedor — completada parcialmente

Implementado en `cmd/goddns/main.go`:

- Carga del proveedor Simply a partir de `config.Config`.
- Registro y selección mediante `provider.Registry`.
- Transferencia de modo, API key, cuenta, producto, record ID y TTL.
- El modo REST descubre el registro cuando `record_id` es `0`.

Pendiente:

- Decidir si el `record_id` descubierto debe persistirse.
- Añadir una opción de configuración para activar la espera de verificación DNS.

### Monitor — `internal/monitor`

Implementado:

- Comprobación inicial inmediata.
- Carga del estado persistido.
- Descubrimiento de IP mediante una función inyectable.
- Comparación de IP anterior y actual.
- Actualización DNS solo cuando la IP cambia o no existe estado.
- Persistencia solo después de actualizar DNS correctamente.
- Conservación del estado anterior ante errores.
- Verificación DNS opcional mediante una función inyectable.
- Ciclo periódico con `time.Ticker`.
- Cancelación mediante `context.Context`.
- Errores de comprobaciones periódicas no detienen el monitor.

Tests incluidos para primera ejecución, IP sin cambios, cambios, errores de discovery, errores del proveedor y errores de verificación.

Pendiente:

- Integrar logs detallados de cada comprobación y error.
- Añadir una opción para activar la espera de verificación DNS.

El monitor no importa el paquete `simply` directamente.

### CLI — `cmd/goddns`

Implementado con `flag`:

- `goddns run --config PATH`.
- `goddns check --config PATH`.
- `goddns status --config PATH`.
- `goddns version`.

`run` ejecuta el monitor en primer plano y captura SIGINT/SIGTERM. `check` solo descubre y muestra la IP; no actualiza DNS. `status` devuelve el estado JSON persistido.

Pendiente:

- Mejorar mensajes de error y salida operativa.
- Añadir pruebas de integración de comandos reales.

No introducir Cobra ni otras dependencias.

### Logging — `internal/logger`

Implementado con `log/slog` y niveles `debug`, `info`, `warn` y `error`.

Pendiente:

- Integrar logs detallados de cada comprobación y error del monitor.
- Revisar formato final para FreeBSD.

### Prioridad 3 — Servicio y documentación operativa

Falta crear:

- `packaging/freebsd/goddns` como script `rc.d`.
- `packaging/linux/goddns.service`.
- Instalación en `/usr/local/bin/goddns`.
- Configuración en `/usr/local/etc/goddns/config.json`.
- Estado en `/var/db/goddns/`.
- Revisar README con instalación real y rutas finales de FreeBSD.

## Comandos de verificación

Ejecutar siempre antes de cerrar una etapa:

```bash
CGO_ENABLED=0 go test -count=1 ./...
CGO_ENABLED=0 go vet ./...
git diff --check
git status --short --branch
```

Cobertura:

```bash
CGO_ENABLED=0 go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

## Configuración actual de ejemplo

`configs/config.example.json` contiene:

- Intervalo de `10m`.
- Proveedor `simply`.
- Modo `dyndns2`.
- Dominio `lumiar.info`.
- Host `home`.
- Producto configurado para permitir cambiar a REST.
- Tres fuentes IPv4.
- Consenso mínimo de 2.
- Estado en `/var/db/goddns/state.json`.

Nunca usar las credenciales de ejemplo en producción.

## Archivos y artefactos pendientes

- `coverage.html` es un informe generado localmente y está ignorado por `.gitignore`.
- `README.md` contiene documentación general; deberá revisarse con las rutas finales del servicio.
- `LICENSE` todavía no tiene contenido revisado.

## Última validación conocida

La suite completa pasa con `CGO_ENABLED=0` y `go vet ./...` no informa errores después de la implementación de DynDNS2, REST y `dnscheck`.

## Siguiente acción recomendada

Implementar los scripts de servicio FreeBSD/Linux y realizar una prueba de integración real con Simply y una zona DNS controlada.
