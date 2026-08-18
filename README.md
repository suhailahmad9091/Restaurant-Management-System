# Restaurant Management System API

A REST API for managing restaurants and their menus, written in Go. Access is
role based: an **admin** manages sub-admins, users, restaurants and dishes; a
**sub-admin** manages the users and restaurants they created; a **user** browses
restaurants and dishes and can measure how far a restaurant is from one of their
saved addresses.

## Stack

| Concern | Library |
| --- | --- |
| Routing | [chi](https://github.com/go-chi/chi) v5 |
| Database | PostgreSQL 13 via [sqlx](https://github.com/jmoiron/sqlx) + [lib/pq](https://github.com/lib/pq) |
| Migrations | [golang-migrate](https://github.com/golang-migrate/migrate) |
| Auth | [golang-jwt](https://github.com/golang-jwt/jwt) v5, HS256 |
| Validation | [validator](https://github.com/go-playground/validator) v10 |
| Passwords | bcrypt |
| Concurrency | [errgroup](https://pkg.go.dev/golang.org/x/sync/errgroup) |
| Logging | [logrus](https://github.com/sirupsen/logrus) |

Requires Go 1.22 or newer.

## Getting started

Start Postgres:

```bash
docker compose up -d
```

This runs `postgres:13` on host port **5434**, with database `rms` and user
`local` / `local`. Data persists in `./pgdata`.

Set the environment and run the server:

```bash
export DB_HOST=localhost
export DB_PORT=5434
export DB_NAME=rms
export DB_USER=local
export DB_PASS=local
export JWT_SECRET_KEY=change-me

go run ./cmd
```

The server listens on **:8080**. Migrations from `database/migrations` run
automatically at startup, so there is no separate migrate step.

Run it from the repository root — the migration source is the relative path
`file://database/migrations`, so starting from elsewhere fails to find it.

### Environment variables

| Variable | Description |
| --- | --- |
| `DB_HOST` | Postgres host |
| `DB_PORT` | Postgres port (`5434` with the bundled compose file) |
| `DB_NAME` | Database name |
| `DB_USER` | Database user |
| `DB_PASS` | Database password |
| `JWT_SECRET_KEY` | Secret used to sign and verify tokens |

### Creating the first admin

There is no public sign-up: every account-creating route requires an admin or
sub-admin token, so the first admin has to be inserted by hand. Hash a password
with bcrypt, then:

```sql
INSERT INTO users (name, email, password, role)
VALUES ('Admin', 'admin@example.com', '<bcrypt-hash>', 'admin');
```

Log in with that account to get a token, and create everything else through the
API.

## Authentication

`POST /v1/login` returns a JWT. Send it on protected routes in a **`token`**
header — not `Authorization: Bearer`:

```bash
curl -H "token: <jwt>" http://localhost:8080/v1/user/all-restaurants
```

Tokens expire one hour after issue. Each token also carries a session id and the
caller's role; logging out archives the session, which invalidates the token
immediately even though it has not expired yet. The role is read from the token,
so a role change only takes effect on the next login.

## API

Base path `/v1`. Everything except `/login` requires a token, and the `/admin`,
`/sub-admin` and `/user` groups additionally require the matching role — a
mismatch is `403`.

### Public

| Method | Path | Body | Success |
| --- | --- | --- | --- |
| `POST` | `/login` | `email`, `password` | `200` + token |

`password` must be 6–15 characters.

### Any authenticated caller

| Method | Path | Body | Success |
| --- | --- | --- | --- |
| `POST` | `/logout` | — | `200` |
| `GET` | `/dishes-by-restaurant` | `restaurantId` | `200` |

### Admin — `/admin`

| Method | Path | Body | Success |
| --- | --- | --- | --- |
| `POST` | `/create-sub-admin` | `name`, `email`, `password` | `201` |
| `GET` | `/all-sub-admin` | — | `200` |
| `POST` | `/create-user` | `name`, `email`, `password`, `address[]` | `201` |
| `GET` | `/all-users` | — | `200` |
| `POST` | `/create-restaurant` | `name`, `address`, `latitude`, `longitude` | `201` |
| `GET` | `/all-restaurants` | — | `200` |
| `POST` | `/{restaurantId}/` | `name`, `price` | `201` |
| `GET` | `/all-dishes` | — | `200` |

Each entry in `address[]` is `address`, `latitude`, `longitude`. Creating a user
inserts the account and all of its addresses in a single transaction, so a
failure on any address leaves no half-built user behind.

### Sub-admin — `/sub-admin`

The same routes minus sub-admin management, and the list endpoints are scoped to
rows the caller created:

| Method | Path | Success | Notes |
| --- | --- | --- | --- |
| `POST` | `/create-user` | `201` | |
| `GET` | `/all-users` | `200` | Only users this sub-admin created |
| `POST` | `/create-restaurant` | `201` | |
| `GET` | `/all-restaurants` | `200` | Only restaurants this sub-admin created |
| `POST` | `/{restaurantId}/` | `201` | |
| `GET` | `/all-dishes` | `200` | Only dishes of this sub-admin's restaurants |

### User — `/user`

| Method | Path | Body | Success |
| --- | --- | --- | --- |
| `GET` | `/all-restaurants` | — | `200` |
| `GET` | `/all-dishes` | — | `200` |
| `GET` | `/calculate-distance` | `userAddressId`, `restaurantAddressId` | `200` |

`/calculate-distance` returns `{"distance": 4.2}`, in kilometres rounded to one
decimal. The two lookups run concurrently through `errgroup`, and the distance
itself is computed by Postgres using the `cube` and `earthdistance` extensions.

Emails are unique among active users. Restaurant names are unique per address,
and dish names are unique per restaurant.

### Status codes

| Code | Meaning |
| --- | --- |
| `200` | Success |
| `201` | User, sub-admin, restaurant, or dish created |
| `400` | Malformed body or failed validation |
| `401` | Missing, invalid, or expired token; wrong email or password |
| `403` | Token is valid but carries the wrong role for the route |
| `404` | Address does not exist or is archived |
| `409` | Email already registered, or that restaurant or dish already exists |
| `500` | Something failed server-side |

Errors come back as:

```json
{
  "id": "aB3xY9z",
  "messageToUser": "invalid email or password",
  "developerInfo": "invalid email or password",
  "error": "invalid credentials",
  "statusCode": 401,
  "isClientError": true
}
```

The `id` is generated per error and written to the server log, so quoting it in
a bug report points straight at the matching log line.

Note that `developerInfo` and `error` carry internal detail — for a driver-level
failure the raw message names it exactly. That is useful in development, but
before this runs anywhere public those two fields should be logged only, not
serialized to the client.

Login answers `401` with the same message whether the email is unknown or the
password is wrong, so the endpoint cannot be used to discover which emails are
registered.

## Data model

Five tables, created by `database/migrations`:

- **users** — id, name, email, password hash, `role` (`admin`, `sub-admin`, `user`), `created_by`
- **address** — id, user_id, address, latitude, longitude
- **restaurants** — id, name, address, latitude, longitude, `created_by`
- **dishes** — id, restaurant_id, name, price
- **user_session** — id, user_id, created_at

`created_by` is what scopes a sub-admin's view: their user and restaurant lists
filter on it.

Nothing is ever hard deleted. Rows carry `archived_at`, and every query filters
on `archived_at IS NULL`. Unique constraints are partial indexes limited to
active rows, so an email or restaurant name frees up once the row is archived.

## Layout

```
cmd/                  entrypoint, graceful shutdown
server/               router, timeouts
handlers/             HTTP handlers: parse, validate, respond
middlewares/          auth, role checks, CORS, panic recovery
database/             connection, migrations, transaction helper
database/dbHelper/    SQL queries
models/               request, response, and database structs
utils/                JSON, JWT, bcrypt, validation, error responses
```

Handlers own HTTP concerns and `dbHelper` owns SQL; helpers return errors and
never write to the response themselves. Handlers that need more than one
statement to be atomic wrap them in `database.Tx`, which commits on success and
rolls back on any returned error — and reports a failed commit rather than
letting the handler answer `201` for a transaction that never landed.

## Testing with Postman

`rms.postman_environment.json` defines `v1` (`localhost:8080/v1`) and an empty
`token` variable. Import it along with `rms.postman_collection.json`, log in,
and paste the returned token into `token`.
