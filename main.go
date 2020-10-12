// Package to illustrate a hashing HTTP service in GoLang.
// Copyright (C) 2020, Adam E. Hampton.  All Rights Reserved.
package main

import (
	"context"
	"crypto/sha512"
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Represents a request to hash a password.  ID is assigned at the time
// a request is made and the clear text submitted by the requester.
type hashRequest struct {
	idNum     uint64
	clearText string
}

// Result container for the stats endpoint.
type statsResult struct {
	// Public: count of requests to the ​/hash​ endpoint made to the server
	Total uint64 `json:"total"`
	// Public: average time taken to process all requests in microseconds
	Average uint64 `json:"average"`
}

// Fixed delay before hashing as required by the project specification.
var hashDelay time.Duration = 5 * time.Second

// Serial number for hash requests.
var hashRequests uint64 = 0

// Total time accumulated in processing the requests.
var timeMetricAccumulator uint64 = 0

// Channel for hashRequest queued to process their SHA512 hashes.
var hashRequestChannel = make(chan hashRequest)

// Concurrent map housting the mapping from request ID uint64 to hash string.
var resultMap sync.Map

// The implementation of `sync.Map` does not offer a count, so track it ourselves.
var resultMapCount uint64 = 0

// Hash a shutdown been requested? Implemented as a uInt for concurrency.
var shutdownRequested uint32 = 0

// calcHashDelayed processes a hashRequest and keeps track how long it took.
func calcHashDelayed(hReqCh chan hashRequest) {

	// Pull the channel record and apply the sleep delay.
	hReq := <-hReqCh
	time.Sleep(hashDelay)

	// Capture timing statistics for the /hash endpont.
	t0 := time.Now()
	defer func(startTime time.Time) {
		duration := time.Now().Sub(startTime)
		microSecs := uint64(duration.Microseconds())
		atomic.AddUint64(&timeMetricAccumulator, microSecs)
	}(t0)

	ckSum := sha512.Sum512([]byte(hReq.clearText))
	b64Str := b64.StdEncoding.EncodeToString([]byte(ckSum[:]))
	// log.Printf("%s --> %s \n", hReq.clearText, b64Str)

	resultMap.Store(hReq.idNum, b64Str)  // Store the value.
	atomic.AddUint64(&resultMapCount, 1) // Bump peg counter after.

	return
}

func hashHandler(w http.ResponseWriter, r *http.Request) {

	// Capture timing statistics for the /hash endpont.
	t0 := time.Now()
	defer func(startTime time.Time) {
		nowTime := time.Now()
		duration := nowTime.Sub(startTime)
		microSecs := uint64(duration.Microseconds())
		atomic.AddUint64(&timeMetricAccumulator, microSecs)
		/*
			// These could share a common lock but this average metric can be fuzzy.
			totalMicroSecs := atomic.LoadUint64(&timeMetricAccumulator)
			requestCount := atomic.LoadUint64(&hashRequests)
			var avgMicroSecs uint64 = 0
			if 0 != requestCount {
				avgMicroSecs = totalMicroSecs / requestCount
			}
			logMsg := fmt.Sprintf("rest - duration:%v, total:%v, avg:%v\n",
				microSecs, totalMicroSecs, avgMicroSecs)
			log.Println(logMsg)
		*/
	}(t0)

	err := r.ParseForm()
	if err != nil {
		panic(err)
	}

	// log.Println("r.PostForm", r.PostForm)
	// log.Println("r.Form", r.Form)
	// body, err := ioutil.ReadAll(r.Body)
	// if err != nil {
	// 	http.Error(w, err.Error(), http.StatusInternalServerError)
	// 	return
	// }
	// _ = body
	// log.Println("r.Body", string(body))

	// Sanity check to make sure we recieve valid input.
	clearText := r.PostFormValue("password")
	if len(clearText) > 0 {
		idNum := atomic.AddUint64(&hashRequests, 1)
		fmt.Printf("req %d --> %s \n", idNum, clearText)

		// Enqueue the request to calculate the hash in the future.
		var hReq = hashRequest{idNum, clearText}
		go calcHashDelayed(hashRequestChannel)
		hashRequestChannel <- hReq

		// Return the idNum to the client.
		fmt.Fprintf(w, "%d", idNum)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/hash/")
	if len(idStr) > 0 {

		idNum, parseErr := strconv.ParseUint(idStr, 10, 64)
		if parseErr != nil {
			errMsg := fmt.Sprintf("Requested idNum not valid integer: %s", idStr)
			http.Error(w, errMsg, http.StatusBadRequest)
			return
		}

		b64Str, recFound := resultMap.Load(idNum)
		if !recFound {
			errMsg := fmt.Sprintf("Results not available for idNum: %d", idNum)
			http.Error(w, errMsg, http.StatusNotFound)
			return
		}

		fmt.Fprintf(w, "%s", b64Str)
		return
	}

	http.Error(w, "Form field 'password' or path request ID parameter required.",
		http.StatusBadRequest)

	return
}

func statsHandler(w http.ResponseWriter, r *http.Request) {

	// Reject any requests that arrive after shutdown.
	if 0 < atomic.LoadUint32(&shutdownRequested) {

	}

	// These could share a common lock but this average metric can be fuzzy.
	totalMicroSecs := atomic.LoadUint64(&timeMetricAccumulator)
	requestCount := atomic.LoadUint64(&hashRequests)
	var avgMicroSecs uint64 = 0
	if 0 != requestCount {
		avgMicroSecs = totalMicroSecs / requestCount
	}

	w.Header().Set("Content-Type", "application/json")

	nowStats := statsResult{Total: requestCount, Average: avgMicroSecs}
	jsonStr, _ := json.Marshal(nowStats)

	fmt.Fprintf(w, "%s", jsonStr)

	return
}

func startupHTTPServices() {

	// Wait for in-flight work to complete.
	defer func() {
		requestCount := atomic.LoadUint64(&hashRequests)
		resultMapCnt := atomic.LoadUint64(&resultMapCount)
		for requestCount != resultMapCnt {
			log.Printf("Shutting down, waiting for %d / %d ...", resultMapCnt, requestCount)
			time.Sleep(1 * time.Second)
			requestCount = atomic.LoadUint64(&hashRequests)
			resultMapCnt = atomic.LoadUint64(&resultMapCount)
		}
		log.Printf("Exiting cleanly, hashes processed: %d", hashRequests)
	}()

	m := http.NewServeMux()
	s := http.Server{Addr: ":8080", Handler: m}

	m.HandleFunc("/hash", hashHandler)
	m.HandleFunc("/hash/", hashHandler)
	m.HandleFunc("/stats", statsHandler)

	// Shutdown is treated specially.
	m.HandleFunc("/shutdown", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Shutdown requested...")
		fmt.Fprintf(w, "Shutdown requested.")
		defer func() {
			s.Shutdown(context.Background())
		}()
	})
	if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func main() {
	startupHTTPServices()
}
