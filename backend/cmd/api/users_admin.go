package main

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

var validUserRoles = map[string]bool{"ADMIN": true, "SUPERVISOR": true, "AGENT": true}

func (a *app) createUser(w http.ResponseWriter, r *http.Request) {
	actor := ident(r)
	if actor.Role != "ADMIN" {
		fail(w, http.StatusForbidden, "acesso restrito a administradores")
		return
	}
	var in struct {
		Name     string   `json:"name"`
		Email    string   `json:"email"`
		Password string   `json:"password"`
		Role     string   `json:"role"`
		GroupIDs []string `json:"group_ids"`
	}
	if decode(r, &in) != nil {
		fail(w, http.StatusBadRequest, "dados inválidos")
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.Role = strings.ToUpper(strings.TrimSpace(in.Role))
	if in.Name == "" || in.Email == "" || !strings.Contains(in.Email, "@") || !validUserRoles[in.Role] {
		fail(w, http.StatusBadRequest, "nome, e-mail e perfil válidos são obrigatórios")
		return
	}
	if len(in.Password) < 8 {
		fail(w, http.StatusBadRequest, "a senha deve ter pelo menos 8 caracteres")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		fail(w, http.StatusInternalServerError, "não foi possível proteger a senha")
		return
	}
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		fail(w, 500, "não foi possível criar o usuário")
		return
	}
	defer tx.Rollback(r.Context())
	var id string
	err = tx.QueryRow(r.Context(), `INSERT INTO users(name,email,password_hash,role) VALUES($1,$2,$3,$4::user_role) RETURNING id`, in.Name, in.Email, string(hash), in.Role).Scan(&id)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			fail(w, http.StatusConflict, "já existe um usuário com este e-mail")
			return
		}
		fail(w, http.StatusInternalServerError, "não foi possível criar o usuário")
		return
	}
	if err = replaceUserGroups(r.Context(), tx, id, in.GroupIDs); err != nil {
		fail(w, 400, "um ou mais grupos são inválidos")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		fail(w, 500, "não foi possível salvar o usuário")
		return
	}
	a.audit(r, actor.ID, "USER_CREATED", "user", id, map[string]any{"role": in.Role})
	write(w, http.StatusCreated, map[string]any{"id": id, "name": in.Name, "email": in.Email, "role": in.Role, "active": true, "group_ids": in.GroupIDs})
}

func (a *app) updateUser(w http.ResponseWriter, r *http.Request) {
	actor := ident(r)
	if actor.Role != "ADMIN" {
		fail(w, http.StatusForbidden, "acesso restrito a administradores")
		return
	}
	id := chi.URLParam(r, "id")
	var in struct {
		Name     *string   `json:"name"`
		Role     *string   `json:"role"`
		Active   *bool     `json:"active"`
		Password *string   `json:"password"`
		GroupIDs *[]string `json:"group_ids"`
	}
	if decode(r, &in) != nil {
		fail(w, http.StatusBadRequest, "dados inválidos")
		return
	}
	if id == actor.ID && in.Active != nil && !*in.Active {
		fail(w, http.StatusBadRequest, "você não pode desativar seu próprio acesso")
		return
	}
	if in.Name != nil {
		trimmed := strings.TrimSpace(*in.Name)
		if trimmed == "" {
			fail(w, http.StatusBadRequest, "nome não pode ficar vazio")
			return
		}
		in.Name = &trimmed
	}
	if in.Role != nil {
		role := strings.ToUpper(strings.TrimSpace(*in.Role))
		if !validUserRoles[role] {
			fail(w, http.StatusBadRequest, "perfil inválido")
			return
		}
		in.Role = &role
	}
	var passwordHash *string
	if in.Password != nil && *in.Password != "" {
		if len(*in.Password) < 8 {
			fail(w, http.StatusBadRequest, "a senha deve ter pelo menos 8 caracteres")
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(*in.Password), bcrypt.DefaultCost)
		if err != nil {
			fail(w, http.StatusInternalServerError, "não foi possível proteger a senha")
			return
		}
		encoded := string(hash)
		passwordHash = &encoded
	}
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		fail(w, 500, "não foi possível atualizar o usuário")
		return
	}
	defer tx.Rollback(r.Context())
	var item map[string]any
	rows, err := queryMaps(r.Context(), tx, `UPDATE users SET name=COALESCE($2,name),role=COALESCE($3::user_role,role),active=COALESCE($4,active),password_hash=COALESCE($5,password_hash),updated_at=now() WHERE id=$1 RETURNING id,name,email,role::text,active`, id, in.Name, in.Role, in.Active, passwordHash)
	if err != nil && err != pgx.ErrNoRows {
		fail(w, http.StatusInternalServerError, "não foi possível atualizar o usuário")
		return
	}
	if len(rows) == 0 {
		fail(w, http.StatusNotFound, "usuário não encontrado")
		return
	}
	if in.GroupIDs != nil {
		if err = replaceUserGroups(r.Context(), tx, id, *in.GroupIDs); err != nil {
			fail(w, 400, "um ou mais grupos são inválidos")
			return
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		fail(w, 500, "não foi possível salvar o usuário")
		return
	}
	item = rows[0]
	a.audit(r, actor.ID, "USER_UPDATED", "user", id, map[string]any{"active": in.Active, "role": in.Role})
	write(w, http.StatusOK, item)
}

func replaceUserGroups(ctx context.Context, tx pgx.Tx, userID string, groupIDs []string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM user_group_members WHERE user_id=$1`, userID); err != nil {
		return err
	}
	if len(groupIDs) == 0 {
		return nil
	}
	tag, err := tx.Exec(ctx, `INSERT INTO user_group_members(group_id,user_id) SELECT id,$1 FROM user_groups WHERE active AND id=ANY($2::uuid[])`, userID, groupIDs)
	if err != nil {
		return err
	}
	if int(tag.RowsAffected()) != len(groupIDs) {
		return pgx.ErrNoRows
	}
	return nil
}
