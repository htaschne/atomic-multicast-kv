package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
)

var (
	id             = flag.Int("id", envInt("PARTITION_ID", 0), "partition id")
	partitionCount = flag.Int("partitions", envInt("PARTITION_COUNT", DefaultPartitionCount), "total partition count")
	modeFlag       = flag.String("mode", envString("PROTOCOL_MODE", string(ModeOriginal)), "protocol mode: original or strengthened")
	peersArg       = flag.String("peers", envString("PEERS", defaultPeers(envInt("PARTITION_COUNT", DefaultPartitionCount), "localhost")), "comma-separated peer map, e.g. 0=http://localhost:4000,1=http://localhost:4001")
	skeenSvc       *Skeen
	router         Router
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
		Dst:  router.DestinationsForPut(reqBody.Key),
		Put: &PutPayload{
			Key:   reqBody.Key,
			Value: reqBody.Value,
		},
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	protocolLogf("[P%d] put request id=%s key=%d value=%d dst=%v", *id, req.ID, reqBody.Key, reqBody.Value, req.Dst)
	if _, err := skeenSvc.Submit(ctx, req); err != nil {
		protocolLogf("[P%d] error processing put request id=%s: %v", *id, req.ID, err)
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
		Dst:  router.DestinationsForRange(start, end),
		Range: &RangePayload{
			Start: start,
			End:   end,
		},
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	protocolLogf("[P%d] range request id=%s dst=%v start=%d end=%d", *id, req.ID, req.Dst, start, end)
	result, err := skeenSvc.Submit(ctx, req)
	if err != nil {
		protocolLogf("[P%d] error processing range request id=%s: %v", *id, req.ID, err)
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
	cfg := ClusterConfig{
		LocalPartition: PartitionID(*id),
		PartitionCount: *partitionCount,
		Peers:          peers,
		Mode:           mode,
	}
	if err := cfg.Validate(); err != nil {
		log.Fatal(err)
	}
	router, err = NewRouter(cfg.PartitionCount)
	if err != nil {
		log.Fatal(err)
	}

	skeenSvc = NewSkeen(
		cfg.LocalPartition,
		WithMode(cfg.Mode),
		WithStore(NewKVStore()),
		WithTransport(NewHTTPTransport(cfg.Peers)),
	)

	port := 4000 + int(cfg.LocalPartition)
	protocolLogf("[P%d] starting server on port %d mode=%s partitions=%d peers=%s", cfg.LocalPartition, port, cfg.Mode, cfg.PartitionCount, *peersArg)

	http.HandleFunc("/put", putHandler)
	http.HandleFunc("/range", rangeHandler)
	http.HandleFunc("/internal/protocol", protocolHandler)

	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		panic(err)
	}
}
