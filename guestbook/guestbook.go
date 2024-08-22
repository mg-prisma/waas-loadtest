package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
)

// Comment represents a single comment in the guestbook
type Comment struct {
	Username string    `json:"username"`
	Message  string    `json:"message"`
	Time     time.Time `json:"time"`
}

// RedisClient is a wrapper for Redis client
type RedisClient struct {
	client *redis.Client
}

func main() {
	// Initialize Redis client
	redisClient := NewRedisClient()

	// Handle POST requests to add a comment
	http.HandleFunc("/comment", postCommentHandler(redisClient))

	// Handle GET requests to retrieve comments
	http.HandleFunc("/comments", getCommentsHandler(redisClient))

	// Serve over HTTPS with TLS certificate and key
	err := http.ListenAndServeTLS("0.0.0.0:8080", "clustereddb.pem", "clustereddb.key", nil)
	if err != nil {
		log.Fatal("ListenAndServeTLS: ", err)
	}
}

// NewRedisClient creates a new Redis client
func NewRedisClient() *RedisClient {
	// Initialize Redis connection options
	opt := redis.Options{
		Addr:     "redis-container:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	}

	// Create and return a new Redis client
	return &RedisClient{
		client: redis.NewClient(&opt),
	}
}

// postCommentHandler handles the POST request to add a comment
func postCommentHandler(redisClient *RedisClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Decode JSON request body
		var newComment Comment
		err := json.NewDecoder(r.Body).Decode(&newComment)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Add timestamp to the comment
		newComment.Time = time.Now()

		// Convert the comment to JSON
		commentJSON, err := json.Marshal(newComment)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Add the comment to Redis
		err = redisClient.client.LPush(r.Context(), "comments", commentJSON).Err()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Respond with success message
		w.WriteHeader(http.StatusCreated)
	}
}

// getCommentsHandler handles the GET request to retrieve comments
func getCommentsHandler(redisClient *RedisClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Retrieve the most recent comments from Redis
		commentsJSON, err := redisClient.client.LRange(r.Context(), "comments", 0, 9).Result()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Decode comments from JSON
		var comments []Comment
		for _, commentJSON := range commentsJSON {
			var comment Comment
			err := json.Unmarshal([]byte(commentJSON), &comment)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			comments = append(comments, comment)
		}

		// Respond with the comments
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(comments)
	}
}
