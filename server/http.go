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
	sseConnections map[int64]sseConnection
	nextSSEID      int64
}

type sseConnection struct {
	stream interface {
		PatchElements(string, ...datastar.PatchElementOption) error
	}
	done <-chan struct{}
}

func NewHTTPServer() *http.Server {
	srv := &HTTPServer{
		sseConnections: map[int64]sseConnection{},
	}
	mux := http.NewServeMux()

	mux.HandleFunc("/", srv.firstRender)
	mux.HandleFunc("/sse", srv.sse)
	mux.HandleFunc("/accounts/new", srv.createAccount)
	mux.HandleFunc("/accounts/delete", srv.deleteAccount)
	mux.HandleFunc("/datastar.js", datastarJS)
	mux.HandleFunc("/main.css", mainCss)

	return &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
}

func writeJsonResponse(w http.ResponseWriter, statusCode int, response HttpServerResponseDTO) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(response)
}

func (s *HTTPServer) firstRender(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, app.Layout())
}

func (s *HTTPServer) sse(w http.ResponseWriter, r *http.Request) {
	stream := datastar.NewSSE(w, r)
	connectionID := s.addSSEConnection(stream, r.Context().Done())
	defer s.removeSSEConnection(connectionID)

	<-r.Context().Done()
}

func (s *HTTPServer) addSSEConnection(stream interface {
	PatchElements(string, ...datastar.PatchElementOption) error
}, done <-chan struct{}) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextSSEID++
	connectionID := s.nextSSEID
	s.sseConnections[connectionID] = sseConnection{stream: stream, done: done}

	return connectionID
}

func (s *HTTPServer) removeSSEConnection(connectionID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.sseConnections, connectionID)
}

func (s *HTTPServer) broadcastToSSEConnections(message string) {
	s.mu.Lock()
	activeConnections := make([]sseConnection, 0, len(s.sseConnections))

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
		_ = connection.stream.PatchElements(message)
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
		s.broadcastToSSEConnections(app.AccountsTable())
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
		s.broadcastToSSEConnections(app.AccountsTable())
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

func parsePostRequestData(w http.ResponseWriter, r *http.Request, fields map[string]HttpPostFieldDataType) (map[string]any, bool) {
	// Add some basic logging
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

func datastarJS(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "datastar.js")
}

func mainCss(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	http.ServeFile(w, r, "main.css")
}
