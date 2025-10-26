package postgres

// This file documents PostgreSQL-specific SQL syntax and implementation notes
// for future development of the PostgreSQL backend.
//
// Key Differences from SQLite:
//
// 1. Placeholder Syntax:
//    SQLite:      ? (single placeholder for all parameters)
//    PostgreSQL:  $1, $2, $3, ... (numbered placeholders)
//
//    Example:
//      SQLite:      "SELECT * FROM posts WHERE id = ? AND author = ?"
//      PostgreSQL:  "SELECT * FROM posts WHERE id = $1 AND author = $2"
//
// 2. Boolean Type:
//    SQLite:      INTEGER (0 for false, 1 for true)
//    PostgreSQL:  BOOLEAN (true/false, more efficient)
//
// 3. Timestamp Handling:
//    SQLite:      strftime('%s', 'now') returns Unix timestamp as string
//    PostgreSQL:  EXTRACT(EPOCH FROM NOW())::BIGINT or (now() at time zone 'utc')
//
//    Example:
//      SQLite:      "INSERT INTO posts (..., fetched_at) VALUES (..., strftime('%s', 'now'))"
//      PostgreSQL:  "INSERT INTO posts (..., fetched_at) VALUES (..., EXTRACT(EPOCH FROM NOW())::BIGINT)"
//
// 4. UPSERT Syntax (same in both):
//    Both SQLite and PostgreSQL support:
//      INSERT ... ON CONFLICT(id) DO UPDATE SET ...
//
// 5. JSON Support:
//    SQLite:      TEXT or JSON1 extension (limited query capabilities)
//    PostgreSQL:  JSONB (native, indexed, queryable with powerful operators)
//
// 6. Case Sensitivity:
//    SQLite:      Identifiers are case-insensitive by default
//    PostgreSQL:  Identifiers are case-insensitive, but stored in lowercase unless quoted
//                 String comparisons are case-sensitive by default (use LOWER/UPPER for case-insensitive)
//
// 7. Available Drivers:
//    lib/pq:      github.com/lib/pq - Standard PostgreSQL driver with database/sql support
//    pgx:         github.com/jackc/pgx - High-performance native PostgreSQL driver
//                 (can use either database/sql interface or native interface)
//
// 8. Connection String Formats:
//    lib/pq:      "postgres://user:password@host:port/dbname"
//                 "host=localhost user=postgres password=secret dbname=reddit port=5432"
//    pgx:         "postgres://user:password@host:port/dbname"
//
// Implementation Notes:
//
// - PostgreSQL supports TRUE/FALSE keywords for boolean values
// - String literals must use single quotes, not double quotes
// - Column names can be quoted with double quotes for case sensitivity
// - PostgreSQL has stricter type checking than SQLite
// - Use RETURNING clause for getting generated values (SERIAL auto-increment)
// - Consider PARTIAL INDEX for optimizing queries on sparse data
// - Use ARRAY types or JSONB for complex data structures (more efficient than JSON)
//
// Example UPSERT with PostgreSQL placeholders:
//
//   INSERT INTO posts (
//       id, name, score, author, subreddit, created_utc, fetched_at
//   ) VALUES (
//       $1, $2, $3, $4, $5, $6, EXTRACT(EPOCH FROM NOW())::BIGINT
//   )
//   ON CONFLICT(id) DO UPDATE SET
//       name = EXCLUDED.name,
//       score = EXCLUDED.score,
//       author = EXCLUDED.author,
//       subreddit = EXCLUDED.subreddit,
//       created_utc = EXCLUDED.created_utc,
//       fetched_at = EXTRACT(EPOCH FROM NOW())::BIGINT
//
// Example Case-Insensitive Subreddit Filter:
//
//   SELECT * FROM posts
//   WHERE LOWER(subreddit) = LOWER($1)
//   ORDER BY created_utc DESC
//
// Migration Tool:
// Use golang-migrate (github.com/golang-migrate/migrate) with PostgreSQL driver:
//
//   import _ "github.com/golang-migrate/migrate/v4/database/postgres"
//   import _ "github.com/golang-migrate/migrate/v4/source/file"
