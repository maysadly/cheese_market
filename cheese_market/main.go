package main

import (
	"bytes"
	"cheese_market/auth"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var collection *mongo.Collection

type Product struct {
	ID       string  `json:"id" bson:"_id,omitempty"`
	Name     string  `json:"name" bson:"name"`
	Price    float64 `json:"price" bson:"price"`
	Category string  `json:"category" bson: "category"`
}

func connectMongoDB() {
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")

	client, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	err = client.Ping(context.TODO(), nil)
	if err != nil {
		log.Fatalf("MongoDB connection failed: %v", err)
	}

	fmt.Println("Connected to MongoDB!")

	db := client.Database("cheeseMarket")
	collection = db.Collection("products")
}

func serveHTML(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "templates/admin.html")
}

func serveUser(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "templates/user.html")
}

func serveLogin(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "templates/login.html")
}

func serveRegistration(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "templates/register.html")
}

func handleProducts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		id := r.URL.Query().Get("id")
		sortBy := r.URL.Query().Get("sortBy")
		order := r.URL.Query().Get("order")
		page := r.URL.Query().Get("page")
		pageSize := r.URL.Query().Get("pageSize")
		category := r.URL.Query().Get("category")

		filter := bson.M{}

		if id != "" {
			objectID, err := primitive.ObjectIDFromHex(id)
			if err != nil {
				http.Error(w, "Invalid ID format", http.StatusBadRequest)
				return
			}
			filter["_id"] = objectID
		}

		if category != "" {
			filter["category"] = category // Добавлено: фильтр по категории
		}

		findOptions := options.Find()

		// Sorting
		if sortBy != "" {
			sortOrder := 1
			if order == "desc" {
				sortOrder = -1
			}
			findOptions.SetSort(bson.D{{Key: sortBy, Value: sortOrder}})
		}

		// Pagination
		p, _ := strconv.Atoi(page)
		ps, _ := strconv.Atoi(pageSize)
		if p < 1 {
			p = 1
		}
		if ps < 1 {
			ps = 10
		}
		skip := (p - 1) * ps
		findOptions.SetSkip(int64(skip))
		findOptions.SetLimit(int64(ps))

		// Getting products
		cursor, err := collection.Find(context.TODO(), filter, findOptions)
		if err != nil {
			http.Error(w, "Failed to fetch data", http.StatusInternalServerError)
			return
		}
		defer cursor.Close(context.TODO())

		var products []Product
		for cursor.Next(context.TODO()) {
			var product Product
			if err = cursor.Decode(&product); err != nil {
				http.Error(w, "Failed to decode data", http.StatusInternalServerError)
				return
			}
			products = append(products, product)
		}

		if err := cursor.Err(); err != nil {
			http.Error(w, "Error during cursor iteration", http.StatusInternalServerError)
			return
		}

		if len(products) == 0 {
			http.Error(w, "No products match the filter", http.StatusNotFound)
			return
		}

		totalCount, err := collection.CountDocuments(context.TODO(), filter)
		if err != nil {
			http.Error(w, "Failed to fetch total count", http.StatusInternalServerError)
			return
		}

		response := map[string]interface{}{
			"products":   products,
			"total":      totalCount,
			"page":       p,
			"pageSize":   ps,
			"totalPages": (totalCount + int64(ps) - 1) / int64(ps),
		}

		json.NewEncoder(w).Encode(response)

	case http.MethodPost:
		var product Product
		err := json.NewDecoder(r.Body).Decode(&product)
		if err != nil {
			http.Error(w, "Failed to decode request body", http.StatusBadRequest)
			return
		}
		_, err = collection.InsertOne(context.TODO(), product)
		if err != nil {
			http.Error(w, "Failed to insert data", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"message": "Product added successfully!"})

	case http.MethodPut:
		var payload struct {
			ID       string  `json:"id"`
			Name     string  `json:"name"`
			Price    float64 `json:"price"`
			Category string  `json:"category"`
		}

		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Failed to decode request body", http.StatusBadRequest)
			return
		}

		objectID, err := primitive.ObjectIDFromHex(payload.ID)
		if err != nil {
			http.Error(w, "Invalid ID format", http.StatusBadRequest)
			return
		}

		filter := bson.M{"_id": objectID}
		update := bson.M{
			"$set": bson.M{
				"name":     payload.Name,
				"price":    payload.Price,
				"category": payload.Category,
			},
		}

		result, err := collection.UpdateOne(context.TODO(), filter, update)
		if err != nil {
			http.Error(w, "Failed to update product", http.StatusInternalServerError)
			return
		}

		if result.MatchedCount == 0 {
			http.Error(w, "Product not found", http.StatusNotFound)
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"message": "Product updated successfully!"})

	case http.MethodDelete:
		var payload struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Failed to decode request body", http.StatusBadRequest)
			return
		}

		objectID, err := primitive.ObjectIDFromHex(payload.ID)
		if err != nil {
			http.Error(w, "Invalid ID format", http.StatusBadRequest)
			return
		}

		filter := bson.M{"_id": objectID}
		_, err = collection.DeleteOne(context.TODO(), filter)
		if err != nil {
			http.Error(w, "Failed to delete product", http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"message": "Product deleted successfully!"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleSendEmail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Struct to hold the incoming JSON payload
	var payload struct {
		To      string `json:"to"`
		Subject string `json:"subject"`
		Body    string `json:"body"`
		File    struct {
			Filename string `json:"filename"`
			Content  string `json:"content"` // Base64-encoded content
		} `json:"file"`
	}

	// Decode the JSON payload
	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		http.Error(w, "Failed to decode request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if payload.To == "" || payload.Subject == "" || payload.Body == "" {
		http.Error(w, "Fields 'to', 'subject', and 'body' are required", http.StatusBadRequest)
		return
	}

	// Decode the file content from Base64
	fileContent, err := base64.StdEncoding.DecodeString(payload.File.Content)
	if err != nil {
		http.Error(w, "Failed to decode file content", http.StatusBadRequest)
		return
	}

	// Write the file to a temporary location (optional)
	tempFilePath := fmt.Sprintf("/tmp/%s", payload.File.Filename)
	err = ioutil.WriteFile(tempFilePath, fileContent, 0644)
	if err != nil {
		http.Error(w, "Failed to write file", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tempFilePath) // Clean up the file after sending

	// Simulate sending the email (replace with actual email logic)
	fmt.Printf("Email sent to: %s\nSubject: %s\nBody: %s\nAttached File: %s\n",
		payload.To, payload.Subject, payload.Body, tempFilePath)

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return
	}

	url := "http://localhost:8081/send_email"

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return
	}
	defer resp.Body.Close()

	// Respond to the client
	response := map[string]string{"message": "Email sent successfully with attachment!"}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func main() {
	connectMongoDB()

	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.Handle("/products", http.HandlerFunc(handleProducts))
	http.Handle("/", http.HandlerFunc(serveHTML))                      // admin.html page
	http.Handle("/user", http.HandlerFunc(serveUser))                  // user.html page
	http.Handle("/login", http.HandlerFunc(auth.LoginHandler))         // login page
	http.Handle("/register", http.HandlerFunc(auth.RegisterHandler))   // registration page
	http.Handle("/logout", http.HandlerFunc(auth.LogoutHandler))       // logout
	http.Handle("/dashboard", http.HandlerFunc(auth.DashboardHandler)) // after login
	http.HandleFunc("/send_email", handleSendEmail)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: nil,
	}

	fmt.Println("Server running on http://localhost:8080/login")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe(): %v", err)
		}
	}()

	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}
