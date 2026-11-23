# GoDDNS

GoDDNS es una herramienta multiplataforma escrita en Go para detectar cambios en la IPv4 pública de un equipo y actualizar automáticamente un registro DNS en Simply.com.

El objetivo final es poder ejecutar GoDDNS como una aplicación puntual o como un proceso residente en FreeBSD, manteniendo sincronizado un registro DNS sin intervención manual.

## Estado del proyecto

El proyecto se encuentra en desarrollo del MVP `v0.1.0`. Ya están implementados y probados los componentes de configuración, descubrimiento de IP, almacenamiento, monitor, CLI y comunicación con Simply.com. Todavía faltan el servicio FreeBSD y la validación de una integración real completa.

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
- Persistencia de estado JSON con escritura atómica y sincronización a disco.
- Validación de estado, recuperación tras reinicio y protección de permisos.
- Monitor con comprobación inicial, detección de cambios, reintentos y cancelación.
- CLI con `run`, `check`, `status` y `version`.
- Logger estructurado basado en `log/slog`.
- Compilación sin CGO y sin dependencias externas.

### Pendiente

- Añadir instalación como servicio `rc.d` en FreeBSD.
- Completar una prueba de integración real con una zona DNS controlada.
- Añadir opciones de verificación DNS al flujo de ejecución si se desea esperar propagación.
- Revisar permisos de configuración y estado en la instalación real.
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
internal/storage/           Persistencia atómica del estado
internal/monitor/           Orquestación del ciclo de actualización
internal/logger/            Logger estructurado con `log/slog`
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

La CLI ya está conectada al flujo principal. `check` consulta la IP pública sin modificar DNS; `run` inicia el monitor en primer plano; `status` muestra el estado persistido.

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

Las comprobaciones reales dependen de Internet y de credenciales válidas de Simply.com. Los comandos disponibles son:

```bash
goddns run --config /etc/goddns/config.json
goddns check --config ./config.json
goddns status --config ./config.json
goddns version
```

## Ejecución como servicio

### FreeBSD

El archivo `packaging/freebsd/goddns` es un script `rc.d`. El proceso Go permanece en primer plano y `/usr/sbin/daemon` se encarga de mantenerlo como servicio.

Instalación básica:

```bash
install -d -o root -g wheel -m 0755 /usr/local/etc/goddns
install -d -o goddns -g goddns -m 0750 /var/db/goddns
install -m 0600 configs/config.json /usr/local/etc/goddns/config.json
install -m 0755 goddns /usr/local/bin/goddns
install -m 0555 packaging/freebsd/goddns /usr/local/etc/rc.d/goddns
```

El usuario `goddns` debe existir y tener permisos de escritura sobre `/var/db/goddns` y el archivo de log configurado. Después:

```bash
sysrc goddns_enable=YES
service goddns start
service goddns status
service goddns stop
```

La configuración predeterminada del servicio es:

```text
/usr/local/bin/goddns
/usr/local/etc/goddns/config.json
/var/db/goddns/state.json
/var/log/goddns.log
```

### Linux con systemd

El archivo `packaging/linux/goddns.service` utiliza `/etc/goddns/config.json` y `/var/lib/goddns/state.json`.

```bash
install -d -o root -g root -m 0755 /etc/goddns
install -d -o goddns -g goddns -m 0750 /var/lib/goddns
install -m 0600 configs/config.json /etc/goddns/config.json
install -m 0755 goddns /usr/local/bin/goddns
install -m 0644 packaging/linux/goddns.service /etc/systemd/system/goddns.service
systemctl daemon-reload
systemctl enable --now goddns
systemctl status goddns
```

Los logs se consultan con:

```bash
journalctl -u goddns -f
```

Los usuarios y directorios de servicio deben crearse previamente según la distribución Linux utilizada.

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
