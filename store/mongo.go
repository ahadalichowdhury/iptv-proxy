package store

import (
	"context"
	"os"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const defaultDBName = "tv_proxy"

type Store struct {
	client *mongo.Client
	db     *mongo.Database
}

var (
	globalStore *Store
	storeOnce   sync.Once
	storeErr    error
)

func Open(ctx context.Context) (*Store, error) {
	storeOnce.Do(func() {
		uri := os.Getenv("MONGODB_URI")
		if uri == "" {
			storeErr = errMissingEnv("MONGODB_URI")
			return
		}

		dbName := os.Getenv("MONGODB_DB")
		if dbName == "" {
			dbName = defaultDBName
		}

		client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
		if err != nil {
			storeErr = err
			return
		}

		pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := client.Ping(pingCtx, nil); err != nil {
			storeErr = err
			return
		}

		store := &Store{
			client: client,
			db:     client.Database(dbName),
		}
		if err := store.ensureIndexes(ctx); err != nil {
			storeErr = err
			return
		}

		globalStore = store
	})

	return globalStore, storeErr
}

func (s *Store) DB() *mongo.Database {
	return s.db
}

func (s *Store) Close(ctx context.Context) error {
	if s.client == nil {
		return nil
	}
	return s.client.Disconnect(ctx)
}

func (s *Store) ensureIndexes(ctx context.Context) error {
	indexCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	streamIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "createdAt", Value: -1}}},
		{Keys: bson.D{{Key: "sourceId", Value: 1}}},
		{Keys: bson.D{{Key: "channelKey", Value: 1}, {Key: "sourceId", Value: 1}}},
	}
	if _, err := s.db.Collection("streams").Indexes().CreateMany(indexCtx, streamIndexes); err != nil {
		return err
	}

	sourceIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "sourceUrl", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	}
	if _, err := s.db.Collection("playlist_sources").Indexes().CreateMany(indexCtx, sourceIndexes); err != nil {
		return err
	}

	presenceIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "lastSeen", Value: 1}}},
	}
	_, err := s.db.Collection("presence_sessions").Indexes().CreateMany(indexCtx, presenceIndexes)
	return err
}

type missingEnvError string

func (e missingEnvError) Error() string {
	return string(e) + " is not configured"
}

func errMissingEnv(key string) error {
	return missingEnvError(key)
}
