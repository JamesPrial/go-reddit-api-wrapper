package postgres

// Post Converters for PostgreSQL Backend
//
// This file is reserved for PostgreSQL-specific post conversion logic.
// Most conversion logic is shared via storage/converters.go and can be
// reused directly from that package.
//
// When implementing the full PostgreSQL backend:
//
// 1. Shared Converters:
//    The storage/converters.go package provides:
//    - postToInsertArgs() - converts a types.Post to database insert arguments
//    - commentToInsertArgs() - converts a types.Comment to database insert arguments
//    - scanPostRow() - helper for scanning post rows from query results
//    - scanCommentRow() - helper for scanning comment rows from query results
//
//    These can be imported and used directly for PostgreSQL since the schema
//    structure is identical (only SQL syntax differs: $1 vs ?, timestamp handling, etc.)
//
// 2. PostgreSQL-Specific Adaptations (if needed):
//    - Boolean value handling (PostgreSQL uses BOOLEAN type, SQLite uses INTEGER)
//    - Timestamp precision (PostgreSQL timestamps have microsecond precision)
//    - JSON handling (PostgreSQL has native JSONB support)
//    - NULL handling and type conversions
//
// 3. Row Scanning:
//    When retrieving posts from PostgreSQL:
//    ```go
//    func (s *PostgresStore) scanPostRow(row *sql.Row) (*types.Post, error) {
//        // Use shared converter from storage package
//        return scanPostRow(row, s.db)
//    }
//    ```
//
// Example usage in future ListPosts implementation:
//
//   rows, err := s.db.QueryContext(ctx, query, args...)
//   if err != nil {
//       return nil, err
//   }
//   defer rows.Close()
//
//   var posts []*types.Post
//   for rows.Next() {
//       post, err := scanPostRow(rows, s.db)  // Use shared converter
//       if err != nil {
//           return nil, err
//       }
//       posts = append(posts, post)
//   }
//   return posts, rows.Err()
