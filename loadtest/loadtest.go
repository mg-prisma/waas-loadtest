package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"sort"
	"time"
)

var (
	baseURL     string
	numRequests int
	threads     int
)

type Stats struct {
	SuccessfulGET  int
	SuccessfulPOST int
	TotalBytesSent int64
	TotalBytesRecv int64
	Errors         int
	Latencies      []time.Duration
}

// Comment represents a single comment in the guestbook
type Comment struct {
	Username string    `json:"username"`
	Message  string    `json:"message"`
	Time     time.Time `json:"time"`
}

func init() {
	flag.StringVar(&baseURL, "host", "http://your_guestbook_url.com", "Host and port of the guestbook app")
	flag.IntVar(&numRequests, "n", 1000, "Number of requests")
	flag.IntVar(&threads, "threads", 10, "Number of concurrent request threads")
	flag.Parse()
}

func main() {
	startTime := time.Now()
	statsChan := make(chan Stats, threads)
	requestsPerThread := numRequests / threads

	for i := 0; i < threads; i++ {
		go makeRequests(requestsPerThread, statsChan)
	}

	var (
		successfulGET  = 0
		successfulPOST = 0
		totalBytesSent int64
		totalBytesRecv int64
		errors         = 0
		latencies      []time.Duration
	)

	for i := 0; i < threads; i++ {
		s := <-statsChan
		successfulGET += s.SuccessfulGET
		successfulPOST += s.SuccessfulPOST
		totalBytesSent += s.TotalBytesSent
		totalBytesRecv += s.TotalBytesRecv
		errors += s.Errors
		latencies = append(latencies, s.Latencies...)
	}

	elapsedTime := time.Since(startTime)

	// Calculate percentiles for latencies
	latenciesSorted := make([]time.Duration, len(latencies))
	copy(latenciesSorted, latencies)

	// Sort latencies
	sortDurationSlice(latenciesSorted)

	percentiles := []int{50, 90, 95, 99}
	percentileLatencies := make(map[int]time.Duration)
	for _, p := range percentiles {
		idx := (len(latenciesSorted) * p) / 100
		percentileLatencies[p] = latenciesSorted[idx]
	}

	// Calculate requests per second
	requestsPerSecond := float64(numRequests) / elapsedTime.Seconds()

	// Print statistics
	fmt.Println("Elapsed Time:", elapsedTime)
	fmt.Println("Total Bytes Sent:", totalBytesSent)
	fmt.Println("Total Bytes Received:", totalBytesRecv)
	fmt.Println("Successful GET Requests:", successfulGET)
	fmt.Println("Successful POST Requests:", successfulPOST)
	fmt.Println("Total Errors:", errors)
	fmt.Println("50th Percentile Latency:", percentileLatencies[50])
	fmt.Println("90th Percentile Latency:", percentileLatencies[90])
	fmt.Println("95th Percentile Latency:", percentileLatencies[95])
	fmt.Println("99th Percentile Latency:", percentileLatencies[99])
	fmt.Println("Requests per Second:", requestsPerSecond)
}

func makeRequests(requestsPerThread int, statsChan chan Stats) {
	var s Stats
	for i := 0; i < requestsPerThread; i++ {
		// Generate random query parameter
		queryKey := randomString(10)
		queryValue := randomString(10)

		// Choose randomly between GET and POST
		if rand.Intn(2) == 0 {
			// Perform GET request with exponential backoff
			resp, latency, bytesSent, bytesRecv, err := exponentialBackoffGET(fmt.Sprintf("%s/comments?%s=%s", baseURL, queryKey, queryValue))
			if err != nil {
				fmt.Println("GET request error:", err)
				s.Errors++
				continue
			}
			s.TotalBytesSent += bytesSent
			s.TotalBytesRecv += bytesRecv
			s.Latencies = append(s.Latencies, latency)
			defer resp.Body.Close()

			s.SuccessfulGET++
		} else {
			// Generate random comment string for POST request
			comment := randomString(30)

			// Perform POST request with exponential backoff
			c := Comment{Time: time.Now(), Username: "test", Message: comment}
			resp, latency, bytesSent, bytesRecv, err := exponentialBackoffPOST(fmt.Sprintf("%s/comment?%s=%s", baseURL, queryKey, queryValue), c)
			if err != nil {
				fmt.Println("POST request error:", err)
				s.Errors++
				continue
			}
			s.TotalBytesSent += bytesSent
			s.TotalBytesRecv += bytesRecv
			s.Latencies = append(s.Latencies, latency)
			defer resp.Body.Close()

			s.SuccessfulPOST++
		}
	}
	statsChan <- s
}

func exponentialBackoffGET(url string) (*http.Response, time.Duration, int64, int64, error) {
	backoff := 20 * time.Millisecond
	var latency time.Duration
	var requestSize, responseSize int64
	for i := 0; i < 10; i++ {
		start := time.Now()
		resp, err := http.Get(url)
		latency = time.Since(start)
		if err == nil {
			// Calculate request and response sizes
			requestSize = 0 // No request body in GET request
			if resp != nil && resp.Body != nil {
				responseSize, _ = io.Copy(ioutil.Discard, resp.Body)
			}
			return resp, latency, requestSize, responseSize, nil
		}
		time.Sleep(backoff)
		backoff *= 2
		if backoff > 5*time.Second {
			backoff = 5 * time.Second
		}
	}
	return nil, 0, 0, 0, fmt.Errorf("exponential backoff exceeded")
}

func exponentialBackoffPOST(url string, comment Comment) (*http.Response, time.Duration, int64, int64, error) {
	backoff := 20 * time.Millisecond
	var latency time.Duration
	var requestSize, responseSize int64
	for i := 0; i < 10; i++ {
		start := time.Now()
		commentJSON, err := json.Marshal(comment)
		if err != nil {
			return nil, 0, 0, 0, err
		}

		resp, err := http.Post(url, "application/json", bytes.NewBuffer(commentJSON))
		latency = time.Since(start)
		if err == nil {
			// Calculate request and response sizes
			requestSize = int64(len(commentJSON))
			if resp != nil && resp.Body != nil {
				responseSize, _ = io.Copy(ioutil.Discard, resp.Body)
			}
			return resp, latency, requestSize, responseSize, nil
		}
		time.Sleep(backoff)
		backoff *= 2
		if backoff > 5*time.Second {
			backoff = 5 * time.Second
		}
	}
	return nil, 0, 0, 0, fmt.Errorf("exponential backoff exceeded")
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func sortDurationSlice(slice []time.Duration) {
	sort.Slice(slice, func(i, j int) bool { return slice[i] < slice[j] })
}
