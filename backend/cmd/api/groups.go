package main

import (
	"context"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

var groupColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func (a *app) listGroups(w http.ResponseWriter, r *http.Request) {
	rows, err := queryMaps(r.Context(), a.db, `
		SELECT g.id,g.name,g.description,g.color,g.active,g.created_at,g.updated_at,
		COALESCE(jsonb_agg(m.user_id::text ORDER BY u.name) FILTER (WHERE m.user_id IS NOT NULL),'[]'::jsonb) user_ids,
		count(m.user_id) member_count
		FROM user_groups g
		LEFT JOIN user_group_members m ON m.group_id=g.id
		LEFT JOIN users u ON u.id=m.user_id
		GROUP BY g.id ORDER BY g.active DESC,g.name`)
	if err != nil {
		fail(w, http.StatusInternalServerError, "não foi possível carregar os grupos")
		return
	}
	write(w, http.StatusOK, map[string]any{"items": rows})
}

func (a *app) createGroup(w http.ResponseWriter, r *http.Request) {
	actor := ident(r)
	if actor.Role != "ADMIN" {
		fail(w, http.StatusForbidden, "acesso restrito a administradores")
		return
	}
	var in struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Color       string   `json:"color"`
		UserIDs     []string `json:"user_ids"`
	}
	if decode(r, &in) != nil {
		fail(w, http.StatusBadRequest, "dados inválidos")
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	if in.Color == "" {
		in.Color = "#15a76e"
	}
	if in.Name == "" || len(in.Name) > 80 || !groupColorPattern.MatchString(in.Color) {
		fail(w, http.StatusBadRequest, "nome e cor válidos são obrigatórios")
		return
	}
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		fail(w, 500, "não foi possível criar o grupo")
		return
	}
	defer tx.Rollback(r.Context())
	var id string
	err = tx.QueryRow(r.Context(), `INSERT INTO user_groups(name,description,color) VALUES($1,$2,$3) RETURNING id`, in.Name, in.Description, strings.ToLower(in.Color)).Scan(&id)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			fail(w, 409, "já existe um grupo com este nome")
			return
		}
		fail(w, 500, "não foi possível criar o grupo")
		return
	}
	if err = replaceGroupMembers(r.Context(), tx, id, in.UserIDs); err != nil {
		fail(w, 400, "um ou mais usuários são inválidos")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		fail(w, 500, "não foi possível salvar o grupo")
		return
	}
	a.audit(r, actor.ID, "GROUP_CREATED", "user_group", id, map[string]any{"name": in.Name, "members": len(in.UserIDs)})
	write(w, http.StatusCreated, map[string]any{"id": id, "name": in.Name, "description": in.Description, "color": strings.ToLower(in.Color), "active": true, "user_ids": in.UserIDs})
}

func (a *app) updateGroup(w http.ResponseWriter, r *http.Request) {
	actor := ident(r)
	if actor.Role != "ADMIN" {
		fail(w, 403, "acesso restrito a administradores")
		return
	}
	id := chi.URLParam(r, "id")
	var in struct {
		Name        *string   `json:"name"`
		Description *string   `json:"description"`
		Color       *string   `json:"color"`
		Active      *bool     `json:"active"`
		UserIDs     *[]string `json:"user_ids"`
	}
	if decode(r, &in) != nil {
		fail(w, 400, "dados inválidos")
		return
	}
	if in.Name != nil {
		value := strings.TrimSpace(*in.Name)
		if value == "" || len(value) > 80 {
			fail(w, 400, "nome inválido")
			return
		}
		in.Name = &value
	}
	if in.Description != nil {
		value := strings.TrimSpace(*in.Description)
		in.Description = &value
	}
	if in.Color != nil {
		value := strings.ToLower(*in.Color)
		if !groupColorPattern.MatchString(value) {
			fail(w, 400, "cor inválida")
			return
		}
		in.Color = &value
	}
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		fail(w, 500, "não foi possível atualizar o grupo")
		return
	}
	defer tx.Rollback(r.Context())
	var exists bool
	err = tx.QueryRow(r.Context(), `UPDATE user_groups SET name=COALESCE($2,name),description=COALESCE($3,description),color=COALESCE($4,color),active=COALESCE($5,active),updated_at=now() WHERE id=$1 RETURNING true`, id, in.Name, in.Description, in.Color, in.Active).Scan(&exists)
	if err == pgx.ErrNoRows {
		fail(w, 404, "grupo não encontrado")
		return
	}
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			fail(w, 409, "já existe um grupo com este nome")
			return
		}
		fail(w, 500, "não foi possível atualizar o grupo")
		return
	}
	if in.UserIDs != nil {
		if err = replaceGroupMembers(r.Context(), tx, id, *in.UserIDs); err != nil {
			fail(w, 400, "um ou mais usuários são inválidos")
			return
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		fail(w, 500, "não foi possível salvar o grupo")
		return
	}
	a.audit(r, actor.ID, "GROUP_UPDATED", "user_group", id, map[string]any{"active": in.Active})
	w.WriteHeader(http.StatusNoContent)
}

func replaceGroupMembers(ctx context.Context, tx pgx.Tx, groupID string, userIDs []string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM user_group_members WHERE group_id=$1`, groupID); err != nil {
		return err
	}
	if len(userIDs) == 0 {
		return nil
	}
	tag, err := tx.Exec(ctx, `INSERT INTO user_group_members(group_id,user_id) SELECT $1,id FROM users WHERE id=ANY($2::uuid[])`, groupID, userIDs)
	if err != nil {
		return err
	}
	if int(tag.RowsAffected()) != len(userIDs) {
		return pgx.ErrNoRows
	}
	return nil
}
