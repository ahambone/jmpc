// Unit Tests for a hashing HTTP service in GoLang.
// Copyright (C) 2020, Adam E. Hampton.  All Rights Reserved.
package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"
)

func shutItAllDown() {
	http.Get("http://localhost:8080/shutdown")
	os.Exit(0)
}

// TestInitialStats - stats should report zero initially.
func TestInitialStats(t *testing.T) {

	go func() {
		startupHTTPServices()
	}()

	defer shutItAllDown()

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

	if 0 == strings.Compare(desiredResponse, bodyStr) {
		t.Errorf("Expected a match to [%s], got [%s]", desiredResponse, bodyStr)
	}

}
