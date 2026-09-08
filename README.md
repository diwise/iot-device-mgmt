# iot-device-mgmt

Device management service.

# Design

```mermaid
flowchart LR
    api --http--> iot-core
    iot-agent --http--> api --http--> iot-agent
    iot-agent --rabbitMQ-->handler
    core --cloudevent-->external-service
    core --http--> iot-device-mgmt-web
    subgraph iot-device-mgmt
        api
        handlerhttps://cloudevents.io/
        core
        watchdog-->watchdog
    end 
```

## Dependencies 

# Storage
When the service is started data will be loaded from configuration files and stored in a database. If `POSTGRES_HOST` is set, postgreSql will be use. If not, sqlite is used instead.

# Watchdog
Watchdog is a feature that will periodically verify the sensors. Currently only last observed time is checked. If larger than `interval` a warning status will be set. 

# Security

## Authorization
Authorization is handled via OIDC access tokens that are delegated to [Open Policy Agent](https://www.openpolicyagent.org) for validation and decoding. This service does not impose any restrictions on the structure of a token's claims, allowing freedom for policy writers to integrate with existing organisational policies more easily.

By default the policy evaluation result must contain a `tenants` list with the tenants that the client is allowed to access. This list can be fetched from an arbitrary claim in the access token or created in the policy file based on other properties such as groups or subject identity (sub).

The access-object authorization model can be enabled with `AUTHZ_ACCESS_OBJECT_ENABLED=true` or `-authz-access-object=true`. In that mode the policy result must contain an `access` object that maps tenant names to allowed scopes, for example `{"access":{"default":["devices.read","devices.update"]}}`.

A [basic policy file](./assets/config/authz.rego) is included in the built image by default, but is expected to be replaced with an organisational specific policy at the time of deployment.

# Configuration

## Environment variables
```json
"RABBITMQ_HOST": "<rabbit mq hostname>"
"RABBITMQ_PORT": "5672"
"RABBITMQ_VHOST": "/"
"RABBITMQ_USER": "user"
"RABBITMQ_PASS": "bitnami"
"RABBITMQ_DISABLED": "false"
"SERVICE_PORT": "<8080>",
"AUTHZ_ACCESS_OBJECT_ENABLED": "false",
"POSTGRES_HOST": "url to postgresql database"
```
## CLI flags
 - `devices` - A directory containing data of known devices (devices.csv) & sensorTypes (sensorTypes.csv)
 - `policies` - An authorization policy file
 - `authz-access-object` - Enable the access-object authorization policy result model
 - `config` - Device management configuration file (`config.yaml`)
 - `devmode` - Enable dev mode (parsed but currently unused after parsing)

## Faktisk konfiguration (kod ar facit, HARM-002)
Precedens: default < miljovariabel < CLI-flagga. RabbitMQ konfigureras via `messaging.LoadConfiguration`.

| Variabel | Default | Notering |
| --- | --- | --- |
| `LISTEN_ADDRESS` | `0.0.0.0` | Galler bade publik server och kontrollserver |
| `SERVICE_PORT` | `8080` | Publik server (`/api/v0/...`, `/openapi.yaml`, `/docs`) |
| `CONTROL_PORT` | `8000` | Kontrollserver: pprof, liveness, readiness-stubbar (`rabbitmq`, `timescale`) som returnerar OK |
| `POLICIES_FILE` | `/opt/diwise/config/authz.rego` | Kravs vid startup |
| `AUTHZ_ACCESS_OBJECT_ENABLED` | `false` | Switches between the `tenants` and `access` result models (see Security above) |
| `ALLOWED_SEED_TENANTS` | `default` | Kommaseparerad lista vid seedning |
| `SEED_EXISTING_DEVICES` | `true` |  |
| `POSTGRES_HOST` | (tom) | Lagring ar enbart PostgreSQL/pgx i nulaget |
| `POSTGRES_PORT` | `5432` |  |
| `POSTGRES_DBNAME` | `diwise` |  |
| `POSTGRES_USER` | (tom) |  |
| `POSTGRES_PASSWORD` | (tom) |  |
| `POSTGRES_SSLMODE` | `disable` |  |
| `ENABLE_TRACING` | `true` | Tracing pa publik server |
| `RABBITMQ_HOST` | (tom, kravs om inte avstangd) | Se `messaging.LoadConfiguration` |
| `RABBITMQ_PORT` | `5672` |  |
| `RABBITMQ_VHOST` | `/` |  |
| `RABBITMQ_USER` | `user` |  |
| `RABBITMQ_PASS` | `bitnami` |  |
| `RABBITMQ_DISABLED` | `false` |  |
| `RABBITMQ_INIT_TIMEOUT` | `10` | Sekunder |
| `POSTGRES_MAX_CONNS` | `10` |  |
| `POSTGRES_MIN_CONNS` | `2` |  |
| `POSTGRES_MAX_CONN_LIFETIME` | `30m` |  |
| `POSTGRES_MAX_CONN_IDLE_TIME` | `5m` |  |
| `POSTGRES_HEALTH_CHECK_PERIOD` | `30s` |  |

Filer som kravs vid startup: `config.yaml` (default `/opt/diwise/config/config.yaml`), `devices.csv` (default `/opt/diwise/config/devices.csv`), `authz.rego` (default `/opt/diwise/config/authz.rego`).
Not: avsnittet om SQLite-fallback under Storage och `notifications.yaml` nedan beskriver aldre beteende och galler inte for aktuell kod.

Health paths pa kontrollservern (`CONTROL_PORT`): `/health`, `/healthz`, `/livez`, `/readyz`, `/readyz/{check}`.

Externa Kubernetes- och Compose-definitioner finns inte i detta repo och ar darfor inte inventerade har.

## Configuration files
First row of csv-files contains headers.
### devices.csv
```
devEUI;internalID;lat;lon;where;types;sensorType;name;description;active;tenant;interval;source
a81758fffe06bfa3;intern-a81758fffe06bfa3;62.39160;17.30723;water;urn:oma:lwm2m:ext:3303,urn:oma:lwm2m:ext:3302,urn:oma:lwm2m:ext:3301;elsys;name-a81758fffe06bfa3;desc-a81758fffe06bfa3;true;default;0;origin
```
 - `devEUI` - id of physical sensor
 - `internalID` - internal id that will be used within the diwise plattform
 - `lat` - latitude 
 - `lon` - longitude
 - `where` - environment
 - `types` - measurement types that will be converted from the sensor payload
 - `sensorType` - name of decoder that the sensor will use 
 - `name` - display name of sensor
 - `description` - description
 - `active` - if set to false measurements will not be delivered
 - `tenant` - name of tenant 
 - `interval` - overrides interval set in sensorTypes
 - `source` - name of the source

### notifications.yaml
Configuration if a [cloud event](https://cloudevents.io/) should be sent to the configured endpoint.
```yaml
notifications:
  - id: qalcosonic
    name: Qalcosonic W1 StatusCodes
    type: diwise.statusmessage
    subscribers:
    - endpoint: http://endpoint/api/cloudevents
```

# Links
[iot-device-mgmt](https://diwise.github.io/) on diwise.github.io
