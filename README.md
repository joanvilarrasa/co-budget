# CO Budget

## Summary

CO Budget is an experimental application to try Datastar in a real project.
The goal is to turn this into a practical app for me and my girlfriend to manage our finances together.

## Technologies

- Go (`net/http`) for the backend server
- Datastar for reactive HTML updates
- Server-Sent Events (SSE) for live UI patching
- SQLite (via `modernc.org/sqlite`) for local persistence
- Tailwind CSS + Basecoat for UI styling/components

## Architecture

- `main.go`: application bootstrap (database setup + HTTP server start)
- `database/`: SQLite connection and SQL migration runner
- `data/`: account store and domain operations (create, update, delete, query)
- `server/`: HTTP routes, form parsing, JSON responses, SSE broadcasting
- `app/`: HTML templates and page/table rendering
- `lib/`: shared helpers (template parsing and utilities)

The server handles account form submissions, updates persistence through the data layer, and broadcasts table patches over SSE so connected clients stay in sync.

## Start Development

1. Install Go (version `1.26` or compatible with `go.mod`).
2. Install dependencies:

```bash
go mod tidy
```

3. Run the app:

```bash
go run .
```

4. Open `http://localhost:8080`.

Notes:
- The SQLite database file is created at `./db.sqlite`.
- SQL migrations in `./database` are applied automatically at startup.
