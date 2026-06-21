package store

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (s *Store) RecordPresenceHeartbeat(ctx context.Context, sessionID string, path *string) error {
	_, err := s.db.Collection("presence_sessions").UpdateOne(
		ctx,
		bson.M{"_id": sessionID},
		bson.M{"$set": bson.M{
			"lastSeen": time.Now().UTC(),
			"path":     path,
		}},
		options.Update().SetUpsert(true),
	)
	return err
}

func (s *Store) CountActivePresenceSessions(ctx context.Context, cutoff time.Time) (int64, error) {
	return s.db.Collection("presence_sessions").CountDocuments(ctx, bson.M{
		"lastSeen": bson.M{"$gt": cutoff},
	})
}

func (s *Store) DeleteStalePresenceSessions(ctx context.Context, cutoff time.Time) error {
	_, err := s.db.Collection("presence_sessions").DeleteMany(ctx, bson.M{
		"lastSeen": bson.M{"$lt": cutoff},
	})
	return err
}
