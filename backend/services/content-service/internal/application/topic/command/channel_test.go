package command

import (
	"context"
	"errors"
	"testing"

	channelDomain "content-service/internal/domain/channel"
	domain "content-service/internal/domain/topic"
)

func TestCreateValidatesChannel(t *testing.T) {
	repo := newFakeRepo()
	channels := &fakeChannelReader{channel: &channelDomain.Channel{ID: 77, OwnerID: 2, Name: "channel"}}
	service := NewServiceWithChannelReader(repo, fakeIDGen{}, &fakePublisher{}, &fakeCommentReader{}, nil, nil, nil, channels)

	created, err := service.Create(context.Background(), domain.CreateCmd{Slug: "in-channel", Title: "title", Body: "body", AuthorID: 1, ChannelID: 77})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if created.ChannelID != 77 {
		t.Fatalf("ChannelID = %d, want 77", created.ChannelID)
	}
}

func TestCreateRejectsMissingOrArchivedChannel(t *testing.T) {
	tests := []struct {
		name     string
		channels *fakeChannelReader
		want     error
	}{
		{name: "missing", channels: &fakeChannelReader{err: channelDomain.ErrNotFound}, want: domain.ErrChannelNotFound},
		{name: "archived", channels: &fakeChannelReader{channel: &channelDomain.Channel{ID: 77, OwnerID: 2, Name: "channel", IsArchived: true}}, want: domain.ErrChannelArchived},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := newFakeRepo()
			service := NewServiceWithChannelReader(repo, fakeIDGen{}, &fakePublisher{}, &fakeCommentReader{}, nil, nil, nil, test.channels)
			_, err := service.Create(context.Background(), domain.CreateCmd{Slug: test.name, Title: "title", Body: "body", AuthorID: 1, ChannelID: 77})
			if !errors.Is(err, test.want) {
				t.Fatalf("Create error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestUpdateRejectsArchivedSourceOrDestinationChannel(t *testing.T) {
	tests := []struct {
		name     string
		channels map[int64]*channelDomain.Channel
	}{
		{
			name: "archived source",
			channels: map[int64]*channelDomain.Channel{
				77: {ID: 77, OwnerID: 2, Name: "source", IsArchived: true},
				88: {ID: 88, OwnerID: 2, Name: "destination"},
			},
		},
		{
			name: "archived destination",
			channels: map[int64]*channelDomain.Channel{
				77: {ID: 77, OwnerID: 2, Name: "source"},
				88: {ID: 88, OwnerID: 2, Name: "destination", IsArchived: true},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := newFakeRepo()
			topic, err := domain.New(101, domain.CreateCmd{Slug: "topic", Title: "title", Body: "body", AuthorID: 1, ChannelID: 77})
			if err != nil {
				t.Fatal(err)
			}
			repo.topics[topic.ID] = topic
			channels := &fakeChannelReader{channels: test.channels}
			service := NewServiceWithChannelReader(repo, fakeIDGen{}, &fakePublisher{}, &fakeCommentReader{}, nil, nil, nil, channels)

			_, err = service.Update(context.Background(), topic.ID, domain.UpdateCmd{Title: "updated", Body: "body", ChannelID: 88})
			if !errors.Is(err, domain.ErrChannelArchived) {
				t.Fatalf("Update error = %v, want ErrChannelArchived", err)
			}
			if repo.topics[topic.ID].ChannelID != 77 {
				t.Fatalf("stored ChannelID = %d, want 77", repo.topics[topic.ID].ChannelID)
			}
		})
	}
}

type fakeChannelReader struct {
	channel  *channelDomain.Channel
	channels map[int64]*channelDomain.Channel
	err      error
}

func (r *fakeChannelReader) FindChannelByID(_ context.Context, id int64, _ int64, _ bool) (*channelDomain.Channel, error) {
	if r.channels != nil {
		channel, ok := r.channels[id]
		if !ok {
			return nil, channelDomain.ErrNotFound
		}
		return channel, nil
	}
	return r.channel, r.err
}
