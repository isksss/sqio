package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// RoleInfo describes a database role or user.
type RoleInfo struct {
	Name       string `json:"name"`
	Host       string `json:"host,omitempty"`
	Login      bool   `json:"login"`
	Superuser  bool   `json:"superuser,omitempty"`
	CreateRole bool   `json:"create_role,omitempty"`
	CreateDB   bool   `json:"create_db,omitempty"`
}

// GrantInfo describes a database privilege grant.
type GrantInfo struct {
	Grantee   string `json:"grantee"`
	Object    string `json:"object"`
	Privilege string `json:"privilege"`
	Grantable bool   `json:"grantable"`
	Raw       string `json:"raw,omitempty"`
}

// Roles returns role/user metadata for the configured database.
func Roles(ctx context.Context, cfg Config) ([]RoleInfo, error) {
	conn, driver, err := openConnection(ctx, cfg)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	switch driver {
	case "sqlite":
		return []RoleInfo{}, nil
	case "pgx":
		return postgresRoles(ctx, conn)
	case "mysql":
		return mysqlRoles(ctx, conn)
	default:
		return nil, fmt.Errorf("unsupported driver: %s", driver)
	}
}

// Grants returns privilege grants. Empty role means current user where the
// driver supports that directly.
func Grants(ctx context.Context, cfg Config, role string) ([]GrantInfo, error) {
	conn, driver, err := openConnection(ctx, cfg)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	switch driver {
	case "sqlite":
		return []GrantInfo{}, nil
	case "pgx":
		return postgresGrants(ctx, conn, role)
	case "mysql":
		if strings.TrimSpace(role) != "" {
			return nil, fmt.Errorf("mysql grants supports current user only")
		}
		return mysqlGrants(ctx, conn)
	default:
		return nil, fmt.Errorf("unsupported driver: %s", driver)
	}
}

func postgresRoles(ctx context.Context, conn *sql.DB) ([]RoleInfo, error) {
	rows, err := conn.QueryContext(ctx, `
select rolname, rolcanlogin, rolsuper, rolcreaterole, rolcreatedb
from pg_catalog.pg_roles
order by rolname`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	roles := []RoleInfo{}
	for rows.Next() {
		var role RoleInfo
		if err := rows.Scan(&role.Name, &role.Login, &role.Superuser, &role.CreateRole, &role.CreateDB); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func postgresGrants(ctx context.Context, conn *sql.DB, role string) ([]GrantInfo, error) {
	query := `
select grantee, table_schema || '.' || table_name, privilege_type, is_grantable = 'YES'
from information_schema.role_table_grants`
	args := []interface{}{}
	if role != "" {
		query += ` where grantee = $1`
		args = append(args, role)
	}
	query += ` order by grantee, table_schema, table_name, privilege_type`
	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	grants := []GrantInfo{}
	for rows.Next() {
		var grant GrantInfo
		if err := rows.Scan(&grant.Grantee, &grant.Object, &grant.Privilege, &grant.Grantable); err != nil {
			return nil, err
		}
		grants = append(grants, grant)
	}
	return grants, rows.Err()
}

func mysqlRoles(ctx context.Context, conn *sql.DB) ([]RoleInfo, error) {
	var currentUser string
	if err := conn.QueryRowContext(ctx, `select current_user()`).Scan(&currentUser); err != nil {
		return nil, err
	}
	name, host := splitMySQLAccount(currentUser)
	return []RoleInfo{{Name: name, Host: host, Login: true}}, nil
}

func mysqlGrants(ctx context.Context, conn *sql.DB) ([]GrantInfo, error) {
	rows, err := conn.QueryContext(ctx, `show grants`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	grants := []GrantInfo{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		grants = append(grants, GrantInfo{Raw: raw})
	}
	return grants, rows.Err()
}

func splitMySQLAccount(account string) (string, string) {
	parts := strings.Split(account, "@")
	if len(parts) < 2 {
		return strings.Trim(account, "'`\""), ""
	}
	return strings.Trim(parts[0], "'`\""), strings.Trim(parts[1], "'`\"")
}
