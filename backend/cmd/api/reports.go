package main

import (
	"net/http"
	"strconv"
)

func (a *app) reports(w http.ResponseWriter, r *http.Request) {
	if ident(r).Role == "AGENT" {
		fail(w, http.StatusForbidden, "acesso restrito a supervisores e administradores")
		return
	}
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	if days != 7 && days != 30 && days != 90 {
		days = 30
	}

	summary, err := queryMaps(r.Context(), a.db, `
		SELECT
			count(*) FILTER (WHERE started_at::date >= current_date-($1::int-1)) conversations_started,
			count(*) FILTER (WHERE closed_at::date >= current_date-($1::int-1)) conversations_closed,
			count(*) FILTER (WHERE closed_at::date >= current_date-($1::int-1) AND result='SALE') sales,
			count(*) FILTER (WHERE status<>'CLOSED') open_conversations,
			COALESCE(round(100.0 * count(*) FILTER (WHERE closed_at::date >= current_date-($1::int-1) AND result='SALE') / NULLIF(count(*) FILTER (WHERE closed_at::date >= current_date-($1::int-1)),0),1),0)::float8 conversion_rate,
			COALESCE(round(avg(EXTRACT(EPOCH FROM (closed_at-started_at))/60) FILTER (WHERE closed_at::date >= current_date-($1::int-1)),0),0)::float8 average_service_minutes
		FROM conversations`, days)
	if err != nil || len(summary) == 0 {
		fail(w, http.StatusInternalServerError, "não foi possível gerar o relatório")
		return
	}

	daily, err := queryMaps(r.Context(), a.db, `
		WITH dates AS (
			SELECT generate_series(current_date-($1::int-1),current_date,interval '1 day')::date AS report_day
		)
		SELECT d.report_day AS day,
			count(c.id) FILTER (WHERE c.started_at::date=d.report_day) started,
			count(c.id) FILTER (WHERE c.closed_at::date=d.report_day) closed,
			count(c.id) FILTER (WHERE c.closed_at::date=d.report_day AND c.result='SALE') sales
		FROM dates d LEFT JOIN conversations c
			ON c.started_at::date=d.report_day OR c.closed_at::date=d.report_day
		GROUP BY d.report_day ORDER BY d.report_day`, days)
	if err != nil {
		fail(w, http.StatusInternalServerError, "não foi possível gerar a evolução diária")
		return
	}

	agents, err := queryMaps(r.Context(), a.db, `
		SELECT u.id,u.name,
			count(c.id) FILTER (WHERE c.started_at::date >= current_date-($1::int-1)) contacts,
			count(c.id) FILTER (WHERE c.closed_at::date >= current_date-($1::int-1)) closed,
			count(c.id) FILTER (WHERE c.closed_at::date >= current_date-($1::int-1) AND c.result='SALE') sales,
			count(c.id) FILTER (WHERE c.status<>'CLOSED') open_now,
			COALESCE(round(100.0 * count(c.id) FILTER (WHERE c.closed_at::date >= current_date-($1::int-1) AND c.result='SALE') / NULLIF(count(c.id) FILTER (WHERE c.closed_at::date >= current_date-($1::int-1)),0),1),0)::float8 conversion_rate
		FROM users u LEFT JOIN conversations c ON c.assigned_user_id=u.id
		WHERE u.active AND u.role IN ('AGENT','SUPERVISOR')
		GROUP BY u.id,u.name
		ORDER BY sales DESC,contacts DESC,u.name`, days)
	if err != nil {
		fail(w, http.StatusInternalServerError, "não foi possível gerar o desempenho da equipe")
		return
	}

	results, err := queryMaps(r.Context(), a.db, `
		SELECT COALESCE(NULLIF(result,''),'OTHER') result,count(*) total
		FROM conversations
		WHERE closed_at::date >= current_date-($1::int-1)
		GROUP BY COALESCE(NULLIF(result,''),'OTHER') ORDER BY total DESC`, days)
	if err != nil {
		fail(w, http.StatusInternalServerError, "não foi possível gerar os resultados")
		return
	}

	var inbound, outbound int
	_ = a.db.QueryRow(r.Context(), `SELECT
		count(*) FILTER (WHERE sender_type='CUSTOMER'),
		count(*) FILTER (WHERE sender_type='AGENT')
		FROM messages WHERE created_at::date >= current_date-($1::int-1)`, days).Scan(&inbound, &outbound)
	summary[0]["inbound_messages"] = inbound
	summary[0]["outbound_messages"] = outbound

	write(w, http.StatusOK, map[string]any{
		"days":    days,
		"summary": summary[0],
		"daily":   daily,
		"agents":  agents,
		"results": results,
	})
}
