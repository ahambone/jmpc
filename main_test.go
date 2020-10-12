// Unit Tests for a hashing HTTP service in GoLang.
// Copyright (C) 2020, Adam E. Hampton.  All Rights Reserved.
package main

import (
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

type testRequest struct {
	t         *testing.T
	clearText string
}

const numClients = 16

var testRequestChan = make(chan testRequest, numClients)

func init() {
	go func() {
		startupHTTPServices()
	}()
}

// TestInitialStats - stats should report zero initially.
func TestInitialStats(t *testing.T) {

	resp, err := http.Get("http://localhost:8080/stats")
	if err != nil {
		log.Fatal(err)
		t.Error(err)
	}
	defer resp.Body.Close()

	desiredResponse := "{\"total\":0,\"average\":0}"

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
		t.Error(err)
	}
	bodyStr := string(bodyBytes)

	if 0 != strings.Compare(desiredResponse, bodyStr) {
		t.Errorf("Expected a match to [%s], got [%s]", desiredResponse, bodyStr)
	}

}

func TestSingleHash(t *testing.T) {

	resp, err := http.PostForm("http://localhost:8080/hash",
		url.Values{"password": {"angryMonkey"}})
	if err != nil {
		log.Fatal(err)
		t.Error(err)
	}
	defer resp.Body.Close()

	desiredResponse := "1"

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
		t.Error(err)
	}
	bodyStr := string(bodyBytes)

	if 0 != strings.Compare(desiredResponse, bodyStr) {
		t.Errorf("Expected a match to [%s], got [%s]", desiredResponse, bodyStr)
	}

	// Immediately after submission we should get a 404, the results aren't ready.
	resp1, err1 := http.Get("http://localhost:8080/hash/1")
	if err1 != nil {
		log.Fatal(err1)
		t.Error(err1)
	}
	defer resp1.Body.Close()

	if http.StatusNotFound != resp1.StatusCode {
		t.Errorf("Expected StatusCode [%d], got [%d]", http.StatusNotFound, resp1.StatusCode)
	}

	desiredResponse = "Results not available for idNum: 1"

	bodyBytes1, err2 := ioutil.ReadAll(resp1.Body)
	if err2 != nil {
		log.Fatal(err2)
		t.Error(err2)
	}
	bodyStr1 := strings.TrimSpace(string(bodyBytes1))

	if 0 != strings.Compare(desiredResponse, bodyStr1) {
		t.Errorf("Expected a match to [%s], got [%s]", desiredResponse, bodyStr1)
	}

	// Wait a time to ensure the remote side is done working.
	time.Sleep(5010 * time.Millisecond)

	respExp, errExp := http.Get("http://localhost:8080/hash/1")
	if errExp != nil {
		log.Fatal(errExp)
		t.Error(errExp)
	}
	defer respExp.Body.Close()

	if http.StatusOK != respExp.StatusCode {
		t.Errorf("Expected StatusCode [%d], got [%d]", http.StatusOK, respExp.StatusCode)
	}

	desiredResponse = "ZEHhWB65gUlzdVwtDQArEyx+KVLzp/aTaRaPlBzYRIFj6vjFdqEb0Q5B8zVKCZ0vKbZPZklJz0Fd7su2A+gf7Q=="

	bodyBytesExp, errExp2 := ioutil.ReadAll(respExp.Body)
	if errExp2 != nil {
		log.Fatal(errExp2)
		t.Error(errExp2)
	}
	bodyStrExp := strings.TrimSpace(string(bodyBytesExp))

	if 0 != strings.Compare(desiredResponse, bodyStrExp) {
		t.Errorf("Expected a match to [%s], got [%s]", desiredResponse, bodyStrExp)
	}

}

func doOneRequest(tReq testRequest) {

	t := tReq.t
	clearText := tReq.clearText

	resp, err := http.PostForm("http://localhost:8080/hash", url.Values{"password": {clearText}})
	if err != nil {
		log.Fatal(err)
		t.Error(err)
	}
	defer resp.Body.Close()

	if http.StatusOK != resp.StatusCode {
		log.Printf("Expected StatusCode [%d], got [%d]", http.StatusOK, resp.StatusCode)
		t.Errorf("Expected StatusCode [%d], got [%d]", http.StatusOK, resp.StatusCode)
	}

}

// Note - NOT RFC4122 compliant, thus just generates strings for passwords.
func pseudoUUID() (uuid string) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	uuid = fmt.Sprintf("%X-%X-%X-%X-%X", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
	return
}

func TestSeveralCalls(t *testing.T) {
	var numTests int = 100
	for i := 0; i < numTests; i++ {
		tReq := testRequest{t, pseudoUUID()}
		testRequestChan <- tReq
		go doOneRequest(<-testRequestChan)
	}
}

func TestStats(t *testing.T) {

	resp, err := http.Get("http://localhost:8080/stats")
	if err != nil {
		log.Fatal(err)
		t.Error(err)
	}
	defer resp.Body.Close()

	if http.StatusOK != resp.StatusCode {
		log.Printf("Expected StatusCode [%d], got [%d]", http.StatusOK, resp.StatusCode)
		t.Errorf("Expected StatusCode [%d], got [%d]", http.StatusOK, resp.StatusCode)
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
		t.Error(err)
	}
	bodyStr := string(bodyBytes)

	if 0 == len(bodyStr) {
		t.Errorf("Expected a populated string, got [%s]", bodyStr)
	}

	if !strings.Contains(bodyStr, "total") {
		t.Errorf("Expected a string to contain 'total', got [%s]", bodyStr)
	}

	if !strings.Contains(bodyStr, "average") {
		t.Errorf("Expected a string to contain 'average', got [%s]", bodyStr)
	}

}

// TestShutDown tests shuttind down the server, so keep it at the bottom of
// the test module.  This ensures it cleanly closes down testing.
func TestShutDown(t *testing.T) {
	// log.Printf("TestShutDown(): Issuing shutdown call...")
	http.Get("http://localhost:8080/stats")
}
