package grpc

import (
	"context"
	"testing"

	pb "search-service/api/proto/searchpb"
	"search-service/internal/application/search/command"
	"search-service/internal/application/search/query"
	domain "search-service/internal/domain/search"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestHandlerServesUserIndexAndSearchRPCs(t *testing.T) {
	repo := &userHandlerRepository{
		userHits: []domain.UserHit{{
			Document: domain.UserDocument{ID: 42, Username: "alice", Nickname: "Alice", Status: 1, CreatedAt: 1000, UpdatedAt: 2000},
			Score:    3.5,
		}},
		userTotal: 1,
	}
	handler := NewHandler(command.NewService(repo), query.NewService(repo))
	doc := &pb.UserDocument{Id: 42, Username: "alice", Nickname: "Alice", Status: 1, CreatedAt: 1000, UpdatedAt: 2000}

	if _, err := handler.EnsureUserIndex(t.Context(), &pb.EnsureUserIndexRequest{}); err != nil {
		t.Fatalf("EnsureUserIndex() error = %v", err)
	}
	if _, err := handler.IndexUser(t.Context(), &pb.IndexUserRequest{User: doc}); err != nil {
		t.Fatalf("IndexUser() error = %v", err)
	}
	if repo.indexedUser != (domain.UserDocument{ID: 42, Username: "alice", Nickname: "Alice", Status: 1, CreatedAt: 1000, UpdatedAt: 2000}) {
		t.Fatalf("indexed user = %#v", repo.indexedUser)
	}
	if _, err := handler.ReindexUser(t.Context(), &pb.IndexUserRequest{User: doc}); err != nil {
		t.Fatalf("ReindexUser() error = %v", err)
	}
	if _, err := handler.DeleteUser(t.Context(), &pb.DeleteUserRequest{Id: 42}); err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}
	if repo.deletedUserID != 42 {
		t.Fatalf("deleted user ID = %d", repo.deletedUserID)
	}
	erased, err := handler.EraseUserData(t.Context(), &pb.EraseUserDataRequest{UserId: 42, DeletionJobId: 91, PolicyVersion: 3})
	if err != nil || !erased.GetCompleted() {
		t.Fatalf("EraseUserData() response=%+v error=%v", erased, err)
	}
	if repo.erasedUserID != 42 || repo.deletionJobID != 91 || repo.policyVersion != 3 {
		t.Fatalf("erasure request user=%d job=%d policy=%d", repo.erasedUserID, repo.deletionJobID, repo.policyVersion)
	}

	response, err := handler.SearchUsers(t.Context(), &pb.SearchUsersRequest{Keyword: "alcie", Page: 2, PageSize: 10})
	if err != nil {
		t.Fatalf("SearchUsers() error = %v", err)
	}
	if repo.searchKeyword != "alcie" || repo.searchPage != 2 || repo.searchPageSize != 10 {
		t.Fatalf("search request = %q/%d/%d", repo.searchKeyword, repo.searchPage, repo.searchPageSize)
	}
	if response.GetTotal() != 1 || len(response.GetItems()) != 1 || response.GetItems()[0].GetUser().GetId() != 42 || response.GetItems()[0].GetScore() != 3.5 {
		t.Fatalf("search response = %#v", response)
	}
}

func TestHandlerRejectsInvalidUserDocument(t *testing.T) {
	repo := &userHandlerRepository{}
	handler := NewHandler(command.NewService(repo), query.NewService(repo))

	_, err := handler.IndexUser(t.Context(), &pb.IndexUserRequest{User: &pb.UserDocument{}})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("IndexUser() code = %s, want %s; err=%v", status.Code(err), codes.InvalidArgument, err)
	}
	_, err = handler.EraseUserData(t.Context(), &pb.EraseUserDataRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("EraseUserData() code = %s, want %s; err=%v", status.Code(err), codes.InvalidArgument, err)
	}
}

type userHandlerRepository struct {
	domain.Repository
	indexedUser    domain.UserDocument
	reindexedUser  domain.UserDocument
	deletedUserID  int64
	erasedUserID   int64
	deletionJobID  int64
	policyVersion  int32
	userHits       []domain.UserHit
	userTotal      int64
	searchKeyword  string
	searchPage     int32
	searchPageSize int32
}

func (*userHandlerRepository) EnsureUserIndex(context.Context) error { return nil }

func (r *userHandlerRepository) IndexUser(_ context.Context, doc domain.UserDocument) error {
	r.indexedUser = doc
	return nil
}

func (r *userHandlerRepository) ReindexUser(_ context.Context, doc domain.UserDocument) error {
	r.reindexedUser = doc
	return nil
}

func (r *userHandlerRepository) DeleteUser(_ context.Context, id int64) error {
	r.deletedUserID = id
	return nil
}

func (r *userHandlerRepository) EraseUserData(_ context.Context, userID, deletionJobID int64, policyVersion int32) error {
	r.erasedUserID = userID
	r.deletionJobID = deletionJobID
	r.policyVersion = policyVersion
	return nil
}

func (r *userHandlerRepository) SearchUsers(_ context.Context, keyword string, page, pageSize int32) ([]domain.UserHit, int64, error) {
	r.searchKeyword = keyword
	r.searchPage = page
	r.searchPageSize = pageSize
	return r.userHits, r.userTotal, nil
}
