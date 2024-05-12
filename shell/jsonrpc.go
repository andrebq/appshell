package shell

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/d5/tengo/v2"
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

func (s *Shell) EnableJSONRPCClient() {
	const internalRPCErrorCode = -32603
	callCount := uint64(0)
	s.jsonrpcMod = map[string]tengo.Object{
		"call": &tengo.UserFunction{
			Name: "call",
			Value: tengo.CallableFunc(func(args ...tengo.Object) (tengo.Object, error) {
				if len(args) != 3 {
					return tengo.UndefinedValue, tengo.ErrWrongNumArguments
				}
				endpoint, ok := tengo.ToString(args[0])
				if !ok {
					return tengo.UndefinedValue, tengo.ErrInvalidArgumentType{
						Name:     "endpoint",
						Expected: "string",
						Found:    args[0].TypeName(),
					}
				}
				method, ok := tengo.ToString(args[1])
				if !ok {
					return tengo.UndefinedValue, tengo.ErrInvalidArgumentType{
						Name:     "endpoint",
						Expected: "string",
						Found:    args[0].TypeName(),
					}
				}
				params, err := json.Marshal(tengo.ToInterface(args[2]))
				if err != nil {
					return tengo.UndefinedValue, tengo.ErrInvalidArgumentType{
						Name:     "params",
						Expected: "any (json serializable)",
						Found:    args[2].TypeName(),
					}
				}

				callCount++
				jreq := jsonRPCReq{
					Version: "2.0",
					Method:  method,
					Params:  json.RawMessage(params),
					ID:      strconv.FormatUint(callCount, 36),
				}
				buf, _ := json.Marshal(jreq)

				req, err := http.NewRequestWithContext(s.ctx, "POST", endpoint, bytes.NewBuffer(buf))
				if err != nil {
					return tengo.UndefinedValue, err
				}
				res, err := http.DefaultClient.Do(req)
				if err != nil {
					return tengo.UndefinedValue, err
				}
				defer res.Body.Close()
				var reply jsonRPCReply
				if res.StatusCode != 200 {
					reply.ID = jreq.ID
					reply.Error.Code = internalRPCErrorCode
					reply.Error.Message = fmt.Sprintf("unexpected status code from server: %v", res.StatusCode)
				} else {
					json.NewDecoder(res.Body).Decode(&reply)
				}

				if reply.Error.Code != 0 {
					return tengo.UndefinedValue, fmt.Errorf("[json-rpc-error: %v] %v", reply.Error.Code, reply.Error.Message)
				} else if reply.Result == nil {
					return tengo.UndefinedValue, fmt.Errorf("[json-rpc-error: %v] %v", internalRPCErrorCode, "empty result")
				}

				var out any
				err = json.Unmarshal(*reply.Result, &out)
				if err != nil {
					return tengo.UndefinedValue, fmt.Errorf("[json-rpc-error: %v] decoding error: %v", internalRPCErrorCode, err)
				}

				return tengo.FromInterface(out)
			}),
		},
	}
}
