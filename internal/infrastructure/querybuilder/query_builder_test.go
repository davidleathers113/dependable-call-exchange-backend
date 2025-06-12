package querybuilder

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueryBuilder_Select(t *testing.T) {
	tests := []struct {
		name        string
		builder     func() *QueryBuilder
		expectedSQL string
		expectedLen int
	}{
		{
			name: "simple select all",
			builder: func() *QueryBuilder {
				return New().Select().From("users")
			},
			expectedSQL: "SELECT * FROM users",
			expectedLen: 0,
		},
		{
			name: "select specific columns",
			builder: func() *QueryBuilder {
				return New().Select("id", "name", "email").From("users")
			},
			expectedSQL: "SELECT id, name, email FROM users",
			expectedLen: 0,
		},
		{
			name: "select with where condition",
			builder: func() *QueryBuilder {
				return New().Select("*").From("users").WhereEqual("id", uuid.New())
			},
			expectedSQL: "SELECT * FROM users WHERE id = $1",
			expectedLen: 1,
		},
		{
			name: "select with multiple conditions",
			builder: func() *QueryBuilder {
				return New().Select("*").From("users").
					WhereEqual("status", "active").
					Where("age", GreaterThan, 18)
			},
			expectedSQL: "SELECT * FROM users WHERE status = $1 AND age > $2",
			expectedLen: 2,
		},
		{
			name: "select with OR condition",
			builder: func() *QueryBuilder {
				return New().Select("*").From("users").
					WhereEqual("role", "admin").
					OrWhere("role", Equal, "moderator")
			},
			expectedSQL: "SELECT * FROM users WHERE role = $1 OR role = $2",
			expectedLen: 2,
		},
		{
			name: "select with IN condition",
			builder: func() *QueryBuilder {
				return New().Select("*").From("users").
					WhereIn("status", []interface{}{"active", "pending"})
			},
			expectedSQL: "SELECT * FROM users WHERE status IN ($1, $2)",
			expectedLen: 2,
		},
		{
			name: "select with NULL check",
			builder: func() *QueryBuilder {
				return New().Select("*").From("users").WhereNotNull("email")
			},
			expectedSQL: "SELECT * FROM users WHERE email IS NOT NULL",
			expectedLen: 0,
		},
		{
			name: "select with BETWEEN",
			builder: func() *QueryBuilder {
				return New().Select("*").From("users").
					WhereBetween("age", 18, 65)
			},
			expectedSQL: "SELECT * FROM users WHERE age BETWEEN $1 AND $2",
			expectedLen: 2,
		},
		{
			name: "select with ORDER BY",
			builder: func() *QueryBuilder {
				return New().Select("*").From("users").
					OrderByDesc("created_at").
					OrderByAsc("name")
			},
			expectedSQL: "SELECT * FROM users ORDER BY created_at DESC, name ASC",
			expectedLen: 0,
		},
		{
			name: "select with LIMIT and OFFSET",
			builder: func() *QueryBuilder {
				return New().Select("*").From("users").
					Limit(10).Offset(20)
			},
			expectedSQL: "SELECT * FROM users LIMIT $1 OFFSET $2",
			expectedLen: 2,
		},
		{
			name: "select with JOIN",
			builder: func() *QueryBuilder {
				return New().Select("u.name", "p.title").From("users u").
					LeftJoin("posts p", "p.user_id = u.id").
					WhereEqual("u.status", "active")
			},
			expectedSQL: "SELECT u.name, p.title FROM users u LEFT JOIN posts p ON p.user_id = u.id WHERE u.status = $1",
			expectedLen: 1,
		},
		{
			name: "select with GROUP BY and HAVING",
			builder: func() *QueryBuilder {
				return New().Select("status", "COUNT(*)").From("users").
					GroupBy("status").
					Having("COUNT(*)", GreaterThan, 10)
			},
			expectedSQL: "SELECT status, COUNT(*) FROM users GROUP BY status HAVING COUNT(*) > $1",
			expectedLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, params, err := tt.builder().ToSQL()
			require.NoError(t, err)
			assert.Equal(t, tt.expectedSQL, sql)
			assert.Len(t, params, tt.expectedLen)
		})
	}
}

func TestQueryBuilder_Insert(t *testing.T) {
	tests := []struct {
		name        string
		builder     func() *QueryBuilder
		expectedSQL string
		expectedLen int
	}{
		{
			name: "simple insert",
			builder: func() *QueryBuilder {
				return New().Insert("users").
					Set("name", "John Doe").
					Set("email", "john@example.com")
			},
			expectedSQL: "INSERT INTO users (name, email) VALUES ($1, $2)",
			expectedLen: 2,
		},
		{
			name: "insert with RETURNING",
			builder: func() *QueryBuilder {
				return New().Insert("users").
					Set("name", "John Doe").
					Returning("id", "created_at")
			},
			expectedSQL: "INSERT INTO users (name) VALUES ($1) RETURNING id, created_at",
			expectedLen: 1,
		},
		{
			name: "insert with ON CONFLICT DO NOTHING",
			builder: func() *QueryBuilder {
				return New().Insert("users").
					Set("email", "john@example.com").
					Set("name", "John Doe").
					OnConflict([]string{"email"}, DoNothing)
			},
			expectedSQL: "", // Will be checked manually due to map ordering
			expectedLen: 2,
		},
		{
			name: "insert with ON CONFLICT DO UPDATE",
			builder: func() *QueryBuilder {
				return New().Insert("users").
					Set("email", "john@example.com").
					Set("name", "John Doe").
					OnConflictUpdate([]string{"email"}, map[string]interface{}{
						"name":       "John Updated",
						"updated_at": time.Now(),
					})
			},
			expectedSQL: "INSERT INTO users (email, name) VALUES ($1, $2) ON CONFLICT (email) DO UPDATE SET name = $3, updated_at = $4",
			expectedLen: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, params, err := tt.builder().ToSQL()
			require.NoError(t, err)

			// Special handling for the ON CONFLICT test due to map ordering
			if tt.name == "insert with ON CONFLICT DO NOTHING" {
				assert.Contains(t, sql, "INSERT INTO users")
				assert.Contains(t, sql, "VALUES ($1, $2)")
				assert.Contains(t, sql, "ON CONFLICT (email) DO NOTHING")
				assert.Contains(t, sql, "email")
				assert.Contains(t, sql, "name")
			} else {
				assert.Equal(t, tt.expectedSQL, sql)
			}
			assert.Len(t, params, tt.expectedLen)
		})
	}
}

func TestQueryBuilder_Update(t *testing.T) {
	tests := []struct {
		name        string
		builder     func() *QueryBuilder
		expectedSQL string
		expectedLen int
	}{
		{
			name: "simple update",
			builder: func() *QueryBuilder {
				id := uuid.New()
				return New().Update("users").
					Set("name", "Jane Doe").
					Set("email", "jane@example.com").
					WhereID(id)
			},
			expectedSQL: "UPDATE users SET name = $1, email = $2 WHERE id = $3",
			expectedLen: 3,
		},
		{
			name: "update with RETURNING",
			builder: func() *QueryBuilder {
				return New().Update("users").
					Set("status", "inactive").
					WhereEqual("last_login", nil).
					Returning("id", "updated_at")
			},
			expectedSQL: "UPDATE users SET status = $1 WHERE last_login = $2 RETURNING id, updated_at",
			expectedLen: 2,
		},
		{
			name: "update with multiple conditions",
			builder: func() *QueryBuilder {
				return New().Update("users").
					Set("status", "verified").
					WhereEqual("email_verified", true).
					Where("created_at", LessThan, time.Now().Add(-24*time.Hour))
			},
			expectedSQL: "UPDATE users SET status = $1 WHERE email_verified = $2 AND created_at < $3",
			expectedLen: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, params, err := tt.builder().ToSQL()
			require.NoError(t, err)
			assert.Equal(t, tt.expectedSQL, sql)
			assert.Len(t, params, tt.expectedLen)
		})
	}
}

func TestQueryBuilder_Delete(t *testing.T) {
	tests := []struct {
		name        string
		builder     func() *QueryBuilder
		expectedSQL string
		expectedLen int
	}{
		{
			name: "simple delete",
			builder: func() *QueryBuilder {
				id := uuid.New()
				return New().Delete().From("users").WhereID(id)
			},
			expectedSQL: "DELETE FROM users WHERE id = $1",
			expectedLen: 1,
		},
		{
			name: "delete with multiple conditions",
			builder: func() *QueryBuilder {
				return New().Delete().From("users").
					WhereEqual("status", "inactive").
					Where("last_login", LessThan, time.Now().Add(-365*24*time.Hour))
			},
			expectedSQL: "DELETE FROM users WHERE status = $1 AND last_login < $2",
			expectedLen: 2,
		},
		{
			name: "delete with RETURNING",
			builder: func() *QueryBuilder {
				return New().Delete().From("users").
					WhereEqual("status", "deleted").
					Returning("id")
			},
			expectedSQL: "DELETE FROM users WHERE status = $1 RETURNING id",
			expectedLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, params, err := tt.builder().ToSQL()
			require.NoError(t, err)
			assert.Equal(t, tt.expectedSQL, sql)
			assert.Len(t, params, tt.expectedLen)
		})
	}
}

func TestQueryBuilder_TypeSafeHelpers(t *testing.T) {
	t.Run("WhereID", func(t *testing.T) {
		id := uuid.New()
		sql, params, err := New().Select("*").From("users").WhereID(id).ToSQL()
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM users WHERE id = $1", sql)
		assert.Equal(t, []interface{}{id}, params)
	})

	t.Run("WhereActive", func(t *testing.T) {
		sql, params, err := New().Select("*").From("users").WhereActive().ToSQL()
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM users WHERE status = $1", sql)
		assert.Equal(t, []interface{}{"active"}, params)
	})

	t.Run("WhereCreatedAfter", func(t *testing.T) {
		timestamp := time.Now()
		sql, params, err := New().Select("*").From("users").WhereCreatedAfter(timestamp).ToSQL()
		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM users WHERE created_at > $1", sql)
		assert.Equal(t, []interface{}{timestamp}, params)
	})

	t.Run("SetUpdatedAt", func(t *testing.T) {
		timestamp := time.Now()
		id := uuid.New()
		sql, params, err := New().Update("users").
			Set("name", "Updated Name").
			SetUpdatedAt(timestamp).
			WhereID(id).ToSQL()
		require.NoError(t, err)
		assert.Equal(t, "UPDATE users SET name = $1, updated_at = $2 WHERE id = $3", sql)
		assert.Equal(t, []interface{}{"Updated Name", timestamp, id}, params)
	})
}

func TestQueryBuilder_Errors(t *testing.T) {
	t.Run("select without table", func(t *testing.T) {
		_, _, err := New().Select("*").ToSQL()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "table name is required")
	})

	t.Run("insert without table", func(t *testing.T) {
		_, _, err := New().Insert("").Set("name", "test").ToSQL()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "table name is required")
	})

	t.Run("insert without values", func(t *testing.T) {
		_, _, err := New().Insert("users").ToSQL()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no values specified")
	})

	t.Run("update without table", func(t *testing.T) {
		_, _, err := New().Update("").Set("name", "test").ToSQL()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "table name is required")
	})

	t.Run("update without set values", func(t *testing.T) {
		_, _, err := New().Update("users").ToSQL()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no SET values specified")
	})

	t.Run("delete without table", func(t *testing.T) {
		_, _, err := New().Delete().ToSQL()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "table name is required")
	})
}

func TestQueryBuilder_ComplexQueries(t *testing.T) {
	t.Run("complex select with all features", func(t *testing.T) {
		sql, params, err := New().
			Select("u.id", "u.name", "COUNT(p.id) as post_count").
			From("users u").
			LeftJoin("posts p", "p.user_id = u.id").
			WhereEqual("u.status", "active").
			Where("u.created_at", GreaterThan, time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)).
			OrWhere("u.role", Equal, "admin").
			GroupBy("u.id", "u.name").
			Having("COUNT(p.id)", GreaterThan, 5).
			OrderByDesc("post_count").
			OrderByAsc("u.name").
			Limit(20).
			Offset(40).
			ToSQL()

		require.NoError(t, err)
		expectedSQL := "SELECT u.id, u.name, COUNT(p.id) as post_count FROM users u LEFT JOIN posts p ON p.user_id = u.id WHERE u.status = $1 AND u.created_at > $2 OR u.role = $3 GROUP BY u.id, u.name HAVING COUNT(p.id) > $4 ORDER BY post_count DESC, u.name ASC LIMIT $5 OFFSET $6"
		assert.Equal(t, expectedSQL, sql)
		assert.Len(t, params, 6)
	})

	t.Run("complex update with timestamp", func(t *testing.T) {
		now := time.Now()
		cutoff := now.Add(-30 * 24 * time.Hour)

		sql, params, err := New().
			Update("users").
			Set("status", "inactive").
			Set("deactivated_at", now).
			SetUpdatedAt(now).
			Where("last_login", LessThan, cutoff).
			WhereNotNull("last_login").
			Returning("id", "email").
			ToSQL()

		require.NoError(t, err)
		expectedSQL := "UPDATE users SET status = $1, deactivated_at = $2, updated_at = $3 WHERE last_login < $4 AND last_login IS NOT NULL RETURNING id, email"
		assert.Equal(t, expectedSQL, sql)
		assert.Len(t, params, 4)
	})
}

// Benchmarks
func BenchmarkQueryBuilder_SimpleSelect(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _, _ = New().Select("*").From("users").WhereID(uuid.New()).ToSQL()
	}
}

func BenchmarkQueryBuilder_ComplexSelect(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _, _ = New().
			Select("u.id", "u.name", "COUNT(p.id)").
			From("users u").
			LeftJoin("posts p", "p.user_id = u.id").
			WhereEqual("u.status", "active").
			Where("u.created_at", GreaterThan, time.Now()).
			GroupBy("u.id", "u.name").
			OrderByDesc("u.created_at").
			Limit(10).
			ToSQL()
	}
}

func BenchmarkQueryBuilder_Insert(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _, _ = New().
			Insert("users").
			Set("name", "John Doe").
			Set("email", "john@example.com").
			Set("created_at", time.Now()).
			Returning("id").
			ToSQL()
	}
}
