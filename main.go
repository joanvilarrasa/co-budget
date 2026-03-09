package main

import (
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/starfederation/datastar-go/datastar"
)

var count atomic.Int64

func main() {
	http.HandleFunc("/", home)
	http.HandleFunc("/increment", increment)
	http.HandleFunc("/datastar.js", datastarJS)
	http.HandleFunc("/main.css", mainCss)

	http.HandleFunc("/goto/dashboard", goToDashboard)
	http.HandleFunc("/goto/expenses", goToExpenses)

	fmt.Println("listening on http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}

func home(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeFile(w, r, "home.html")
}

func increment(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)
	next := count.Add(1)
	_ = sse.PatchSignals([]byte(fmt.Sprintf(`{"count": %d}`, next)))
}

func goToDashboard(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)
	sse.PatchElements(
		`<div id="page">Dashboard Page</div>`,
	)
	sse.PatchSignals([]byte(`{page: 'dashboard'}`))
}

func goToExpenses(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)
	sse.PatchElements(
		`<div id="page">Expenses Page</div>`,
	)
	sse.PatchSignals([]byte(`{page: 'expenses'}`))
}

func datastarJS(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "datastar.js")
}

func mainCss(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	http.ServeFile(w, r, "main.css")
}
