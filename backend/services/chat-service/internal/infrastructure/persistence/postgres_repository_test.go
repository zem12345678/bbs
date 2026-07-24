package persistence

import (
	"context"
	"database/sql"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestLockRoomThenMemberForUpdateUsesSharedLockOrder(t *testing.T) {
	now := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	query := &scriptedRowQuerier{rows: []pgx.Row{
		scriptedRow{values: []any{
			int64(8), "AB12CD3E", "room", int64(42), "", int64(0), int64(7), int16(1), now, now,
		}},
		scriptedRow{values: []any{
			int64(8), int64(42), int16(1), int16(1), int64(0), int64(7), int64(0),
			sql.NullInt64{}, int32(0), now, sql.NullTime{}, now, now,
		}},
	}}

	room, member, found, err := lockRoomThenMemberForUpdate(context.Background(), query, "AB12CD3E", 42)
	if err != nil {
		t.Fatal(err)
	}
	if !found || room.ID != 8 || member.UserID != 42 {
		t.Fatalf("locked values = room %#v, member %#v, found %t", room, member, found)
	}
	if len(query.calls) != 2 {
		t.Fatalf("query calls = %d, want 2", len(query.calls))
	}
	if !strings.Contains(query.calls[0].sql, "FROM chat_rooms") || !strings.Contains(query.calls[0].sql, "FOR UPDATE") {
		t.Fatalf("first lock query = %q, want room FOR UPDATE", query.calls[0].sql)
	}
	if !strings.Contains(query.calls[1].sql, "FROM chat_room_members") || !strings.Contains(query.calls[1].sql, "FOR UPDATE") {
		t.Fatalf("second lock query = %q, want membership FOR UPDATE", query.calls[1].sql)
	}
	if !reflect.DeepEqual(query.calls[0].args, []any{"AB12CD3E"}) || !reflect.DeepEqual(query.calls[1].args, []any{int64(8), int64(42)}) {
		t.Fatalf("lock query args = %#v", query.calls)
	}
}

type scriptedRowQuerier struct {
	rows  []pgx.Row
	calls []scriptedQueryCall
}

type scriptedQueryCall struct {
	sql  string
	args []any
}

func (q *scriptedRowQuerier) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	q.calls = append(q.calls, scriptedQueryCall{sql: query, args: append([]any(nil), args...)})
	row := q.rows[0]
	q.rows = q.rows[1:]
	return row
}

type scriptedRow struct {
	values []any
	err    error
}

func (r scriptedRow) Scan(destinations ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(destinations) != len(r.values) {
		return pgx.ErrNoRows
	}
	for index, destination := range destinations {
		reflect.ValueOf(destination).Elem().Set(reflect.ValueOf(r.values[index]))
	}
	return nil
}
