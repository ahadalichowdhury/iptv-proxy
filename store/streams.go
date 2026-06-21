package store

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (s *Store) nextID(ctx context.Context, name string) (int, error) {
	coll := s.db.Collection("counters")
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	var result struct {
		Seq int `bson:"seq"`
	}
	err := coll.FindOneAndUpdate(
		ctx,
		bson.M{"_id": name},
		bson.M{"$inc": bson.M{"seq": 1}},
		opts,
	).Decode(&result)
	if err != nil {
		return 0, err
	}
	return result.Seq, nil
}

func (s *Store) ListStreams(ctx context.Context) ([]Stream, error) {
	return s.listStreams(ctx, bson.M{})
}

func (s *Store) ListManualStreams(ctx context.Context) ([]Stream, error) {
	return s.listStreams(ctx, bson.M{"sourceId": nil})
}

func (s *Store) ListStreamsBySourceID(ctx context.Context, sourceID int) ([]StreamSummary, error) {
	cur, err := s.db.Collection("streams").Find(
		ctx,
		bson.M{"sourceId": sourceID},
		options.Find().SetProjection(bson.M{
			"_id": 1, "title": 1, "url": 1, "groupTitle": 1, "logo": 1, "channelKey": 1, "links": 1,
		}),
	)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var rows []StreamSummary
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []StreamSummary{}
	}
	return rows, nil
}

func (s *Store) ListStreamsForManualImport(ctx context.Context) ([]ManualImportStream, error) {
	cur, err := s.db.Collection("streams").Find(
		ctx,
		bson.M{},
		options.Find().SetProjection(bson.M{"_id": 1, "url": 1, "title": 1, "sourceId": 1}),
	)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var rows []ManualImportStream
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []ManualImportStream{}
	}
	return rows, nil
}

func (s *Store) GetStreamByID(ctx context.Context, id int) (*Stream, error) {
	var stream Stream
	err := s.db.Collection("streams").FindOne(ctx, bson.M{"_id": id}).Decode(&stream)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &stream, nil
}

func (s *Store) InsertStream(ctx context.Context, input StreamWrite) (*Stream, error) {
	id, err := s.nextID(ctx, "streams")
	if err != nil {
		return nil, err
	}

	stream := Stream{
		ID:         id,
		Title:      input.Title,
		URL:        input.URL,
		GroupTitle: input.GroupTitle,
		Logo:       input.Logo,
		ChannelKey: input.ChannelKey,
		Links:      input.Links,
		Status:     input.Status,
		SourceID:   input.SourceID,
		CreatedAt:  time.Now().UTC(),
	}
	if stream.Status == "" {
		stream.Status = StreamLive
	}

	_, err = s.db.Collection("streams").InsertOne(ctx, stream)
	if err != nil {
		return nil, err
	}
	return &stream, nil
}

func (s *Store) UpdateStream(ctx context.Context, id int, patch StreamPatch) error {
	set := bson.M{}
	if patch.Title != nil {
		set["title"] = *patch.Title
	}
	if patch.URL != nil {
		set["url"] = *patch.URL
	}
	if patch.GroupTitle != nil {
		set["groupTitle"] = *patch.GroupTitle
	}
	if patch.Logo != nil {
		set["logo"] = *patch.Logo
	}
	if patch.ChannelKey != nil {
		set["channelKey"] = *patch.ChannelKey
	}
	if patch.Links != nil {
		set["links"] = *patch.Links
	}
	if patch.Status != nil {
		set["status"] = *patch.Status
	}
	if patch.SourceID != nil {
		set["sourceId"] = *patch.SourceID
	}
	if len(set) == 0 {
		return nil
	}

	_, err := s.db.Collection("streams").UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": set})
	return err
}

func (s *Store) DeleteStream(ctx context.Context, id int) error {
	_, err := s.db.Collection("streams").DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (s *Store) DeleteStreamsBySourceID(ctx context.Context, sourceID int) error {
	_, err := s.db.Collection("streams").DeleteMany(ctx, bson.M{"sourceId": sourceID})
	return err
}

func (s *Store) listStreams(ctx context.Context, filter bson.M) ([]Stream, error) {
	cur, err := s.db.Collection("streams").Find(
		ctx,
		filter,
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}),
	)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var rows []Stream
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []Stream{}
	}
	return rows, nil
}
