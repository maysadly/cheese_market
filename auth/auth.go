package auth

import (
	"context"
	"errors"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

var tpl *template.Template
var secretKey = []byte("mysecretkey")

var collection *mongo.Collection

type User struct {
	Email    string `bson:"email"`
	Username string `bson:"username"`
	Password string `bson:"password"`
	Role     string `bson:"role"`
}

type PageData struct {
	ErrorMessage string
}

type CustomClaims struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func init() {
	// Get absolute paths
	basePath, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("Failed to determine absolute path: %v", err)
	}

	// Setup templates with absolute paths
	templatePath := filepath.Join(basePath, "templates", "*.html")
	tpl, err = template.ParseGlob(templatePath)
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}

	// Setup logging to an absolute path
	logFilePath := filepath.Join(basePath, "server.log")
	file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	multiWriter := io.MultiWriter(file, os.Stdout)

	// Set the output for the logger
	log.SetOutput(multiWriter)

	// Optional: Customize log flags (e.g., include timestamps)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.SetOutput(file)
	log.Println("Logging to file:", logFilePath)

	// MongoDB connection setup
	clientOptions := options.Client().ApplyURI("mongodb://mongodb:27017")
	client, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}

	err = client.Ping(context.TODO(), nil)
	if err != nil {
		log.Fatalf("MongoDB connection failed: %v", err)
	}

	collection = client.Database("cheeseMarket").Collection("users")
	log.Println("Connected to MongoDB!")
}

func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	data := PageData{}
	if r.Method == http.MethodPost {
		r.ParseForm()
		email := r.FormValue("email")
		username := r.FormValue("username")
		password := r.FormValue("password")

		log.Printf("Attempt to register user: %s", username)

		var existingUser User
		err := collection.FindOne(context.TODO(), bson.M{"username": username}).Decode(&existingUser)
		if err == nil {
			data.ErrorMessage = "User with such login already exists"
			tpl.ExecuteTemplate(w, "register.html", data)
			log.Printf("Registration failed: User %s already exists", username)
			return
		} else if err != mongo.ErrNoDocuments {
			data.ErrorMessage = "Error checking user presence"
			tpl.ExecuteTemplate(w, "register.html", data)
			log.Printf("Error checking user presence for %s: %v", username, err)
			return
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			data.ErrorMessage = "Hashing password error"
			tpl.ExecuteTemplate(w, "register.html", data)
			log.Printf("Hashing password error for %s: %v", username, err)
			return
		}

		role := "user"
		count, _ := collection.CountDocuments(context.TODO(), bson.M{})
		if count == 0 {
			role = "admin"
		}

		user := User{
			Email:    email,
			Username: username,
			Password: string(hashedPassword),
			Role:     role,
		}

		_, err = collection.InsertOne(context.TODO(), user)
		if err != nil {
			data.ErrorMessage = "Registration error"
			tpl.ExecuteTemplate(w, "register.html", data)
			log.Printf("Registration error for user %s: %v", username, err)
			return
		}

		if role == "admin" {
			http.Redirect(w, r, "/admin", http.StatusSeeOther)
		} else {
			http.Redirect(w, r, "/user", http.StatusSeeOther)
		}
		log.Printf("User %s registered successfully with role %s", username, role)
		return
	}
	tpl.ExecuteTemplate(w, "register.html", data)
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	data := PageData{}
	if r.Method == http.MethodPost {
		r.ParseForm()
		username := r.FormValue("username")
		password := r.FormValue("password")

		log.Printf("Attempt to login user: %s", username)

		var user User
		err := collection.FindOne(context.TODO(), bson.M{"username": username}).Decode(&user)
		if err != nil {
			data.ErrorMessage = "Incorrect login or password"
			tpl.ExecuteTemplate(w, "login.html", data)
			log.Printf("Login failed for user %s: incorrect login or password", username)
			return
		}

		err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
		if err != nil {
			data.ErrorMessage = "Incorrect login or password"
			tpl.ExecuteTemplate(w, "login.html", data)
			log.Printf("Login failed for user %s: incorrect login or password", username)
			return
		}

		claims := CustomClaims{
			Username: username,
			Role:     user.Role,
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    "cheese",                                      // Issuer of the token
				Subject:   username,                                      // Subject (usually the user's ID or username)
				Audience:  []string{"cheese"},                            // Audience
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)), // Expiration time
				IssuedAt:  jwt.NewNumericDate(time.Now()),                // Issued at
				NotBefore: jwt.NewNumericDate(time.Now()),                // Not valid before
			},
		}

		// Create a new token with the claims
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

		tokenString, err := token.SignedString(secretKey)
		if err != nil {
			data.ErrorMessage = "Generation token error"
			tpl.ExecuteTemplate(w, "login.html", data)
			log.Printf("Token generation error for user %s: %v", username, err)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:  "token",
			Value: tokenString,
			Path:  "/",
		})
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		log.Printf("User %s logged in successfully", username)
		return
	}
	tpl.ExecuteTemplate(w, "login.html", data)
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:   "token",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
	log.Println("User logged out successfully")
}

func DashboardHandler(w http.ResponseWriter, r *http.Request) {
	tokenCookie, err := r.Cookie("token")
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		log.Println("Unauthorized access attempt to dashboard")
		return
	}

	tokenString := tokenCookie.Value
	claims := jwt.MapClaims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return secretKey, nil
	})

	if err != nil || !token.Valid {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		log.Println("Unauthorized access attempt to dashboard with invalid token")
		return
	}

	role := claims["role"].(string)

	if role == "admin" {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		log.Println("Admin user redirected to admin dashboard")
	} else {
		http.Redirect(w, r, "/user", http.StatusSeeOther)
		log.Println("Regular user redirected to user dashboard")
	}
}

func (c CustomClaims) Valid() error {
	return c.RegisteredClaims.Valid()
}

// VerifyJWTToken проверяет и валидирует токен из cookies
func VerifyJWTToken(r *http.Request) (*CustomClaims, error) {
	// Get the token from cookies
	tokenCookie, err := r.Cookie("token")
	if err != nil {
		return nil, errors.New("token not found in cookies")
	}

	tokenString := tokenCookie.Value

	// Parse the token with claims
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Check the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return secretKey, nil
	})

	// Check token validity
	if err != nil {
		return nil, errors.New("invalid token")
	}

	// Extract claims
	claims, ok := token.Claims.(*CustomClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	// Check expiration time
	if claims.RegisteredClaims.ExpiresAt.Unix() < time.Now().Unix() {
		return nil, errors.New("token expired")
	}

	return claims, nil
}

// AuthMiddleware verifies the token and provides authenticated access to subsequent handlers.
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Проверяем токен и извлекаем claims
		claims, err := VerifyJWTToken(r)
		if err != nil {
			// Логируем и перенаправляем на страницу входа
			log.Printf("Unauthorized access attempt: %v", err)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Логируем аутентифицированного пользователя
		log.Printf("Authenticated user: %s with role: %s", claims.Username, claims.Role)

		// Передаём управление следующему обработчику
		next.ServeHTTP(w, r)
	})
}
