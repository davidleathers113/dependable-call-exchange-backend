package querybuilder

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// QueryBuilder provides a fluent interface for building SQL queries with type safety
type QueryBuilder struct {
	queryType  QueryType
	table      string
	columns    []string
	values     []interface{}
	conditions []Condition
	joins      []Join
	orderBy    []OrderBy
	groupBy    []string
	having     []Condition
	limit      *int
	offset     *int
	returning  []string
	setValues  map[string]interface{}
	onConflict *ConflictClause
}

// QueryType represents the type of SQL query
type QueryType int

const (
	SelectQuery QueryType = iota
	InsertQuery
	UpdateQuery
	DeleteQuery
)

// Condition represents a WHERE/HAVING condition
type Condition struct {
	Column   string
	Operator Operator
	Value    interface{}
	Logical  LogicalOperator
}

// Join represents a SQL JOIN clause
type Join struct {
	Type      JoinType
	Table     string
	Condition string
}

// OrderBy represents an ORDER BY clause
type OrderBy struct {
	Column    string
	Direction Direction
}

// ConflictClause represents an ON CONFLICT clause for INSERT
type ConflictClause struct {
	Columns []string
	Action  ConflictAction
	Updates map[string]interface{}
}

// Operator represents SQL comparison operators
type Operator int

const (
	Equal Operator = iota
	NotEqual
	GreaterThan
	GreaterThanOrEqual
	LessThan
	LessThanOrEqual
	Like
	ILike
	In
	NotIn
	IsNull
	IsNotNull
	Between
	NotBetween
)

// LogicalOperator represents logical operators (AND, OR)
type LogicalOperator int

const (
	And LogicalOperator = iota
	Or
)

// JoinType represents SQL JOIN types
type JoinType int

const (
	InnerJoin JoinType = iota
	LeftJoin
	RightJoin
	FullJoin
)

// Direction represents sort direction
type Direction int

const (
	Asc Direction = iota
	Desc
)

// ConflictAction represents ON CONFLICT actions
type ConflictAction int

const (
	DoNothing ConflictAction = iota
	DoUpdate
)

// New creates a new QueryBuilder instance
func New() *QueryBuilder {
	return &QueryBuilder{
		setValues: make(map[string]interface{}),
	}
}

// Table Query Builder Methods

// Select starts a SELECT query
func (qb *QueryBuilder) Select(columns ...string) *QueryBuilder {
	qb.queryType = SelectQuery
	qb.columns = columns
	return qb
}

// Insert starts an INSERT query
func (qb *QueryBuilder) Insert(table string) *QueryBuilder {
	qb.queryType = InsertQuery
	qb.table = table
	return qb
}

// Update starts an UPDATE query
func (qb *QueryBuilder) Update(table string) *QueryBuilder {
	qb.queryType = UpdateQuery
	qb.table = table
	return qb
}

// Delete starts a DELETE query
func (qb *QueryBuilder) Delete() *QueryBuilder {
	qb.queryType = DeleteQuery
	return qb
}

// From sets the table for SELECT queries
func (qb *QueryBuilder) From(table string) *QueryBuilder {
	qb.table = table
	return qb
}

// Values sets the values for INSERT queries
func (qb *QueryBuilder) Values(values ...interface{}) *QueryBuilder {
	qb.values = values
	return qb
}

// Set adds a column=value pair for UPDATE/INSERT queries
func (qb *QueryBuilder) Set(column string, value interface{}) *QueryBuilder {
	qb.setValues[column] = value
	return qb
}

// Condition Builder Methods

// Where adds a WHERE condition with AND logic
func (qb *QueryBuilder) Where(column string, operator Operator, value interface{}) *QueryBuilder {
	return qb.addCondition(column, operator, value, And)
}

// OrWhere adds a WHERE condition with OR logic
func (qb *QueryBuilder) OrWhere(column string, operator Operator, value interface{}) *QueryBuilder {
	return qb.addCondition(column, operator, value, Or)
}

// WhereEqual is a convenience method for equality conditions
func (qb *QueryBuilder) WhereEqual(column string, value interface{}) *QueryBuilder {
	return qb.Where(column, Equal, value)
}

// WhereIn adds an IN condition
func (qb *QueryBuilder) WhereIn(column string, values []interface{}) *QueryBuilder {
	return qb.Where(column, In, values)
}

// WhereNotNull adds an IS NOT NULL condition
func (qb *QueryBuilder) WhereNotNull(column string) *QueryBuilder {
	return qb.Where(column, IsNotNull, nil)
}

// WhereBetween adds a BETWEEN condition
func (qb *QueryBuilder) WhereBetween(column string, start, end interface{}) *QueryBuilder {
	return qb.Where(column, Between, []interface{}{start, end})
}

// Join Methods

// InnerJoin adds an INNER JOIN
func (qb *QueryBuilder) InnerJoin(table, condition string) *QueryBuilder {
	return qb.addJoin(InnerJoin, table, condition)
}

// LeftJoin adds a LEFT JOIN
func (qb *QueryBuilder) LeftJoin(table, condition string) *QueryBuilder {
	return qb.addJoin(LeftJoin, table, condition)
}

// RightJoin adds a RIGHT JOIN
func (qb *QueryBuilder) RightJoin(table, condition string) *QueryBuilder {
	return qb.addJoin(RightJoin, table, condition)
}

// Ordering and Grouping

// OrderBy adds an ORDER BY clause
func (qb *QueryBuilder) OrderBy(column string, direction Direction) *QueryBuilder {
	qb.orderBy = append(qb.orderBy, OrderBy{Column: column, Direction: direction})
	return qb
}

// OrderByAsc adds an ORDER BY ASC clause
func (qb *QueryBuilder) OrderByAsc(column string) *QueryBuilder {
	return qb.OrderBy(column, Asc)
}

// OrderByDesc adds an ORDER BY DESC clause
func (qb *QueryBuilder) OrderByDesc(column string) *QueryBuilder {
	return qb.OrderBy(column, Desc)
}

// GroupBy adds a GROUP BY clause
func (qb *QueryBuilder) GroupBy(columns ...string) *QueryBuilder {
	qb.groupBy = append(qb.groupBy, columns...)
	return qb
}

// Having adds a HAVING condition
func (qb *QueryBuilder) Having(column string, operator Operator, value interface{}) *QueryBuilder {
	qb.having = append(qb.having, Condition{
		Column:   column,
		Operator: operator,
		Value:    value,
		Logical:  And,
	})
	return qb
}

// Limit and Offset

// Limit sets the LIMIT clause
func (qb *QueryBuilder) Limit(limit int) *QueryBuilder {
	qb.limit = &limit
	return qb
}

// Offset sets the OFFSET clause
func (qb *QueryBuilder) Offset(offset int) *QueryBuilder {
	qb.offset = &offset
	return qb
}

// Advanced Features

// Returning sets the RETURNING clause for INSERT/UPDATE/DELETE
func (qb *QueryBuilder) Returning(columns ...string) *QueryBuilder {
	qb.returning = columns
	return qb
}

// OnConflict sets the ON CONFLICT clause for INSERT
func (qb *QueryBuilder) OnConflict(columns []string, action ConflictAction) *QueryBuilder {
	qb.onConflict = &ConflictClause{
		Columns: columns,
		Action:  action,
		Updates: make(map[string]interface{}),
	}
	return qb
}

// OnConflictUpdate sets ON CONFLICT ... DO UPDATE
func (qb *QueryBuilder) OnConflictUpdate(columns []string, updates map[string]interface{}) *QueryBuilder {
	qb.onConflict = &ConflictClause{
		Columns: columns,
		Action:  DoUpdate,
		Updates: updates,
	}
	return qb
}

// Build Methods

// ToSQL generates the SQL query and parameter list
func (qb *QueryBuilder) ToSQL() (string, []interface{}, error) {
	switch qb.queryType {
	case SelectQuery:
		return qb.buildSelect()
	case InsertQuery:
		return qb.buildInsert()
	case UpdateQuery:
		return qb.buildUpdate()
	case DeleteQuery:
		return qb.buildDelete()
	default:
		return "", nil, fmt.Errorf("unknown query type")
	}
}

// Helper Methods

func (qb *QueryBuilder) addCondition(column string, operator Operator, value interface{}, logical LogicalOperator) *QueryBuilder {
	qb.conditions = append(qb.conditions, Condition{
		Column:   column,
		Operator: operator,
		Value:    value,
		Logical:  logical,
	})
	return qb
}

func (qb *QueryBuilder) addJoin(joinType JoinType, table, condition string) *QueryBuilder {
	qb.joins = append(qb.joins, Join{
		Type:      joinType,
		Table:     table,
		Condition: condition,
	})
	return qb
}

func (qb *QueryBuilder) buildSelect() (string, []interface{}, error) {
	if qb.table == "" {
		return "", nil, fmt.Errorf("table name is required for SELECT query")
	}

	var query strings.Builder
	var params []interface{}
	paramIndex := 1

	// SELECT clause
	query.WriteString("SELECT ")
	if len(qb.columns) == 0 {
		query.WriteString("*")
	} else {
		query.WriteString(strings.Join(qb.columns, ", "))
	}

	// FROM clause
	query.WriteString(" FROM ")
	query.WriteString(qb.table)

	// JOIN clauses
	for _, join := range qb.joins {
		query.WriteString(" ")
		query.WriteString(joinTypeToString(join.Type))
		query.WriteString(" ")
		query.WriteString(join.Table)
		query.WriteString(" ON ")
		query.WriteString(join.Condition)
	}

	// WHERE clause
	whereClause, whereParams, newIndex := qb.buildWhereClause(paramIndex)
	if whereClause != "" {
		query.WriteString(" WHERE ")
		query.WriteString(whereClause)
		params = append(params, whereParams...)
		paramIndex = newIndex
	}

	// GROUP BY clause
	if len(qb.groupBy) > 0 {
		query.WriteString(" GROUP BY ")
		query.WriteString(strings.Join(qb.groupBy, ", "))
	}

	// HAVING clause
	if len(qb.having) > 0 {
		havingClause, havingParams, newIndex := qb.buildConditions(qb.having, paramIndex)
		query.WriteString(" HAVING ")
		query.WriteString(havingClause)
		params = append(params, havingParams...)
		paramIndex = newIndex
	}

	// ORDER BY clause
	if len(qb.orderBy) > 0 {
		query.WriteString(" ORDER BY ")
		orderClauses := make([]string, len(qb.orderBy))
		for i, order := range qb.orderBy {
			direction := "ASC"
			if order.Direction == Desc {
				direction = "DESC"
			}
			orderClauses[i] = fmt.Sprintf("%s %s", order.Column, direction)
		}
		query.WriteString(strings.Join(orderClauses, ", "))
	}

	// LIMIT clause
	if qb.limit != nil {
		query.WriteString(fmt.Sprintf(" LIMIT $%d", paramIndex))
		params = append(params, *qb.limit)
		paramIndex++
	}

	// OFFSET clause
	if qb.offset != nil {
		query.WriteString(fmt.Sprintf(" OFFSET $%d", paramIndex))
		params = append(params, *qb.offset)
		paramIndex++
	}

	return query.String(), params, nil
}

func (qb *QueryBuilder) buildInsert() (string, []interface{}, error) {
	if qb.table == "" {
		return "", nil, fmt.Errorf("table name is required for INSERT query")
	}

	var query strings.Builder
	var params []interface{}
	paramIndex := 1

	query.WriteString("INSERT INTO ")
	query.WriteString(qb.table)

	if len(qb.setValues) > 0 {
		// Column-value pairs
		columns := make([]string, 0, len(qb.setValues))
		placeholders := make([]string, 0, len(qb.setValues))

		for column, value := range qb.setValues {
			columns = append(columns, column)
			placeholders = append(placeholders, fmt.Sprintf("$%d", paramIndex))
			params = append(params, value)
			paramIndex++
		}

		query.WriteString(" (")
		query.WriteString(strings.Join(columns, ", "))
		query.WriteString(") VALUES (")
		query.WriteString(strings.Join(placeholders, ", "))
		query.WriteString(")")
	} else if len(qb.values) > 0 {
		// Direct values (requires columns to be set separately)
		placeholders := make([]string, len(qb.values))
		for i := range qb.values {
			placeholders[i] = fmt.Sprintf("$%d", paramIndex)
			paramIndex++
		}
		params = qb.values

		query.WriteString(" VALUES (")
		query.WriteString(strings.Join(placeholders, ", "))
		query.WriteString(")")
	} else {
		return "", nil, fmt.Errorf("no values specified for INSERT query")
	}

	// ON CONFLICT clause
	if qb.onConflict != nil {
		query.WriteString(" ON CONFLICT (")
		query.WriteString(strings.Join(qb.onConflict.Columns, ", "))
		query.WriteString(")")

		switch qb.onConflict.Action {
		case DoNothing:
			query.WriteString(" DO NOTHING")
		case DoUpdate:
			query.WriteString(" DO UPDATE SET ")
			setClauses := make([]string, 0, len(qb.onConflict.Updates))
			for column, value := range qb.onConflict.Updates {
				setClauses = append(setClauses, fmt.Sprintf("%s = $%d", column, paramIndex))
				params = append(params, value)
				paramIndex++
			}
			query.WriteString(strings.Join(setClauses, ", "))
		}
	}

	// RETURNING clause
	if len(qb.returning) > 0 {
		query.WriteString(" RETURNING ")
		query.WriteString(strings.Join(qb.returning, ", "))
	}

	return query.String(), params, nil
}

func (qb *QueryBuilder) buildUpdate() (string, []interface{}, error) {
	if qb.table == "" {
		return "", nil, fmt.Errorf("table name is required for UPDATE query")
	}
	if len(qb.setValues) == 0 {
		return "", nil, fmt.Errorf("no SET values specified for UPDATE query")
	}

	var query strings.Builder
	var params []interface{}
	paramIndex := 1

	query.WriteString("UPDATE ")
	query.WriteString(qb.table)
	query.WriteString(" SET ")

	// SET clause
	setClauses := make([]string, 0, len(qb.setValues))
	for column, value := range qb.setValues {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", column, paramIndex))
		params = append(params, value)
		paramIndex++
	}
	query.WriteString(strings.Join(setClauses, ", "))

	// WHERE clause
	whereClause, whereParams, newIndex := qb.buildWhereClause(paramIndex)
	if whereClause != "" {
		query.WriteString(" WHERE ")
		query.WriteString(whereClause)
		params = append(params, whereParams...)
		paramIndex = newIndex
	}

	// RETURNING clause
	if len(qb.returning) > 0 {
		query.WriteString(" RETURNING ")
		query.WriteString(strings.Join(qb.returning, ", "))
	}

	return query.String(), params, nil
}

func (qb *QueryBuilder) buildDelete() (string, []interface{}, error) {
	if qb.table == "" {
		return "", nil, fmt.Errorf("table name is required for DELETE query")
	}

	var query strings.Builder
	var params []interface{}
	paramIndex := 1

	query.WriteString("DELETE FROM ")
	query.WriteString(qb.table)

	// WHERE clause
	whereClause, whereParams, newIndex := qb.buildWhereClause(paramIndex)
	if whereClause != "" {
		query.WriteString(" WHERE ")
		query.WriteString(whereClause)
		params = append(params, whereParams...)
		paramIndex = newIndex
	}

	// RETURNING clause
	if len(qb.returning) > 0 {
		query.WriteString(" RETURNING ")
		query.WriteString(strings.Join(qb.returning, ", "))
	}

	return query.String(), params, nil
}

func (qb *QueryBuilder) buildWhereClause(startIndex int) (string, []interface{}, int) {
	return qb.buildConditions(qb.conditions, startIndex)
}

func (qb *QueryBuilder) buildConditions(conditions []Condition, startIndex int) (string, []interface{}, int) {
	if len(conditions) == 0 {
		return "", nil, startIndex
	}

	var parts []string
	var params []interface{}
	paramIndex := startIndex

	for i, condition := range conditions {
		var part string
		var conditionParams []interface{}

		// Add logical operator (except for first condition)
		if i > 0 {
			if condition.Logical == Or {
				part = "OR "
			} else {
				part = "AND "
			}
		}

		// Build condition
		switch condition.Operator {
		case Equal:
			part += fmt.Sprintf("%s = $%d", condition.Column, paramIndex)
			conditionParams = append(conditionParams, condition.Value)
			paramIndex++
		case NotEqual:
			part += fmt.Sprintf("%s != $%d", condition.Column, paramIndex)
			conditionParams = append(conditionParams, condition.Value)
			paramIndex++
		case GreaterThan:
			part += fmt.Sprintf("%s > $%d", condition.Column, paramIndex)
			conditionParams = append(conditionParams, condition.Value)
			paramIndex++
		case GreaterThanOrEqual:
			part += fmt.Sprintf("%s >= $%d", condition.Column, paramIndex)
			conditionParams = append(conditionParams, condition.Value)
			paramIndex++
		case LessThan:
			part += fmt.Sprintf("%s < $%d", condition.Column, paramIndex)
			conditionParams = append(conditionParams, condition.Value)
			paramIndex++
		case LessThanOrEqual:
			part += fmt.Sprintf("%s <= $%d", condition.Column, paramIndex)
			conditionParams = append(conditionParams, condition.Value)
			paramIndex++
		case Like:
			part += fmt.Sprintf("%s LIKE $%d", condition.Column, paramIndex)
			conditionParams = append(conditionParams, condition.Value)
			paramIndex++
		case ILike:
			part += fmt.Sprintf("%s ILIKE $%d", condition.Column, paramIndex)
			conditionParams = append(conditionParams, condition.Value)
			paramIndex++
		case In:
			if values, ok := condition.Value.([]interface{}); ok {
				placeholders := make([]string, len(values))
				for j, value := range values {
					placeholders[j] = fmt.Sprintf("$%d", paramIndex)
					conditionParams = append(conditionParams, value)
					paramIndex++
				}
				part += fmt.Sprintf("%s IN (%s)", condition.Column, strings.Join(placeholders, ", "))
			}
		case NotIn:
			if values, ok := condition.Value.([]interface{}); ok {
				placeholders := make([]string, len(values))
				for j, value := range values {
					placeholders[j] = fmt.Sprintf("$%d", paramIndex)
					conditionParams = append(conditionParams, value)
					paramIndex++
				}
				part += fmt.Sprintf("%s NOT IN (%s)", condition.Column, strings.Join(placeholders, ", "))
			}
		case IsNull:
			part += fmt.Sprintf("%s IS NULL", condition.Column)
		case IsNotNull:
			part += fmt.Sprintf("%s IS NOT NULL", condition.Column)
		case Between:
			if values, ok := condition.Value.([]interface{}); ok && len(values) == 2 {
				part += fmt.Sprintf("%s BETWEEN $%d AND $%d", condition.Column, paramIndex, paramIndex+1)
				conditionParams = append(conditionParams, values[0], values[1])
				paramIndex += 2
			}
		case NotBetween:
			if values, ok := condition.Value.([]interface{}); ok && len(values) == 2 {
				part += fmt.Sprintf("%s NOT BETWEEN $%d AND $%d", condition.Column, paramIndex, paramIndex+1)
				conditionParams = append(conditionParams, values[0], values[1])
				paramIndex += 2
			}
		}

		parts = append(parts, part)
		params = append(params, conditionParams...)
	}

	return strings.Join(parts, " "), params, paramIndex
}

func joinTypeToString(joinType JoinType) string {
	switch joinType {
	case InnerJoin:
		return "INNER JOIN"
	case LeftJoin:
		return "LEFT JOIN"
	case RightJoin:
		return "RIGHT JOIN"
	case FullJoin:
		return "FULL OUTER JOIN"
	default:
		return "INNER JOIN"
	}
}

// Type-safe helper methods for common patterns

// WhereID is a convenience method for filtering by UUID ID
func (qb *QueryBuilder) WhereID(id uuid.UUID) *QueryBuilder {
	return qb.WhereEqual("id", id)
}

// WhereStatus is a convenience method for filtering by status
func (qb *QueryBuilder) WhereStatus(status string) *QueryBuilder {
	return qb.WhereEqual("status", status)
}

// WhereActive filters for active records
func (qb *QueryBuilder) WhereActive() *QueryBuilder {
	return qb.WhereEqual("status", "active")
}

// WhereCreatedAfter filters records created after a specific time
func (qb *QueryBuilder) WhereCreatedAfter(t time.Time) *QueryBuilder {
	return qb.Where("created_at", GreaterThan, t)
}

// WhereUpdatedBefore filters records updated before a specific time
func (qb *QueryBuilder) WhereUpdatedBefore(t time.Time) *QueryBuilder {
	return qb.Where("updated_at", LessThan, t)
}

// SetUpdatedAt sets the updated_at field to current time
func (qb *QueryBuilder) SetUpdatedAt(t time.Time) *QueryBuilder {
	return qb.Set("updated_at", t)
}
