package main

import (
	"encoding/json"
	"flag"
	"log/slog"
	"net/http"
)

type (
	jsonRPCReq struct {
		Version string          `json:"jsonrpc"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
		ID      string          `json:"id"`
	}
	jsonRPCReply struct {
		Version string           `json:"jsonrpc"`
		Result  *json.RawMessage `json:"result,omitempty"`
		Error   struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
		ID string `json:"id"`
	}
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var jreq jsonRPCReq
		json.NewDecoder(r.Body).Decode(&jreq)
		slog.Info("Got request", "method", jreq.Method, "id", jreq.ID, "remote", r.RemoteAddr)
		var jres jsonRPCReply
		jres.ID = jreq.ID
		jres.Version = jreq.Version
		jres.Result = &jreq.Params
		json.NewEncoder(w).Encode(jres)
	})
	bind := flag.String("bind", "localhost:8080", "Bind addr")
	flag.Parse()
	http.ListenAndServe(*bind, nil)
}
