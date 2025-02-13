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
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gorilla/websocket"
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
	ID               string `bson:"_id,omitempty" json:"id"`
	Email            string `bson:"email" json:"email"`
	Username         string `bson:"username" json:"username"`
	Password         string `bson:"password,omitempty" json:"-"`
	Role             string `bson:"role" json:"role"`
	Verified         bool   `bson:"verified" json:"verified"`
	VerificationCode string `bson:"verificationCode,omitempty" json:"-"`
}
type Chat struct {
	ChatID    string    `bson:"chat_id" json:"chat_id"`
	UserID    string    `bson:"user_id" json:"user_id"`
	AdminID   string    `bson:"admin_id,omitempty" json:"admin_id,omitempty"`
	Status    string    `bson:"status" json:"status"`
	Messages  []Message `bson:"messages" json:"messages"`
	CreatedAt time.Time `bson:"created_at" json:"created_at"`
}

type Message struct {
	Sender    string    `json:"sender" bson:"sender"`
	Content   string    `json:"content" bson:"content"`
	Timestamp time.Time `json:"timestamp" bson:"timestamp"`
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

var chatCollection *mongo.Collection

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
	chatCollection = db.Collection("chats")
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

func processPayment(cart []struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Quantity int     `json:"quantity"`
}) (*PaymentResponse, error) {
	paymentURL := "http://localhost:8082/pay"

	// Convert cart items to payment request format
	items := make([]CartItem, len(cart))
	for i, item := range cart {
		items[i] = CartItem{
			Name:     item.Name,
			Price:    item.Price,
			Quantity: item.Quantity,
		}
	}

	paymentReq := PaymentRequest{
		Currency: "USD",
		Items:    items,
	}

	// Convert request to JSON
	jsonData, err := json.Marshal(paymentReq)
	if err != nil {
		return nil, fmt.Errorf("error marshaling payment request: %v", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", paymentURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Make the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making payment request: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	var paymentResp PaymentResponse
	if err := json.NewDecoder(resp.Body).Decode(&paymentResp); err != nil {
		return nil, fmt.Errorf("error decoding payment response: %v", err)
	}
	log.Println(paymentResp)

	return &paymentResp, nil
}

// Updated handleCart function with payment processing
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

		// Process payment
		paymentResp, err := processPayment(cart)
		if err != nil {
			http.Error(w, fmt.Sprintf("Payment processing failed: %v", err), http.StatusInternalServerError)
			return
		}

		// Only proceed with order if payment was successful
		if paymentResp.Status == "success" {
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
				"items":         orderItems,
				"createdAt":     time.Now(),
				"totalAmount":   paymentResp.TotalAmount,
				"currency":      paymentResp.Currency,
				"paymentStatus": paymentResp.Status,
			}

			_, err := ordersCollection.InsertOne(context.TODO(), orderDocument)
			if err != nil {
				http.Error(w, "Failed to save order to orders collection", http.StatusInternalServerError)
				return
			}
			log.Println(orderDocument)

			json.NewEncoder(w).Encode(map[string]interface{}{
				"message":       "Order successfully processed!",
				"paymentStatus": paymentResp.Status,
				"totalAmount":   paymentResp.TotalAmount,
				"currency":      paymentResp.Currency,
			})
		} else {
			// Payment wasn't successful
			json.NewEncoder(w).Encode(map[string]interface{}{
				"message":       "Payment processing failed",
				"paymentStatus": paymentResp.Status,
				"totalAmount":   paymentResp.TotalAmount,
				"currency":      paymentResp.Currency,
			})
		}

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func ProtectedHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Verify your email"))
}
func getAllUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var users []User
	usersCollection := collection.Database().Collection("users")

	cursor, err := usersCollection.Find(context.TODO(), bson.M{})
	if err != nil {
		http.Error(w, "Failed to fetch users", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	for cursor.Next(context.TODO()) {
		var user User
		if err := cursor.Decode(&user); err != nil {
			http.Error(w, "Error decoding user", http.StatusInternalServerError)
			return
		}
		users = append(users, user)
	}

	json.NewEncoder(w).Encode(users)
}
func updateUserRole(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID := strings.TrimPrefix(r.URL.Path, "/api/users/")
	userID = strings.TrimSuffix(userID, "/role")

	log.Printf("Updating role for userID: %s", userID)

	objID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		log.Printf("Invalid ObjectID: %v", err)
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	var updateData struct {
		Role string `json:"role"`
	}

	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
		log.Printf("Error decoding JSON: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	usersCollection := collection.Database().Collection("users")

	filter := bson.M{"_id": objID}
	update := bson.M{"$set": bson.M{"role": updateData.Role}}

	res, err := usersCollection.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		log.Printf("MongoDB UpdateOne error: %v", err)
		http.Error(w, "Failed to update role", http.StatusInternalServerError)
		return
	}

	if res.MatchedCount == 0 {
		log.Printf("User not found for ID: %s", userID)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"message": "User role updated"})
}
func createChat(userID string) (string, error) {
	chatID := primitive.NewObjectID().Hex() // Генерация уникального ID чата

	chat := Chat{
		ChatID:    chatID,
		UserID:    userID,
		Status:    "active",
		Messages:  []Message{},
		CreatedAt: time.Now(),
	}

	_, err := chatCollection.InsertOne(context.TODO(), chat)
	if err != nil {
		return "", fmt.Errorf("failed to create chat: %v", err)
	}

	return chatID, nil
}
func sendMessage(chatID string, sender string, content string) error {
	message := Message{
		Sender:    sender,
		Content:   content,
		Timestamp: time.Now(),
	}

	filter := bson.M{"chat_id": chatID}
	update := bson.M{"$push": bson.M{"messages": message}}

	_, err := chatCollection.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		return fmt.Errorf("failed to send message: %v", err)
	}

	return nil
}
func closeChat(chatID string) error {
	filter := bson.M{"chat_id": chatID}
	update := bson.M{"$set": bson.M{"status": "inactive"}}

	result, err := chatCollection.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		return fmt.Errorf("failed to close chat: %v", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("chat with ID %s not found", chatID)
	}

	log.Printf("Chat %s successfully closed", chatID)
	return nil
}

func getChatHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	chatID := r.URL.Query().Get("chat_id")
	if chatID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "chat_id is required"})
		return
	}

	filter := bson.M{"chat_id": chatID}
	var chat Chat
	err := chatCollection.FindOne(context.TODO(), filter).Decode(&chat)
	if err == mongo.ErrNoDocuments {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Chat not found"})
		return
	} else if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Если у чата нет сообщений, возвращаем пустой массив
	if chat.Messages == nil {
		chat.Messages = []Message{}
	}

	// Возвращаем массив сообщений
	json.NewEncoder(w).Encode(chat.Messages)
}

var clients = make(map[*websocket.Conn]bool)      // Все клиенты
var adminClients = make(map[*websocket.Conn]bool) // Админы

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func checkChatExists(chatID string) (bool, error) {
	count, err := chatCollection.CountDocuments(context.TODO(), bson.M{"chat_id": chatID})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer ws.Close()

	// Добавляем в список подключенных клиентов
	clients[ws] = true

	for {
		var msg map[string]string
		err := ws.ReadJSON(&msg)
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			delete(clients, ws)
			break
		}

		switch msg["type"] {
		case "check_chat":
			chatID := msg["chat_id"]
			exists, err := checkChatExists(chatID)
			if err != nil {
				log.Printf("Ошибка при проверке чата: %v", err)
			}
			ws.WriteJSON(map[string]interface{}{
				"type":    "chat_status",
				"chat_id": chatID,
				"exists":  exists,
			})

		case "create_chat":
			chatID, err := createChat(msg["user_id"])
			if err != nil {
				log.Printf("Failed to create chat: %v", err)
				continue
			}
			ws.WriteJSON(map[string]string{"type": "chat_created", "chat_id": chatID})

		case "send_message":
			err := sendMessage(msg["chat_id"], msg["sender"], msg["content"])
			if err != nil {
				log.Printf("Failed to send message: %v", err)
				continue
			}
			broadcastMessage(map[string]string{
				"type":    "new_message",
				"chat_id": msg["chat_id"],
				"sender":  msg["sender"],
				"content": msg["content"],
			})
		case "close_chat":
			chatID := msg["chat_id"]
			err := closeChat(chatID)
			if err != nil {
				log.Printf("Failed to close chat: %v", err)
				continue
			}

			response := map[string]string{
				"type":    "chat_closed",
				"chat_id": chatID,
			}

			broadcastMessage(response)

		}

	}
}

// Функция рассылки сообщения всем клиентам (всем админам и пользователю)
func broadcastMessage(msg map[string]string) {
	messageJSON, _ := json.Marshal(msg)

	for client := range clients {
		err := client.WriteMessage(websocket.TextMessage, messageJSON)
		if err != nil {
			log.Printf("Ошибка отправки сообщения клиенту: %v", err)
			client.Close()
			delete(clients, client)
		}
	}
}

// Получение активных чатов для админа
func getActiveChats(w http.ResponseWriter, r *http.Request) {
	filter := bson.M{"status": "active"}
	cursor, err := chatCollection.Find(context.TODO(), filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var chats []Chat
	if err = cursor.All(context.TODO(), &chats); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("Active chats: %+v", chats)

	json.NewEncoder(w).Encode(chats)
}

// Проверка активного чата для пользователя
func getActiveChat(w http.ResponseWriter, r *http.Request) {
	userID := "USER_ID_FROM_SESSION" // Получать из аутентификации
	filter := bson.M{"user_id": userID, "status": "active"}

	var chat Chat
	err := chatCollection.FindOne(context.TODO(), filter).Decode(&chat)

	if err == mongo.ErrNoDocuments {
		json.NewEncoder(w).Encode(map[string]interface{}{"active": false})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"active":  true,
		"chat_id": chat.ChatID,
	})
}

func main() {
	initPaths()
	connectMongoDB()

	limiter := rate.NewLimiter(2, 5)

	fs := http.FileServer(http.Dir(staticDir))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.Handle("/", auth.NoCacheMiddleware(auth.AuthMiddleware(rateLimiter(http.HandlerFunc(auth.DashboardHandler), limiter))))
	http.Handle("/user", auth.NoCacheMiddleware(auth.AuthMiddleware(http.HandlerFunc(serveUser))))
	http.Handle("/dashboard", auth.NoCacheMiddleware(auth.AuthMiddleware(rateLimiter(http.HandlerFunc(auth.DashboardHandler), limiter))))
	http.Handle("/protected", auth.NoCacheMiddleware(auth.AuthMiddleware(http.HandlerFunc(ProtectedHandler))))
	http.Handle("/admin", auth.NoCacheMiddleware(auth.AuthMiddleware(http.HandlerFunc(serveAdmin))))

	http.Handle("/login", http.HandlerFunc(auth.LoginHandler))
	http.Handle("/register", http.HandlerFunc(auth.RegisterHandler))
	http.Handle("/logout", http.HandlerFunc(auth.LogoutHandler))
	http.Handle("/verify", http.HandlerFunc(auth.VerifyHandler))

	http.HandleFunc("/send_email", sendEmailHandler)
	http.HandleFunc("/get_users_email_list", getUsersEmailList)

	http.HandleFunc("/products", handleProducts)
	http.HandleFunc("/cart", handleCart)

	http.HandleFunc("/users", getAllUsers)
	http.HandleFunc("/api/users/", updateUserRole)

	http.HandleFunc("/ws", handleConnections)
	http.HandleFunc("/api/active-chats", getActiveChats)
	http.HandleFunc("/api/active-chat", getActiveChat)
	http.HandleFunc("/api/chat-history", getChatHistory)
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
