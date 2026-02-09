# Tenant Provisioner (Go)

Microservicio para provisionar bases de datos PostgreSQL por tenant usando Docker.

## Arquitectura

- `main.go`: bootstrap de app.
- `internal/config`: carga y validación de configuración.
- `internal/docker`: adapter para ejecutar comandos Docker.
- `internal/provisioner`: lógica de dominio de provision/deprovision.
- `internal/httpapi`: handlers y rutas HTTP.

## Endpoints

- `GET /healthz`
- `POST /api/v1/provision/tenants`
- `DELETE /api/v1/provision/resources/:resource_id`
- `POST /api/v1/provision/deprovision`

### Provision

Request:

```json
{
  "tenant_name": "acme",
  "tenant_id": "optional",
  "limits": {
    "memory_mb": 256,
    "cpu_cores": 0.5
  }
}
```

Response (`201`):

```json
{
  "status": "provisioned",
  "resource_id": "<docker_container_id>",
  "connection_string": "postgres://tenant_user:***@127.0.0.1:<port>/tenant_acme",
  "db_secret_path": "tenants/acme/db"
}
```

### Deprovision

`DELETE /api/v1/provision/resources/:resource_id` o
`POST /api/v1/provision/deprovision` con body:

```json
{
  "resource_id": "<docker_container_id>"
}
```

## Variables de entorno

Ver `.env.example`.

## Ejecutar

```bash
go run .
```

## Seguridad Docker

- Guía recomendada: `docs/docker-access-hardening.md`
