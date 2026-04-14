package main

import (
	"gastos/src/db"
	"gastos/src/handlers"
	"gastos/src/middleware"
	"log"
	"net/http"
	"os"
)

func main() {
	db.Init("./gastos.db")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	mux := http.NewServeMux()

	// Auth (público)
	mux.HandleFunc("/api/auth/register", cors(handlers.Register))
	mux.HandleFunc("/api/auth/login", cors(handlers.Login))

	// Recursos protegidos
	mux.HandleFunc("/api/expenses", cors(middleware.Auth(handlers.Expenses)))
	mux.HandleFunc("/api/expenses/", cors(middleware.Auth(handlers.Expenses)))
	mux.HandleFunc("/api/incomes", cors(middleware.Auth(handlers.Incomes)))
	mux.HandleFunc("/api/incomes/", cors(middleware.Auth(handlers.Incomes)))
	mux.HandleFunc("/api/goals", cors(middleware.Auth(handlers.Goals)))
	mux.HandleFunc("/api/goals/", cors(middleware.Auth(handlers.Goals)))

	// Arquivos estáticos
	fs := http.FileServer(http.Dir("./src/web"))
	mux.Handle("/", fs)

	log.Printf("Servidor em :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func cors(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			return
		}
		h(w, r)
	}
}
