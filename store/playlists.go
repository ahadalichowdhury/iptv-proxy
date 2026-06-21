package store

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func (s *Store) ListPlaylistSources(ctx context.Context) ([]PlaylistSource, error) {
	cur, err := s.db.Collection("playlist_sources").Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var rows []PlaylistSource
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []PlaylistSource{}
	}
	return rows, nil
}

func (s *Store) GetPlaylistSourceByID(ctx context.Context, id int) (*PlaylistSource, error) {
	var source PlaylistSource
	err := s.db.Collection("playlist_sources").FindOne(ctx, bson.M{"_id": id}).Decode(&source)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &source, nil
}

func (s *Store) GetPlaylistSourceByURL(ctx context.Context, sourceURL string) (*PlaylistSource, error) {
	var source PlaylistSource
	err := s.db.Collection("playlist_sources").FindOne(ctx, bson.M{"sourceUrl": sourceURL}).Decode(&source)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &source, nil
}

func (s *Store) FindPlaylistSourceIDByURL(ctx context.Context, sourceURL string) (*int, error) {
	var result struct {
		ID int `bson:"_id"`
	}
	err := s.db.Collection("playlist_sources").FindOne(
		ctx,
		bson.M{"sourceUrl": sourceURL},
	).Decode(&result)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &result.ID, nil
}

func (s *Store) InsertPlaylistSource(ctx context.Context, input PlaylistSourceWrite) (*PlaylistSource, error) {
	id, err := s.nextID(ctx, "playlist_sources")
	if err != nil {
		return nil, err
	}

	source := PlaylistSource{
		ID:             id,
		Title:          input.Title,
		SourceURL:      input.SourceURL,
		SourceSnapshot: input.SourceSnapshot,
		BaseURL:        input.BaseURL,
		CreatedAt:      time.Now().UTC(),
	}

	_, err = s.db.Collection("playlist_sources").InsertOne(ctx, source)
	if err != nil {
		return nil, err
	}
	return &source, nil
}

func (s *Store) UpdatePlaylistSource(ctx context.Context, id int, patch PlaylistSourcePatch) error {
	set := bson.M{}
	if patch.Title != nil {
		set["title"] = *patch.Title
	}
	if patch.SourceURL != nil {
		set["sourceUrl"] = *patch.SourceURL
	}
	if patch.SourceSnapshot != nil {
		set["sourceSnapshot"] = *patch.SourceSnapshot
	}
	if patch.BaseURL != nil {
		set["baseUrl"] = *patch.BaseURL
	}
	if patch.LastRefreshedAt != nil {
		set["lastRefreshedAt"] = *patch.LastRefreshedAt
	}
	if len(set) == 0 {
		return nil
	}

	_, err := s.db.Collection("playlist_sources").UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": set})
	return err
}

func (s *Store) DeletePlaylistSource(ctx context.Context, id int) error {
	if err := s.DeleteStreamsBySourceID(ctx, id); err != nil {
		return err
	}
	_, err := s.db.Collection("playlist_sources").DeleteOne(ctx, bson.M{"_id": id})
	return err
}
