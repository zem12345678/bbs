package persistence

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	domain "chat-service/internal/domain/chat"

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

func TestMoveGroupInOrder(t *testing.T) {
	groups := []domain.Group{{ID: 10}, {ID: 20}, {ID: 30}}
	moved, err := moveGroupInOrder(groups, 20, -1)
	if err != nil {
		t.Fatal(err)
	}
	if !moved || groups[0].ID != 20 || groups[1].ID != 10 || groups[2].ID != 30 {
		t.Fatalf("moved groups = %#v", groups)
	}

	moved, err = moveGroupInOrder(groups, 20, -1)
	if err != nil {
		t.Fatal(err)
	}
	if moved || groups[0].ID != 20 {
		t.Fatalf("boundary move = moved %t, groups %#v", moved, groups)
	}

	_, err = moveGroupInOrder(groups, 99, 1)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing group error = %v, want not found", err)
	}
	_, err = moveGroupInOrder(groups, 20, 0)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("invalid direction error = %v, want invalid input", err)
	}
}

func TestPlaceRoomInOrder(t *testing.T) {
	t.Run("reorders within a group", func(t *testing.T) {
		members := []roomPlacementMember{
			{RoomNo: "A", Membership: domain.Membership{RoomID: 1, GroupID: 10, SortOrder: 0}},
			{RoomNo: "B", Membership: domain.Membership{RoomID: 2, GroupID: 10, SortOrder: 1}},
			{RoomNo: "C", Membership: domain.Membership{RoomID: 3, GroupID: 10, SortOrder: 2}},
		}
		updates, err := placeRoomInOrder(members, "C", domain.Placement{GroupID: 10, SortOrder: 0})
		if err != nil {
			t.Fatal(err)
		}
		assertRoomPlacementOrder(t, updates, []domain.Membership{
			{RoomID: 3, GroupID: 10, SortOrder: 0},
			{RoomID: 1, GroupID: 10, SortOrder: 1},
			{RoomID: 2, GroupID: 10, SortOrder: 2},
		})
	})

	t.Run("reorders both groups after a move", func(t *testing.T) {
		members := []roomPlacementMember{
			{RoomNo: "A", Membership: domain.Membership{RoomID: 1, GroupID: 10, SortOrder: 0}},
			{RoomNo: "B", Membership: domain.Membership{RoomID: 2, GroupID: 10, SortOrder: 1}},
			{RoomNo: "C", Membership: domain.Membership{RoomID: 3, GroupID: 20, SortOrder: 0}},
			{RoomNo: "D", Membership: domain.Membership{RoomID: 4, GroupID: 20, SortOrder: 1}},
		}
		updates, err := placeRoomInOrder(members, "B", domain.Placement{GroupID: 20, SortOrder: 1})
		if err != nil {
			t.Fatal(err)
		}
		assertRoomPlacementOrder(t, updates, []domain.Membership{
			{RoomID: 1, GroupID: 10, SortOrder: 0},
			{RoomID: 3, GroupID: 20, SortOrder: 0},
			{RoomID: 2, GroupID: 20, SortOrder: 1},
			{RoomID: 4, GroupID: 20, SortOrder: 2},
		})
	})

	t.Run("clamps past the end", func(t *testing.T) {
		members := []roomPlacementMember{
			{RoomNo: "A", Membership: domain.Membership{RoomID: 1, GroupID: 10, SortOrder: 0}},
			{RoomNo: "B", Membership: domain.Membership{RoomID: 2, GroupID: 10, SortOrder: 1}},
		}
		updates, err := placeRoomInOrder(members, "A", domain.Placement{GroupID: 10, SortOrder: 99})
		if err != nil {
			t.Fatal(err)
		}
		assertRoomPlacementOrder(t, updates, []domain.Membership{
			{RoomID: 2, GroupID: 10, SortOrder: 0},
			{RoomID: 1, GroupID: 10, SortOrder: 1},
		})
	})

	members := []roomPlacementMember{{RoomNo: "A", Membership: domain.Membership{RoomID: 1}}}
	if _, err := placeRoomInOrder(members, "missing", domain.Placement{}); !errors.Is(err, domain.ErrNotMember) {
		t.Fatalf("missing room error = %v, want not member", err)
	}
	if _, err := placeRoomInOrder(members, "A", domain.Placement{GroupID: -1}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("negative group id error = %v, want invalid input", err)
	}
	if _, err := placeRoomInOrder(members, "A", domain.Placement{SortOrder: -1}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("negative sort order error = %v, want invalid input", err)
	}
}

func TestReleaseGroupRoomPlacements(t *testing.T) {
	members := []roomPlacementMember{
		{RoomNo: "A", Membership: domain.Membership{RoomID: 1, SortOrder: 0}},
		{RoomNo: "B", Membership: domain.Membership{RoomID: 2, SortOrder: 2}},
		{RoomNo: "C", Membership: domain.Membership{RoomID: 3, GroupID: 10, SortOrder: 0}},
		{RoomNo: "D", Membership: domain.Membership{RoomID: 4, GroupID: 10, SortOrder: 1}},
		{RoomNo: "E", Membership: domain.Membership{RoomID: 5, GroupID: 20, SortOrder: 0}},
	}

	assertRoomPlacementOrder(t, releaseGroupRoomPlacements(members, 10), []domain.Membership{
		{RoomID: 1, SortOrder: 0},
		{RoomID: 2, SortOrder: 1},
		{RoomID: 3, SortOrder: 2},
		{RoomID: 4, SortOrder: 3},
	})
}

func TestAppendRoomPlacement(t *testing.T) {
	members := []roomPlacementMember{
		{RoomNo: "A", Membership: domain.Membership{RoomID: 1, SortOrder: 2}},
		{RoomNo: "B", Membership: domain.Membership{RoomID: 2, SortOrder: 0}},
	}
	member := domain.Membership{RoomID: 3}

	assertRoomPlacementOrder(t, appendRoomPlacement(members, &member), []domain.Membership{
		{RoomID: 2, SortOrder: 0},
		{RoomID: 1, SortOrder: 1},
		{RoomID: 3, SortOrder: 2},
	})
	if member.SortOrder != 2 {
		t.Fatalf("new member sort order = %d, want 2", member.SortOrder)
	}
}

func TestRemoveRoomPlacement(t *testing.T) {
	members := []roomPlacementMember{
		{RoomNo: "A", Membership: domain.Membership{RoomID: 1, GroupID: 10, SortOrder: 0}},
		{RoomNo: "B", Membership: domain.Membership{RoomID: 2, GroupID: 10, SortOrder: 1}},
		{RoomNo: "C", Membership: domain.Membership{RoomID: 3, GroupID: 10, SortOrder: 2}},
		{RoomNo: "D", Membership: domain.Membership{RoomID: 4, GroupID: 20, SortOrder: 0}},
	}

	assertRoomPlacementOrder(t, removeRoomPlacement(members, 2, 10), []domain.Membership{
		{RoomID: 1, GroupID: 10, SortOrder: 0},
		{RoomID: 3, GroupID: 10, SortOrder: 1},
	})
}

func assertRoomPlacementOrder(t *testing.T, actual, expected []domain.Membership) {
	t.Helper()
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("room placement order = %#v, want %#v", actual, expected)
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
