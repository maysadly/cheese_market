package main

import (
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
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/PuerkitoBio/goquery"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/time/rate"
	"gopkg.in/gomail.v2"
)

var (
	collection    *mongo.Collection
	templateDir   string
	staticDir     string
	uploadTempDir string
)

type Product struct {
	ID       string  `json:"id" bson:"_id,omitempty"`
	Name     string  `json:"name" bson:"name"`
	Price    float64 `json:"price" bson:"price"`
	Category string  `json:"category" bson:"category"`
}
type User struct {
	ID    string `json:"id" bson:"_id"`
	Email string `json:"email" bson:"email"`
}

func initPaths() {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current working directory: %v", err)
	}

	// Set paths relative to the current working directory
	templateDir = filepath.Join(cwd, "templates")
	staticDir = filepath.Join(cwd, "static")
	uploadTempDir = filepath.Join(cwd, "temp_uploads")

	// Create temp directory if not exists
	if _, err := os.Stat(uploadTempDir); os.IsNotExist(err) {
		err := os.Mkdir(uploadTempDir, 0755)
		if err != nil {
			log.Fatalf("Failed to create temp upload directory: %v", err)
		}
	}
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

// Serves static HTML files from the templates directory
func serveHTML(w http.ResponseWriter, r *http.Request, filename string) {
	filePath := filepath.Join(templateDir, filename)
	http.ServeFile(w, r, filePath)
}

func serveAdmin(w http.ResponseWriter, r *http.Request) {
	serveHTML(w, r, "admin.html")
}

func serveUser(w http.ResponseWriter, r *http.Request) {
	serveHTML(w, r, "user.html")
}

func serveLogin(w http.ResponseWriter, r *http.Request) {
	serveHTML(w, r, "login.html")
}

func serveRegistration(w http.ResponseWriter, r *http.Request) {
	serveHTML(w, r, "register.html")
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
			filter["category"] = category
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

func fetchEmailDataFromPage(url string) (string, string, string, []byte, string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", "", "", nil, "", fmt.Errorf("failed to fetch data from %s: %w", url, err)
	}
	defer resp.Body.Close()

	// Ensure the response status is OK
	if resp.StatusCode != http.StatusOK {
		return "", "", "", nil, "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", "", "", nil, "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Extract email fields from the HTML
	to := doc.Find("#to").Text()
	subject := doc.Find("#subject").Text()
	body := doc.Find("#message").Text()

	// Extract the file URL from the HTML
	fileElement := doc.Find("#file")
	fileURL, exists := fileElement.Attr("data-url")
	if !exists {
		return "", "", "", nil, "", fmt.Errorf("file URL not found in HTML")
	}

	// Fetch the file data from the URL
	fileData, fileName, err := fetchFile(fileURL)
	if err != nil {
		return "", "", "", nil, "", fmt.Errorf("failed to fetch file: %w", err)
	}

	return to, subject, body, fileData, fileName, nil
}

func fetchFile(url string) ([]byte, string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch file from %s: %w", url, err)
	}
	defer resp.Body.Close()

	// Ensure the response status is OK
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read the file data from the response body
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read file data: %w", err)
	}

	// Extract the file name from the URL
	parts := strings.Split(url, "/")
	fileName := parts[len(parts)-1]

	return data, fileName, nil
}

type EmailPayload struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
	File    struct {
		Filename string `json:"filename"`
		Content  string `json:"content"`
	} `json:"file"`
}

func sendEmail(to, subject, body string, fileData []byte, fileName string) error {
	smtpHost := "smtp.office365.com"
	smtpPort := 587
	username := "230047@astanait.edu.kz"
	password := "aRBmKl1O0G0kw"

	m := gomail.NewMessage()
	m.SetHeader("From", username)
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/plain", body)

	if len(fileData) > 0 {
		tempFile, err := ioutil.TempFile("", fileName)
		if err != nil {
			return fmt.Errorf("failed to create temp file: %w", err)
		}
		defer os.Remove(tempFile.Name())

		if _, err := tempFile.Write(fileData); err != nil {
			return fmt.Errorf("failed to write to temp file: %w", err)
		}
		if err := tempFile.Close(); err != nil {
			return fmt.Errorf("failed to close temp file: %w", err)
		}

		m.Attach(tempFile.Name(), gomail.Rename(fileName))
	}

	d := gomail.NewDialer(smtpHost, smtpPort, username, password)

	if err := d.DialAndSend(m); err != nil {
		log.Printf("Error sending email: %v", err)
		return fmt.Errorf("failed to send email: %w", err)
	}

	log.Printf("Email sent successfully to: %s", to)
	return nil
}

func sendEmailHandler(w http.ResponseWriter, r *http.Request) {
	var payload EmailPayload
	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		log.Printf("Error decoding JSON: %v", err)
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	fileData, err := base64.StdEncoding.DecodeString(payload.File.Content)
	if err != nil {
		log.Printf("Error decoding file content: %v", err)
		http.Error(w, "Failed to decode file content", http.StatusInternalServerError)
		return
	}

	log.Printf("Sending email to: %s", payload.To)
	err = sendEmail(payload.To, payload.Subject, payload.Body, fileData, payload.File.Filename)
	if err != nil {
		log.Printf("Error sending email: %v", err)
		http.Error(w, "Failed to send email", http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"status": "Email sent successfully!",
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response JSON: %v", err)
		http.Error(w, "Failed to encode response JSON", http.StatusInternalServerError)
	}

	log.Printf("Email sent successfully!")
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
func getUsersEmailList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	usersCollection := collection.Database().Collection("users")

	cursor, err := usersCollection.Find(context.TODO(), bson.M{})
	if err != nil {
		http.Error(w, "Failed to fetch users", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	var users []User
	for cursor.Next(context.TODO()) {
		var user User
		if err := cursor.Decode(&user); err != nil {
			http.Error(w, "Failed to decode user data", http.StatusInternalServerError)
			return
		}
		users = append(users, user)
	}

	if err := cursor.Err(); err != nil {
		http.Error(w, "Cursor error", http.StatusInternalServerError)
		return
	}

	emails := []string{}
	for _, user := range users {
		emails = append(emails, user.Email)
	}

	json.NewEncoder(w).Encode(emails)
}
func handleCart(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodPost:
		var cart []struct {
			ID       string  `json:"id"`
			Name     string  `json:"name"`
			Price    float64 `json:"price"`
			Quantity int     `json:"quantity"`
		}

		if err := json.NewDecoder(r.Body).Decode(&cart); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		fmt.Printf("Users cart: %+v\n", cart)

		db := collection.Database()
		ordersCollection := db.Collection("orders")

		var orderItems []interface{}
		for _, item := range cart {
			orderItem := Product{
				ID:       item.ID,
				Name:     item.Name,
				Price:    item.Price,
				Category: "General",
			}
			orderItems = append(orderItems, orderItem)
		}

		orderDocument := bson.M{
			"items":     orderItems,
			"createdAt": time.Now(),
		}

		_, err := ordersCollection.InsertOne(context.TODO(), orderDocument)
		if err != nil {
			http.Error(w, "Failed to save order to orders collection", http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"message": "Order successfully processed!"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func ProtectedHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Verify your email"))
}

func main() {
	initPaths()
	connectMongoDB()

	limiter := rate.NewLimiter(2, 5)

	fs := http.FileServer(http.Dir(staticDir))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.Handle("/", auth.AuthMiddleware(rateLimiter(http.HandlerFunc(auth.DashboardHandler), limiter)))
	http.Handle("/user", auth.AuthMiddleware(http.HandlerFunc(serveUser)))
	http.Handle("/dashboard", auth.AuthMiddleware(rateLimiter(http.HandlerFunc(auth.DashboardHandler), limiter)))
	http.Handle("/protected", auth.AuthMiddleware(http.HandlerFunc(ProtectedHandler)))
	http.Handle("/admin", auth.AuthMiddleware(http.HandlerFunc(serveAdmin)))

	http.Handle("/login", http.HandlerFunc(auth.LoginHandler))
	http.Handle("/register", http.HandlerFunc(auth.RegisterHandler))
	http.Handle("/logout", http.HandlerFunc(auth.LogoutHandler))
	http.Handle("/verify", http.HandlerFunc(auth.VerifyHandler))

	http.HandleFunc("/send_email", sendEmailHandler)
	http.HandleFunc("/get_users_email_list", getUsersEmailList)

	http.HandleFunc("/products", handleProducts)
	http.HandleFunc("/cart", handleCart)

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
