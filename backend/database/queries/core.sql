-- name: GetUserByEmail :one
SELECT * FROM users WHERE lower(email) = lower($1) AND active = true;
-- name: ListCustomers :many
SELECT * FROM customers WHERE active=true AND ($1::text='' OR name ILIKE '%'||$1||'%' OR phone ILIKE '%'||$1||'%' OR external_id ILIKE '%'||$1||'%') ORDER BY name LIMIT $2 OFFSET $3;
-- name: ListConversations :many
SELECT c.*, cu.name customer_name, cu.phone, u.name agent_name FROM conversations c JOIN customers cu ON cu.id=c.customer_id JOIN users u ON u.id=c.assigned_user_id ORDER BY c.updated_at DESC;
-- name: ListMessages :many
SELECT * FROM messages WHERE conversation_id=$1 ORDER BY created_at;
