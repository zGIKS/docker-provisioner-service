# Docker sin root para el provisioner

Guía para ejecutar el provisioner con un usuario dedicado y Docker rootless.

## Objetivo

- El provisioner no corre como `root`.
- El provisioner no usa `/var/run/docker.sock` del daemon root.
- El provisioner usa su propio daemon Docker rootless vía `DOCKER_HOST=unix:///run/user/<uid>/docker.sock`.

## Por qué no usar el grupo docker en producción

Agregar un usuario al grupo `docker` da acceso equivalente a root sobre el host.

- Válido para desarrollo local rápido.
- No recomendado para hardening en producción.

## Opción recomendada: usuario dedicado + Docker rootless

### 1) Crear usuario de servicio

```bash
sudo useradd -m -s /bin/bash provisioner
id provisioner
```

### 2) Instalar prerequisitos (Debian/Ubuntu)

```bash
sudo apt-get update
sudo apt-get install -y uidmap dbus-user-session slirp4netns fuse-overlayfs curl
```

Nota: en otras distros cambia el gestor de paquetes, pero los componentes son equivalentes.

### 3) Instalar Docker rootless para `provisioner`

```bash
sudo -iu provisioner
curl -fsSL https://get.docker.com/rootless | sh
```

### 4) Activar daemon rootless de ese usuario

Dentro de la sesión del usuario `provisioner`:

```bash
systemctl --user daemon-reload
systemctl --user enable docker
systemctl --user start docker
```

Desde root/admin habilita linger para que el daemon de usuario siga activo sin login interactivo:

```bash
exit
sudo loginctl enable-linger provisioner
```

### 5) Obtener UID y validar Docker rootless

```bash
id -u provisioner
sudo -iu provisioner env XDG_RUNTIME_DIR=/run/user/$(id -u provisioner) DOCKER_HOST=unix:///run/user/$(id -u provisioner)/docker.sock docker info
```

Debe responder sin usar `/var/run/docker.sock`.

## Ejecutar el provisioner con systemd (recomendado)

### 1) Crear archivo de entorno

Archivo: `/etc/provisioner/provisioner.env`

```bash
PORT=3000
DOCKER_HOST=unix:///run/user/<UID_PROVISIONER>/docker.sock
XDG_RUNTIME_DIR=/run/user/<UID_PROVISIONER>

TENANT_DB_IMAGE=postgres:16-alpine
TENANT_DB_NETWORK=auth-tenants
TENANT_DB_HOST=127.0.0.1
TENANT_DB_USER=tenant_user
TENANT_DB_NAME_PREFIX=tenant_
DOCKER_COMMAND_TIMEOUT_SECONDS=120
```

### 2) Crear unidad systemd del provisioner

Archivo: `/etc/systemd/system/provisioner.service`

```ini
[Unit]
Description=IAM Tenant Provisioner
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=provisioner
Group=provisioner
WorkingDirectory=/home/giks/Documents/IAM-service/provisioner
EnvironmentFile=/etc/provisioner/provisioner.env
ExecStart=/usr/bin/env go run .
Restart=always
RestartSec=3
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/home/giks/Documents/IAM-service/provisioner /run/user/<UID_PROVISIONER>

[Install]
WantedBy=multi-user.target
```

Si compilas binario, mejor reemplazar `ExecStart` por ruta fija, por ejemplo:

```ini
ExecStart=/opt/provisioner/provisioner
```

### 3) Activar servicio

```bash
sudo mkdir -p /etc/provisioner
sudo systemctl daemon-reload
sudo systemctl enable provisioner
sudo systemctl start provisioner
sudo systemctl status provisioner
```

## Verificación end-to-end

1. Ver logs del servicio:

```bash
journalctl -u provisioner -f
```

2. Healthcheck:

```bash
curl -i http://127.0.0.1:3000/healthz
```

3. Probar provision:

```bash
curl -sS -X POST http://127.0.0.1:3000/api/v1/provision/tenants \
  -H 'Content-Type: application/json' \
  -d '{"tenant_name":"acme"}'
```

## Troubleshooting

### Error: `permission denied ... /var/run/docker.sock`

Causa: el servicio está apuntando al socket root de Docker.

Revisar:

- `DOCKER_HOST=unix:///run/user/<uid>/docker.sock`
- `User=provisioner`
- `XDG_RUNTIME_DIR=/run/user/<uid>`

### Error: `no such file or directory /run/user/<uid>/docker.sock`

Causa: daemon rootless del usuario no está levantado.

```bash
sudo -iu provisioner systemctl --user status docker
sudo loginctl enable-linger provisioner
```

### Error: `Cannot connect to the Docker daemon`

Revisar entorno efectivo del servicio:

```bash
sudo systemctl show provisioner --property=Environment
```

## Opción rápida (menos segura): grupo docker

Solo para desarrollo local:

```bash
sudo usermod -aG docker <usuario>
newgrp docker
docker info
```

## Recomendación final

- Producción: usuario dedicado + rootless + systemd + hardening de unidad.
- Desarrollo: grupo `docker` puede usarse temporalmente.
