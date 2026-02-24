package models

import (
	"database/sql"
	"time"
)

type User struct {
	ID           uint64     `json:"id"`
	Email        string     `json:"email"`
	Name         string     `json:"name"`
	PasswordHash string     `json:"-"`
	Role         string     `json:"role"`
	Status       string     `json:"status"`
	LastLogin    *time.Time `json:"lastLogin"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

func CreateUser(db *sql.DB, email, name, passwordHash, role string) (*User, error) {
	res, err := db.Exec(
		"INSERT INTO users (email, name, password_hash, role) VALUES (?, ?, ?, ?)",
		email, name, passwordHash, role,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return GetUserByID(db, uint64(id))
}

func GetUserByEmail(db *sql.DB, email string) (*User, error) {
	u := &User{}
	err := db.QueryRow(
		"SELECT id, email, name, password_hash, role, status, last_login, created_at, updated_at FROM users WHERE email = ?",
		email,
	).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.Role, &u.Status, &u.LastLogin, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func GetUserByID(db *sql.DB, id uint64) (*User, error) {
	u := &User{}
	err := db.QueryRow(
		"SELECT id, email, name, password_hash, role, status, last_login, created_at, updated_at FROM users WHERE id = ?",
		id,
	).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.Role, &u.Status, &u.LastLogin, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func CountUsers(db *sql.DB) (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}

func UpdateLastLogin(db *sql.DB, id uint64) error {
	_, err := db.Exec("UPDATE users SET last_login = NOW() WHERE id = ?", id)
	return err
}

func ListUsers(db *sql.DB) ([]User, error) {
	rows, err := db.Query(
		"SELECT id, email, name, password_hash, role, status, last_login, created_at, updated_at FROM users ORDER BY created_at ASC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.Role, &u.Status, &u.LastLogin, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func UpdateUserRole(db *sql.DB, id uint64, role string) error {
	res, err := db.Exec("UPDATE users SET role = ? WHERE id = ?", role, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func UpdateUserStatus(db *sql.DB, id uint64, status string) error {
	res, err := db.Exec("UPDATE users SET status = ? WHERE id = ?", status, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func UpdateUserPassword(db *sql.DB, id uint64, passwordHash string) error {
	res, err := db.Exec("UPDATE users SET password_hash = ? WHERE id = ?", passwordHash, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
