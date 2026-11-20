# GoDDNS

GoDDNS es una herramienta multiplataforma escrita en Go para detectar cambios en la IPv4 pública de un equipo y actualizar automáticamente un registro DNS en Simply.com.

El objetivo final es poder ejecutar GoDDNS como una aplicación puntual o como un proceso residente en FreeBSD, manteniendo sincronizado un registro DNS sin intervención manual.

## Estado del proyecto

El proyecto se encuentra en desarrollo del MVP `v0.1.0`. Ya están implementados y probados los componentes de configuración, descubrimiento de IP y comunicación con Simply.com, pero todavía no existe el flujo completo de ejecución desde el binario principal.

### Implementado

- Configuración JSON con `time.Duration`, por ejemplo `"10m"`.
- Carga y validación de configuración.
- Rechazo de campos desconocidos y JSON adicional.
- Validación de proveedor, modo DNS, credenciales, fuentes y consenso.
- Descubrimiento concurrente de IPv4 mediante fuentes HTTP.
- Fuentes configurables como ipify, icanhazip e ifconfig.me.
- Timeout HTTP, cancelación por contexto y límite de tamaño de respuestas.
- Validación de IP y consenso mínimo entre fuentes.
- Interfaz `DNSProvider` independiente del monitor.
- Registro genérico de proveedores.
- Integración Simply mediante DynDNS2.
- Integración Simply mediante API REST de DNS.
- Búsqueda automática del registro `A` en modo REST cuando `record_id` es `0`.
- Tests con `httptest` para fuentes HTTP y Simply.
- Verificación DNS mediante `net.Resolver` en `internal/dnscheck`.
- Compilación sin CGO y sin dependencias externas.

### Pendiente

- Implementar la persistencia de estado con escritura atómica.
- Implementar el monitor y el ciclo periódico con `time.Ticker`.
- Integrar discovery, proveedor, verificación DNS y almacenamiento.
- Integrar la configuración con el registro de proveedores.
- Implementar la CLI: `run`, `check`, `status` y `version`.
- Añadir logging estructurado con `log/slog`.
- Añadir instalación como servicio `rc.d` en FreeBSD.
- Completar README operativo, instalación y ejemplos reales.
- Añadir tests del almacenamiento, monitor y CLI.
- Realizar una prueba de integración real con una zona DNS controlada.
- Revisar permisos de configuración y estado.
- Preparar compilaciones y documentación para Linux, Windows y macOS.

## Requisitos

- Go 1.27.3 o compatible.
- Acceso a Internet.
- Una cuenta de Simply.com.
- Una API key de Simply.com.
- Un dominio y un registro `A` que se puedan modificar.

El proyecto utiliza únicamente la biblioteca estándar de Go.

## Estructura

```text
cmd/goddns/                 Punto de entrada del binario
configs/config.example.json Ejemplo de configuración
internal/config/            Carga y validación JSON
internal/discovery/         Fuentes HTTP y consenso de IP
internal/provider/          Interfaz y registro de proveedores
internal/provider/simply/   DynDNS2 y API REST de Simply
internal/dnscheck/          Verificación de resolución DNS
internal/storage/           Modelo de estado, pendiente de implementación
internal/monitor/           Monitor, pendiente de implementación
internal/logger/            Logger, pendiente de implementación
```

## Configuración

Copia el ejemplo y modifica las credenciales:

```bash
cp configs/config.example.json configs/config.json
chmod 600 configs/config.json
```

Ejemplo:

```json
{
  "general": {
    "interval": "10m",
    "log_level": "info"
  },
  "dns": {
    "provider": "simply",
    "mode": "dyndns2",
    "domain": "example.com",
    "host": "home",
    "product": "example.com",
    "record_id": 0,
    "ttl": 3600,
    "credentials": {
      "account": "YOUR_ACCOUNT",
      "api_key": "YOUR_API_KEY"
    }
  },
  "discovery": {
    "ipv4": true,
    "ipv6": false,
    "sources": [
      "https://api.ipify.org",
      "https://icanhazip.com",
      "https://ifconfig.me/ip"
    ],
    "min_consensus": 2
  },
  "state": {
    "path": "/var/db/goddns/state.json"
  }
}
```

### Modos de Simply

#### DynDNS2

```json
"mode": "dyndns2"
```

Utiliza `https://ddns.simply.com/nic/update` con autenticación Basic. Las respuestas `good` y `nochg` se consideran exitosas.

#### API REST

```json
"mode": "rest"
```

Utiliza `https://api.simply.com/2/` con autenticación Bearer mediante la API key. `product` identifica la zona/producto. Si `record_id` es `0`, GoDDNS consulta los registros y busca el registro `A` correspondiente al hostname.

La API REST realiza una actualización completa del registro, por lo que el registro debe existir y debe ser de tipo `A`.

## Uso actual

El cargador puede probarse desde Go, pero la CLI todavía no está conectada al flujo principal. El binario actual contiene únicamente un `main()` mínimo.

Para ejecutar los tests:

```bash
CGO_ENABLED=0 go test -count=1 ./...
CGO_ENABLED=0 go vet ./...
```

Para consultar cobertura:

```bash
CGO_ENABLED=0 go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

Para probar manualmente el descubrimiento se puede crear temporalmente un pequeño programa que utilice `discovery.NewSources` y `discovery.Discover`. Esta prueba depende de Internet y no sustituye los tests con `httptest`.

Los comandos previstos, todavía pendientes de implementar, son:

```bash
goddns run --config /etc/goddns/config.json
goddns check --config ./config.json
goddns status --config ./config.json
goddns version
```

## Seguridad

- No subas `configs/config.json` al repositorio.
- No publiques la API key.
- Usa permisos `0600` para la configuración.
- No incluyas credenciales en logs ni errores.
- No actualices DNS cuando no se alcance consenso.
- No guardes una IP nueva si la actualización DNS falla.
- Recuerda que la verificación DNS puede verse afectada por cachés y propagación.

## Simply.com

La documentación utilizada es:

- https://www.simply.com/es/docs/api/
- https://api.simply.com/2/doc.json

La documentación distingue entre el servicio DynDNS2 y la API REST directa de DNS. GoDDNS mantiene ambos modos para evitar depender de un único mecanismo.

## Licencia

Consulta el archivo `LICENSE`.
