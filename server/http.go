package server

import (
	"co-budget/app"
	"co-budget/data"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/starfederation/datastar-go/datastar"
)

type HttpPostFieldDataType string

const (
	HttpPostFieldTypeString HttpPostFieldDataType = "HttpPostFieldTypeString"
	HttpPostFieldTypeF64    HttpPostFieldDataType = "HttpPostFieldTypeF64"
)

type HttpServerResponseDTO struct {
	Succes  bool   `json:"succes"`
	Message string `json:"message"`
}

type HTTPServer struct {
	mu             sync.Mutex
	sseConnections map[int64]DatastarSSEStream
	nextSSEID      int64
}

type DatastarSSEStream struct {
	sse  *datastar.ServerSentEventGenerator
	done <-chan struct{}
}

type sseEv struct {
	eventType string
	eventData string
}

func NewHTTPServer() *http.Server {
	srv := &HTTPServer{
		sseConnections: map[int64]DatastarSSEStream{},
	}
	mux := http.NewServeMux()

	// Static files
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, app.Layout())
	})
	mux.HandleFunc("/datastar.js", func(w http.ResponseWriter, r *http.Request) {
		srv.serveStaticFile(w, r, "datastar.js", "javascript")
	})
	mux.HandleFunc("/main.css", func(w http.ResponseWriter, r *http.Request) {
		srv.serveStaticFile(w, r, "main.css", "css")
	})

	// Setup the sse connection
	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		stream := datastar.NewSSE(w, r)
		srv.mu.Lock()
		srv.nextSSEID++
		connectionID := srv.nextSSEID
		srv.sseConnections[connectionID] = DatastarSSEStream{sse: stream, done: r.Context().Done()}
		defer delete(srv.sseConnections, connectionID)
		srv.mu.Unlock()

		<-r.Context().Done()
	})

	// Actual endpoints
	mux.HandleFunc("/accounts/new", srv.createAccount)
	mux.HandleFunc("/accounts/update", srv.updateAccount)
	mux.HandleFunc("/accounts/delete", srv.deleteAccount)

	return &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
}

func (s *HTTPServer) createAccount(w http.ResponseWriter, r *http.Request) {
	fields := map[string]HttpPostFieldDataType{
		"name":            HttpPostFieldTypeString,
		"description":     HttpPostFieldTypeString,
		"type":            HttpPostFieldTypeString,
		"initial_balance": HttpPostFieldTypeF64,
	}
	postData, postDataParseErr := parsePostRequestData(w, r, fields)
	if !postDataParseErr {
		return
	}
	nameStr, _ := postData["name"].(string)
	descriptionStr, _ := postData["description"].(string)
	accountTypeStr, _ := postData["type"].(string)
	initialBalanceF64, _ := postData["initial_balance"].(float64)
	createRes, _ := data.AccountCreate(nameStr, descriptionStr, initialBalanceF64, accountTypeStr)
	switch createRes {
	case data.AS_Ok:
		s.broadcastSSE([]sseEv{{eventType: "sseEvPatchElements", eventData: app.Accounts()}})
		writeJsonResponse(w, http.StatusAccepted, HttpServerResponseDTO{
			Succes:  true,
			Message: "Created successfully",
		})
		return
	case data.AS_CreateBadInput:
		writeJsonResponse(w, http.StatusBadRequest, HttpServerResponseDTO{
			Succes:  false,
			Message: "Invalid input",
		})
		return
	default:
		writeJsonResponse(w, http.StatusInternalServerError, HttpServerResponseDTO{
			Succes:  false,
			Message: "Some uncontrolled error has ocurred",
		})
		return
	}
}

func (s *HTTPServer) deleteAccount(w http.ResponseWriter, r *http.Request) {
	fields := map[string]HttpPostFieldDataType{
		"id": HttpPostFieldTypeString, // parse as string first, then to int64
	}
	postData, ok := parsePostRequestData(w, r, fields)
	if !ok {
		return
	}
	idStr, _ := postData["id"].(string)
	id, err := strconv.ParseInt(strings.TrimSpace(idStr), 10, 64)
	if err != nil {
		writeJsonResponse(w, http.StatusBadRequest, HttpServerResponseDTO{
			Succes:  false,
			Message: "Invalid account ID",
		})
		return
	}

	deleteRes := data.AccountDelete(id)
	switch deleteRes {
	case data.AS_Ok:
		s.broadcastSSE([]sseEv{{eventType: "sseEvPatchElements", eventData: app.Accounts()}})
		writeJsonResponse(w, http.StatusAccepted, HttpServerResponseDTO{
			Succes:  true,
			Message: "Deleted successfully",
		})
		return
	case data.AS_AccountNotFound:
		writeJsonResponse(w, http.StatusNotFound, HttpServerResponseDTO{
			Succes:  false,
			Message: "Account not found",
		})
		return
	default:
		writeJsonResponse(w, http.StatusInternalServerError, HttpServerResponseDTO{
			Succes:  false,
			Message: "Some uncontrolled error has ocurred",
		})
		return
	}
}

func (s *HTTPServer) updateAccount(w http.ResponseWriter, r *http.Request) {
	fields := map[string]HttpPostFieldDataType{
		"id":              HttpPostFieldTypeString,
		"name":            HttpPostFieldTypeString,
		"description":     HttpPostFieldTypeString,
		"type":            HttpPostFieldTypeString,
		"initial_balance": HttpPostFieldTypeF64,
	}
	postData, ok := parsePostRequestData(w, r, fields)
	if !ok {
		return
	}

	idStr, _ := postData["id"].(string)
	id, err := strconv.ParseInt(strings.TrimSpace(idStr), 10, 64)
	if err != nil {
		writeJsonResponse(w, http.StatusBadRequest, HttpServerResponseDTO{
			Succes:  false,
			Message: "Invalid account ID",
		})
		return
	}

	nameStr, _ := postData["name"].(string)
	descriptionStr, _ := postData["description"].(string)
	accountTypeStr, _ := postData["type"].(string)
	initialBalanceF64, _ := postData["initial_balance"].(float64)

	updateRes, _ := data.AccountUpdate(id, nameStr, descriptionStr, initialBalanceF64, accountTypeStr)
	switch updateRes {
	case data.AS_Ok:
		s.broadcastSSE([]sseEv{{eventType: "sseEvPatchElements", eventData: app.Accounts()}})
		writeJsonResponse(w, http.StatusAccepted, HttpServerResponseDTO{
			Succes:  true,
			Message: "Updated successfully",
		})
		return
	case data.AS_AccountNotFound:
		writeJsonResponse(w, http.StatusNotFound, HttpServerResponseDTO{
			Succes:  false,
			Message: "Account not found",
		})
		return
	case data.AS_CreateBadInput:
		writeJsonResponse(w, http.StatusBadRequest, HttpServerResponseDTO{
			Succes:  false,
			Message: "Invalid input",
		})
		return
	default:
		writeJsonResponse(w, http.StatusInternalServerError, HttpServerResponseDTO{
			Succes:  false,
			Message: "Some uncontrolled error has ocurred",
		})
		return
	}
}

func (s *HTTPServer) broadcastSSE(events []sseEv) {
	s.mu.Lock()
	activeConnections := make([]DatastarSSEStream, 0, len(s.sseConnections))
	for connectionID, connection := range s.sseConnections {
		select {
		case <-connection.done:
			delete(s.sseConnections, connectionID)
		default:
			activeConnections = append(activeConnections, connection)
		}
	}
	s.mu.Unlock()

	for _, connection := range activeConnections {
		for _, event := range events {
			if event.eventType == "sseEvPatchElements" {
				_ = connection.sse.PatchElements(event.eventData)
			} else {
				_ = connection.sse.PatchSignals([]byte(event.eventData))
			}
		}
	}
}

func parsePostRequestData(w http.ResponseWriter, r *http.Request, fields map[string]HttpPostFieldDataType) (map[string]any, bool) {
	if r.Method != http.MethodPost {
		writeJsonResponse(w, http.StatusMethodNotAllowed, HttpServerResponseDTO{
			Succes:  false,
			Message: "method_not_allowed",
		})
		return map[string]any{}, false
	}
	if parseErr := r.ParseForm(); parseErr != nil {
		writeJsonResponse(w, http.StatusBadRequest, HttpServerResponseDTO{
			Succes:  false,
			Message: "The form body is invalid",
		})
		return map[string]any{}, false
	}
	result := map[string]any{}
	for field, fieldType := range fields {
		value := strings.TrimSpace(r.FormValue(field))
		switch fieldType {
		case HttpPostFieldTypeString:
			result[field] = value
		case HttpPostFieldTypeF64:
			v, err := strconv.ParseFloat(value, 64)
			if err != nil {
				writeJsonResponse(w, http.StatusBadRequest, HttpServerResponseDTO{
					Succes:  false,
					Message: fmt.Sprintf("%s must be a float64", field),
				})
				return map[string]any{}, false
			}
			result[field] = v
		default:
			result[field] = value
		}
	}
	return result, true
}

func writeJsonResponse(w http.ResponseWriter, statusCode int, response HttpServerResponseDTO) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(response)
}

func (s *HTTPServer) serveStaticFile(w http.ResponseWriter, r *http.Request, filepath string, filetype string) {
	switch filetype {
	case "html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case "css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	}
	http.ServeFile(w, r, filepath)
}
