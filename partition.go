package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

var (
	id = flag.Int("id", 0, "The partition id")
)

type PutRequest struct {
	Key   int `json:"key"`
	Value int `json:"value"`
}

func putHandler(w http.ResponseWriter, r *http.Request) {
	var req PutRequest

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	put(req.Key, req.Value)
	w.WriteHeader(http.StatusOK)
}

func parseIntQueryParam(r *http.Request, name string) (int, error) {
	value := r.URL.Query().Get(name)
	if value == "" {
		return 0, fmt.Errorf("missing query param: %s", name)
	}

	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("query param %s must be an integer", name)
	}

	return n, nil
}

func rangeHandler(w http.ResponseWriter, r *http.Request) {
	a, err := parseIntQueryParam(r, "a")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	b, err := parseIntQueryParam(r, "b")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result := getRange(a, b)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func main() {
	flag.Parse()

	port := 4000 + *id

	fmt.Printf("[%s][P%d] Starting server on port %d\n",
		time.Now().Format(time.RFC850), *id, port)

	http.HandleFunc("/put", putHandler)
	http.HandleFunc("/range", rangeHandler)

	err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)

	if err != nil {
		panic(err)
	}

}
