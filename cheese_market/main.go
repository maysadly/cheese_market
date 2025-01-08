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
	"golang.org/x/time/rate"
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

    // Handle preflight request (OPTIONS)
    if r.Method == http.MethodOptions {
        w.WriteHeader(http.StatusOK)
        return
    }

    // Handle non-POST requests
    if r.Method != http.MethodPost {
        http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
        return
    }

    // Define struct for incoming JSON payload
    var payload struct {
        To      string `json:"to"`
        Subject string `json:"subject"`
        Body    string `json:"body"`
        File    struct {
            Filename string `json:"filename"`
            Content  string `json:"content"` // Base64-encoded content
        } `json:"file"`
    }

    // Decode the incoming JSON payload
    err := json.NewDecoder(r.Body).Decode(&payload)
    if err != nil {
        http.Error(w, `{"error": "Failed to decode request body"}`, http.StatusBadRequest)
        return
    }

    // Validate required fields
    if payload.To == "" || payload.Subject == "" || payload.Body == "" {
        http.Error(w, `{"error": "Fields 'to', 'subject', and 'body' are required"}`, http.StatusBadRequest)
        return
    }

    // Decode file content from Base64
    fileContent, err := base64.StdEncoding.DecodeString(payload.File.Content)
    if err != nil {
        http.Error(w, `{"error": "Failed to decode file content"}`, http.StatusBadRequest)
        return
    }

    // Create a temporary file to store the uploaded file content
    tempFilePath := fmt.Sprintf("/tmp/%s", payload.File.Filename)
    err = ioutil.WriteFile(tempFilePath, fileContent, 0644)
    if err != nil {
        log.Printf("Error writing file %s: %v", tempFilePath, err)  // Log error for debugging
        http.Error(w, `{"error": "Failed to write file"}`, http.StatusInternalServerError)
        return
    }

    // Defer the file cleanup after the response is sent
    defer func() {
        if err := os.Remove(tempFilePath); err != nil {
            log.Printf("Failed to remove temp file %s: %v", tempFilePath, err)
        }
    }()

    // Simulate email sending (replace this with actual email-sending logic)
    fmt.Printf("Email sent to: %s\nSubject: %s\nBody: %s\nAttached File: %s\n",
        payload.To, payload.Subject, payload.Body, tempFilePath)

    // Marshal the payload to JSON for the internal API request
    jsonPayload, err := json.Marshal(payload)
    if err != nil {
        log.Printf("Failed to marshal JSON payload: %v", err)
        http.Error(w, `{"error": "Failed to process email data"}`, http.StatusInternalServerError)
        return
    }

    // URL of the internal API to send the email
    url := "http://localhost:8081/send_email"

    // Send a POST request to the internal email service
    resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonPayload))
    if err != nil {
        log.Printf("Error during POST request to send email: %v", err)
        http.Error(w, `{"error": "Failed to send email to internal service"}`, http.StatusInternalServerError)
        return
    }
    defer resp.Body.Close()

    // Respond to the client with a success message
    response := map[string]string{"message": "Email sent successfully with attachment!"}
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(response)
}


func rateLimiter(next http.Handler, limiter *rate.Limiter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow() {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	connectMongoDB()

	limiter := rate.NewLimiter(2, 5)

	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.Handle("/products", http.HandlerFunc(handleProducts))

	http.Handle("/", rateLimiter(http.HandlerFunc(serveHTML), limiter)) // admin.html page
	http.Handle("/user", rateLimiter(http.HandlerFunc(serveUser), limiter)) // user.html page
	http.Handle("/dashboard", rateLimiter(http.HandlerFunc(auth.DashboardHandler), limiter)) // after login

	http.Handle("/login", http.HandlerFunc(auth.LoginHandler))         // login page
	http.Handle("/register", http.HandlerFunc(auth.RegisterHandler))   // registration page
	http.Handle("/logout", http.HandlerFunc(auth.LogoutHandler))       // logout

	http.HandleFunc("/send_email", handleSendEmail)

	srv := &http.Server{
		Addr:         ":8080",
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
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