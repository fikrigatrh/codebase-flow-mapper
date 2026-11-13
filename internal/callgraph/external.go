package callgraph

import "strings"

// categories map a qualified external name ("pkg.(Recv).Name") to a category (ARCHITECTURE §4.1).
// An entry matches the name itself or anything below it ("." or "/" boundary). First match wins.
var categories = []struct{ prefix, cat string }{
	{"database/sql", "db"}, {"gorm.io", "db"}, {"github.com/jackc/pgx", "db"},
	{"github.com/jmoiron/sqlx", "db"}, {"go.mongodb.org", "db"},
	{"github.com/redis", "cache"}, {"github.com/go-redis", "cache"},
	{"net/http.(*Client)", "http_client"}, {"net/http.Get", "http_client"}, {"net/http.Post", "http_client"},
	{"net/http.PostForm", "http_client"}, {"net/http.Head", "http_client"}, {"github.com/go-resty", "http_client"},
	{"github.com/segmentio/kafka-go", "queue"}, {"github.com/IBM/sarama", "queue"},
	{"github.com/rabbitmq", "queue"}, {"cloud.google.com/go/pubsub", "queue"},
	{"log", "log"}, {"go.uber.org/zap", "log"}, {"github.com/sirupsen/logrus", "log"}, {"github.com/rs/zerolog", "log"},
}

func category(qualified string) string {
	for _, c := range categories {
		if qualified == c.prefix || strings.HasPrefix(qualified, c.prefix+".") || strings.HasPrefix(qualified, c.prefix+"/") {
			return c.cat
		}
	}
	return "other"
}
