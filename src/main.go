package main

import (
	"os"
	"log"
	"net/http"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	mux := http.NewServeMux()

	// Serve arquivos da pasta web
	fs := http.FileServer(http.Dir("./src/web"))
	mux.Handle("/", fs)

	log.Printf("Servidor rodando na :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}