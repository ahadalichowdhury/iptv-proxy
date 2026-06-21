package store

import "time"

type StreamStatus string

const (
	StreamLive    StreamStatus = "live"
	StreamOffline StreamStatus = "offline"
)

type Stream struct {
	ID         int          `json:"id" bson:"_id"`
	Title      string       `json:"title" bson:"title"`
	URL        string       `json:"url" bson:"url"`
	GroupTitle *string      `json:"groupTitle" bson:"groupTitle"`
	Logo       *string      `json:"logo" bson:"logo"`
	ChannelKey *string      `json:"channelKey" bson:"channelKey"`
	Links      *string      `json:"links" bson:"links"`
	Status     StreamStatus `json:"status" bson:"status"`
	SourceID   *int         `json:"sourceId" bson:"sourceId"`
	CreatedAt  time.Time    `json:"createdAt" bson:"createdAt"`
}

type PlaylistSource struct {
	ID              int        `json:"id" bson:"_id"`
	Title           string     `json:"title" bson:"title"`
	SourceURL       string     `json:"sourceUrl" bson:"sourceUrl"`
	SourceSnapshot  *string    `json:"sourceSnapshot" bson:"sourceSnapshot"`
	BaseURL         *string    `json:"baseUrl" bson:"baseUrl"`
	LastRefreshedAt *time.Time `json:"lastRefreshedAt" bson:"lastRefreshedAt"`
	CreatedAt       time.Time  `json:"createdAt" bson:"createdAt"`
}

type StreamWrite struct {
	Title      string       `json:"title"`
	URL        string       `json:"url"`
	GroupTitle *string      `json:"groupTitle"`
	Logo       *string      `json:"logo"`
	ChannelKey *string      `json:"channelKey"`
	Links      *string      `json:"links"`
	Status     StreamStatus `json:"status"`
	SourceID   *int         `json:"sourceId"`
}

type PlaylistSourceWrite struct {
	Title          string  `json:"title"`
	SourceURL      string  `json:"sourceUrl"`
	SourceSnapshot *string `json:"sourceSnapshot"`
	BaseURL        *string `json:"baseUrl"`
}

type StreamPatch struct {
	Title      *string       `json:"title"`
	URL        *string       `json:"url"`
	GroupTitle *string       `json:"groupTitle"`
	Logo       *string       `json:"logo"`
	ChannelKey *string       `json:"channelKey"`
	Links      *string       `json:"links"`
	Status     *StreamStatus `json:"status"`
	SourceID   *int          `json:"sourceId"`
}

type PlaylistSourcePatch struct {
	Title           *string    `json:"title"`
	SourceURL       *string    `json:"sourceUrl"`
	SourceSnapshot  *string    `json:"sourceSnapshot"`
	BaseURL         *string    `json:"baseUrl"`
	LastRefreshedAt *time.Time `json:"lastRefreshedAt"`
}

type StreamSummary struct {
	ID         int     `json:"id" bson:"_id"`
	Title      string  `json:"title" bson:"title"`
	URL        string  `json:"url" bson:"url"`
	GroupTitle *string `json:"groupTitle" bson:"groupTitle"`
	Logo       *string `json:"logo" bson:"logo"`
	ChannelKey *string `json:"channelKey" bson:"channelKey"`
	Links      *string `json:"links" bson:"links"`
}

type ManualImportStream struct {
	ID       int    `json:"id" bson:"_id"`
	URL      string `json:"url" bson:"url"`
	Title    string `json:"title" bson:"title"`
	SourceID *int   `json:"sourceId" bson:"sourceId"`
}

type PresenceHeartbeat struct {
	SessionID string  `json:"sessionId"`
	Path      *string `json:"path"`
}
