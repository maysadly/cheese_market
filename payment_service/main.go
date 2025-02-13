package main

import (
	"encoding/json"
	"log"
	"net/http"
)

type PaymentResponse struct {
	Status      string  `json:"status"`
	TotalAmount float64 `json:"total_amount"`
	Currency    string  `json:"currency"`
}

type CartItem struct {
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Quantity int     `json:"quantity"`
}

type PaymentRequest struct {
	Currency string     `json:"currency"`
	Items    []CartItem `json:"items"`
}

func handlePayment(w http.ResponseWriter, r *http.Request) {
	var req PaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Currency == "" || len(req.Items) == 0 {
		http.Error(w, "Missing currency or items", http.StatusBadRequest)
		return
	}

	totalAmount := 0.0
	for _, item := range req.Items {
		totalAmount += item.Price * float64(item.Quantity)
	}
	status := "success"

	response := PaymentResponse{
		Status:      status,
		TotalAmount: totalAmount,
		Currency:    req.Currency,
	}
	log.Println("Payment")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func main() {
	http.HandleFunc("/pay", handlePayment)
	log.Println("Payment simulation service running on port 8082")
	log.Fatal(http.ListenAndServe(":8082", nil))
}
