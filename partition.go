package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	id       = flag.Int("id", envInt("PARTITION_ID", 0), "partition id")
	modeFlag = flag.String("mode", envString("PROTOCOL_MODE", string(ModeOriginal)), "protocol mode: original or strengthened")
	peersArg = flag.String("peers", envString("PEERS", "0=http://localhost:4000,1=http://localhost:4001"), "comma-separated peer map, e.g. 0=http://localhost:4000,1=http://localhost:4001")
	skeenSvc *Skeen
)

type PutRequest struct {
	Key   int `json:"key"`
	Value int `json:"value"`
}

func putHandler(w http.ResponseWriter, r *http.Request) {
	var reqBody PutRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
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

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	log.Printf("[P%d] put request id=%s key=%d value=%d dst=%v", *id, req.ID, reqBody.Key, reqBody.Value, req.Dst)
	if _, err := skeenSvc.Submit(ctx, req); err != nil {
		log.Printf("[P%d] error processing put request id=%s: %v", *id, req.ID, err)
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
		Dst:  destinationsForRange(start, end),
		Range: &RangePayload{
			Start: start,
			End:   end,
		},
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	log.Printf("[P%d] range request id=%s dst=%v start=%d end=%d", *id, req.ID, req.Dst, start, end)
	result, err := skeenSvc.Submit(ctx, req)
	if err != nil {
		log.Printf("[P%d] error processing range request id=%s: %v", *id, req.ID, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func protocolHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var msg ProtocolMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		writeProtocolError(w, http.StatusBadRequest, "invalid protocol message")
		return
	}

	resp, err := skeenSvc.ReceiveProtocol(r.Context(), msg)
	if err != nil {
		writeProtocolError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func writeProtocolError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func parsePeers(raw string) (map[PartitionID]string, error) {
	peers := make(map[PartitionID]string)
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		keyValue := strings.SplitN(part, "=", 2)
		if len(keyValue) != 2 {
			return nil, fmt.Errorf("invalid peer entry %q", part)
		}
		idValue, err := strconv.Atoi(strings.TrimSpace(keyValue[0]))
		if err != nil {
			return nil, fmt.Errorf("invalid peer id %q", keyValue[0])
		}
		peers[PartitionID(idValue)] = strings.TrimSpace(keyValue[1])
	}
	if len(peers) == 0 {
		return nil, fmt.Errorf("no peers configured")
	}
	return peers, nil
}

func envString(name, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	return value
}

func envInt(name string, fallback int) int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}

func main() {
	flag.Parse()

	mode, err := parseProtocolMode(*modeFlag)
	if err != nil {
		log.Fatal(err)
	}
	peers, err := parsePeers(*peersArg)
	if err != nil {
		log.Fatal(err)
	}

	skeenSvc = NewSkeen(
		PartitionID(*id),
		WithMode(mode),
		WithStore(NewKVStore()),
		WithTransport(NewHTTPTransport(peers)),
	)

	port := 4000 + *id
	log.Printf("[P%d] starting server on port %d mode=%s peers=%s", *id, port, mode, *peersArg)

	http.HandleFunc("/put", putHandler)
	http.HandleFunc("/range", rangeHandler)
	http.HandleFunc("/internal/protocol", protocolHandler)

	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		panic(err)
	}
}
