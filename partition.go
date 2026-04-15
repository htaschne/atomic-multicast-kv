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
	id       = flag.Int("id", 0, "The partition id")
	skeenSvc *Skeen
)

type PutRequest struct {
	Key   int `json:"key"`
	Value int `json:"value"`
}

func putHandler(w http.ResponseWriter, r *http.Request) {
	var reqBody PutRequest

	err := json.NewDecoder(r.Body).Decode(&reqBody)
	if err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	req := Request{
		ID:   newRequestID(PartitionID(*id)),
		Type: OpPut,
		Dst:  destinationsForPut(reqBody.Key),
		Put: &PutPayload{
			Key:   reqBody.Key,
			Value: reqBody.Value,
		},
	}

	_, err = skeenSvc.Submit(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

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
	start, err := parseIntQueryParam(r, "start")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	end, err := parseIntQueryParam(r, "end")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	req := Request{
		ID:   newRequestID(PartitionID(*id)),
		Type: OpRange,
		Dst: destinationsForRange(start, end),
		Range: &RangePayload{
			Start: start,
			End:   end,
		},
	}

	fmt.Printf("range request dst: %v\n", req.Dst)
	result, err := skeenSvc.Submit(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func main() {
	flag.Parse()
	skeenSvc = NewSkeen(PartitionID(*id))

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
