# GoDDNS — Pruebas pendientes y validación de producción

Este documento describe cómo validar GoDDNS antes de considerarlo listo para uso real.

## 1. Validación local previa

Desde la raíz del repositorio:

```bash
gofmt -w cmd internal
CGO_ENABLED=0 go test -count=1 ./...
CGO_ENABLED=0 go vet ./...
CGO_ENABLED=1 go test -race ./...
go build -trimpath -ldflags='-s -w' -o /tmp/goddns ./cmd/goddns
/tmp/goddns version
```

Todos los comandos deben finalizar correctamente.

Validar el script FreeBSD:

```bash
sh -n packaging/freebsd/goddns
```

El informe de cobertura es opcional:

```bash
CGO_ENABLED=0 go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

## 2. Preparar una zona DNS de pruebas

Antes de probar actualizaciones reales:

1. Usar un dominio controlado.
2. Crear un registro `A` dedicado, por ejemplo `home.example.com`.
3. No usar inicialmente un registro crítico.
4. Anotar la IP actual del registro.
5. Confirmar que el registro existe antes de usar el modo REST.
6. Crear una API key de Simply con los permisos mínimos necesarios.
7. No pegar la API key en comandos, commits, tickets ni logs.

Consultar el estado inicial:

```bash
dig +short home.example.com A
```

En FreeBSD, si no está disponible `dig`, puede utilizarse:

```bash
drill home.example.com A
```

También puede consultarse un resolver concreto:

```bash
dig @1.1.1.1 +short home.example.com A
dig @8.8.8.8 +short home.example.com A
```

## 3. Crear la configuración real

```bash
mkdir -p /tmp/goddns-validation
cp configs/config.example.json /tmp/goddns-validation/config.json
chmod 600 /tmp/goddns-validation/config.json
```

Modificar como mínimo:

- `dns.domain`.
- `dns.host`.
- `dns.mode`.
- `dns.product` si se usa REST.
- `dns.record_id` si se conoce.
- `dns.credentials.account` para DynDNS2.
- `dns.credentials.api_key`.
- `state.path`.

Para una primera prueba usar un intervalo corto, por ejemplo:

```json
"interval": "1m"
```

No subir nunca esta configuración al repositorio.

## 4. Probar únicamente el descubrimiento

Esta prueba no modifica DNS:

```bash
/tmp/goddns check --config /tmp/goddns-validation/config.json
```

El resultado debe ser una IPv4 pública. Compararla con:

```bash
curl -fsS https://api.ipify.org
```

`check` usa consenso entre las fuentes configuradas y no llama al proveedor DNS.

## 5. Probar DynDNS2 con Simply

Configurar:

```json
"mode": "dyndns2"
```

El registro debe existir previamente. Ejecutar el monitor durante una comprobación:

```bash
timeout 90s /tmp/goddns run --config /tmp/goddns-validation/config.json
```

El proceso debe:

1. Descubrir la IP pública.
2. Solicitar la actualización a Simply.
3. Guardar el estado solo después de una respuesta `good` o `nochg`.
4. Detenerse correctamente al recibir `SIGTERM`.

Consultar el estado:

```bash
/tmp/goddns status --config /tmp/goddns-validation/config.json
```

Debe contener la IP actual y un `last_update` válido.

Verificar el DNS:

```bash
dig @1.1.1.1 +short home.example.com A
dig @8.8.8.8 +short home.example.com A
```

La IP debe coincidir con la salida de `goddns check`. Tener en cuenta el TTL y la caché DNS.

## 6. Probar la API REST de Simply

Configurar:

```json
"mode": "rest",
"product": "example.com",
"record_id": 0
```

Con `record_id: 0`, GoDDNS consulta los registros del producto y busca el registro `A` cuyo nombre coincida con el hostname.

Si se conoce el ID del registro, puede configurarse explícitamente:

```json
"record_id": 123
```

Ejecutar la prueba igual que con DynDNS2:

```bash
timeout 90s /tmp/goddns run --config /tmp/goddns-validation/config.json
/tmp/goddns status --config /tmp/goddns-validation/config.json
```

Verificar posteriormente con `dig` o `drill`.

La API REST realiza una actualización completa del registro. Confirmar que el registro sea de tipo `A` y que sus datos actuales no contengan valores que deban conservarse y que GoDDNS no gestione.

## 7. Probar errores sin perder el estado

No hacer esta prueba contra un registro crítico.

1. Ejecutar una actualización válida y guardar el estado.
2. Hacer una copia del archivo de estado.
3. Configurar temporalmente una API key incorrecta.
4. Ejecutar `run`.
5. Confirmar que el proceso no se detiene definitivamente.
6. Confirmar que el estado anterior no se reemplaza.
7. Restaurar la API key válida.
8. Ejecutar otra comprobación.
9. Confirmar que el siguiente intento puede actualizar y guardar el nuevo estado.

El estado nunca debe marcar una IP como sincronizada si Simply devuelve un error.

## 8. Probar el servicio en FreeBSD

Instalar el binario y el script:

```bash
install -d -o root -g wheel -m 0755 /usr/local/etc/goddns
install -d -o goddns -g goddns -m 0750 /var/db/goddns
install -m 0600 config.json /usr/local/etc/goddns/config.json
install -m 0755 goddns /usr/local/bin/goddns
install -m 0555 packaging/freebsd/goddns /usr/local/etc/rc.d/goddns
```

Crear el usuario `goddns` si no existe y comprobar permisos sobre el directorio de estado y el log.

Activar y probar:

```bash
sysrc goddns_enable=YES
service goddns start
service goddns status
service goddns stop
service goddns start
service goddns restart
```

Revisar:

```bash
cat /var/log/goddns.log
cat /var/db/goddns/state.json
```

Comprobar que `stop` termina el proceso y que `restart` no deja procesos duplicados.

## 9. Probar el servicio en Linux

Instalar:

```bash
install -d -o root -g root -m 0755 /etc/goddns
install -d -o goddns -g goddns -m 0750 /var/lib/goddns
install -m 0600 config.json /etc/goddns/config.json
install -m 0755 goddns /usr/local/bin/goddns
install -m 0644 packaging/linux/goddns.service /etc/systemd/system/goddns.service
```

Crear el usuario `goddns` si no existe y comprobar permisos.

Activar y probar:

```bash
systemctl daemon-reload
systemctl enable --now goddns
systemctl status goddns
journalctl -u goddns -f
systemctl restart goddns
systemctl stop goddns
```

Comprobar que systemd reinicia el proceso si se detiene inesperadamente y que no existen procesos duplicados.

## 10. Comprobar una reinicialización

1. Ejecutar una actualización correcta.
2. Confirmar que existe el archivo de estado.
3. Detener el proceso.
4. Iniciar GoDDNS de nuevo.
5. Confirmar que carga la IP anterior.
6. Confirmar que una IP sin cambios no provoca una actualización DNS innecesaria.

## 11. Comprobar cancelación y apagado

Durante `run`, enviar:

```bash
kill -TERM $(pgrep -f '/usr/local/bin/goddns run')
```

El proceso debe terminar sin dejar archivos temporales ni procesos huérfanos.

## 12. Criterios para considerar listo el MVP

- [ ] `go test ./...` pasa.
- [ ] `go vet ./...` pasa.
- [ ] `go test -race ./...` pasa con `CGO_ENABLED=1`.
- [ ] El binario compila con `CGO_ENABLED=0`.
- [ ] `goddns check` obtiene una IP pública por consenso.
- [ ] DynDNS2 actualiza correctamente un registro controlado.
- [ ] API REST actualiza correctamente un registro controlado.
- [ ] `status` muestra el estado correcto.
- [ ] Un error de Simply conserva el estado anterior.
- [ ] La IP resuelta coincide después de la propagación DNS.
- [ ] El estado sobrevive a un reinicio.
- [ ] El servicio FreeBSD funciona con `rc.d`.
- [ ] El servicio Linux funciona con systemd.
- [ ] Las credenciales no aparecen en logs ni en Git.
- [ ] Los permisos de configuración y estado son restrictivos.
- [ ] No quedan procesos duplicados después de reinicios.

Hasta completar las pruebas reales con Simply y al menos uno de los sistemas objetivo, GoDDNS debe considerarse listo para pruebas controladas, no para producción crítica.
