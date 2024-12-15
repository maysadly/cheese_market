package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type Request struct {
	Message string `json:"message"`
}

type Response struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)

		json.NewEncoder(w).Encode(Response{
			Status:  "fail",
			Message: "Method not allowed. Only GET and POST are supported.",
		})
		return
	}

	var requestData Request
	err := json.NewDecoder(r.Body).Decode(&requestData)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{
			Status:  "fail",
			Message: "Invalid or empty JSON message.",
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(Response{
		Status:  "success",
		Message: "Data successfully received",
	})

}

func main() {
	http.HandleFunc("/", handleRequest)
	fmt.Println("Server is running")
	http.ListenAndServe("localhost:8080", nil)
}
