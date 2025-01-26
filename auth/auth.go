package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/gomail.v2"
)

var tpl *template.Template
var secretKey = []byte("mysecretkey")

var collection *mongo.Collection

type User struct {
    ID               string             `bson:"_id,omitempty"`
    Email            string             `bson:"email"`
    Username         string             `bson:"username"`
    Password         string             `bson:"password"` 
    Role             string             `bson:"role"`
    Verified         bool               `bson:"verified"`
    VerificationCode string             `bson:"verificationCode"`
}

type PageData struct {
    ErrorMessage string
}

type CustomClaims struct {
    Username string                 `json:"username"`
    Role     string                 `json:"role"`
    Email    string                 `json:"email"`
	Verified bool  					`json:"verified"`
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

    // Set the output for the logger to file only
    log.SetOutput(file)

    // Optional: Customize log flags (e.g., include timestamps)
    log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
    log.Println("Logging to file:", logFilePath)

    // MongoDB connection setup
    clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
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


func generateVerificationCode() string {
    rand.Seed(time.Now().UnixNano())
    return fmt.Sprintf("%06d", rand.Intn(1000000))
}

func sendEmail(recipient, code string) error {
    mailer := gomail.NewMessage()
    mailer.SetHeader("From", "230047@astanait.edu.kz")
    mailer.SetHeader("To", recipient)
    mailer.SetHeader("Subject", "Email Verification Code")
    mailer.SetBody("text/plain", fmt.Sprintf("Your verification code is: %s", code))

    dialer := gomail.NewDialer("smtp.office365.com", 587, "230047@astanait.edu.kz", "aRBmKl1O0G0kw")
    return dialer.DialAndSend(mailer)
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

        verificationCode := generateVerificationCode()
        user := User{
            Email:        email,
            Username:     username,
            Password:     string(hashedPassword),
            Role:         role,
            VerificationCode: verificationCode,
            Verified:     false,
        }
        _, err = collection.InsertOne(context.TODO(), user)
        if err != nil {
            data.ErrorMessage = "Registration error"
            tpl.ExecuteTemplate(w, "register.html", data)
            log.Printf("Registration error for user %s: %v", username, err)
            return
        }

        err = sendEmail(email, verificationCode)
        if err != nil {
            data.ErrorMessage = "Failed to send verification email"
            tpl.ExecuteTemplate(w, "register.html", data)
            log.Printf("Failed to send verification email to %s: %v", email, err)
            return
        }

        // Generate token and set cookie
        claims := CustomClaims{
            Username: user.Username,
            Role:     user.Role,
            Email:    user.Email,
            RegisteredClaims: jwt.RegisteredClaims{
                ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
            },
        }
        token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
        tokenString, err := token.SignedString(secretKey)
        if err != nil {
            data.ErrorMessage = "Token generation failed"
            tpl.ExecuteTemplate(w, "register.html", data)
            log.Printf("Token generation failed for user %s: %v", user.Username, err)
            return
        }

        http.SetCookie(w, &http.Cookie{
            Name:     "token",
            Value:    tokenString,
            Expires:  time.Now().Add(24 * time.Hour),
            HttpOnly: true,
        })

        http.Redirect(w, r, "/verify", http.StatusSeeOther)
        log.Printf("User %s registered successfully with role %s. Verification email sent.", username, role)
        return
    }
    tpl.ExecuteTemplate(w, "register.html", data)
}


func VerifyHandler(w http.ResponseWriter, r *http.Request) {
    data := PageData{}

    if r.Method == http.MethodGet {
        tpl.ExecuteTemplate(w, "verify.html", data)
        return
    }

    if r.Method == http.MethodPost {
        var reqBody struct {
            VerificationCode string `json:"verificationCode"`
        }
        err := json.NewDecoder(r.Body).Decode(&reqBody)
        if err != nil {
            log.Printf("Error decoding request body: %v", err)
            http.Error(w, "Invalid request", http.StatusBadRequest)
            return
        }

        log.Printf("Received verification code: %s", reqBody.VerificationCode)

        tokenCookie, err := r.Cookie("token")
        if err != nil {
            log.Printf("Error retrieving token cookie: %v", err)
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        tokenString := tokenCookie.Value
        claims := jwt.MapClaims{}

        token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
            return secretKey, nil
        })

        if err != nil || !token.Valid {
            log.Printf("Error parsing token or token invalid: %v", err)
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        log.Printf("Token claims: %+v", claims)

        emailClaim, ok := claims["email"]
        if !ok || emailClaim == nil {
            log.Printf("Missing or invalid email claim in token")
            http.Error(w, "Invalid token: missing email claim", http.StatusUnauthorized)
            return
        }

        email, ok := emailClaim.(string)
        if !ok {
            log.Printf("Email claim is not a string")
            http.Error(w, "Invalid token: email claim is not a string", http.StatusUnauthorized)
            return
        }

        log.Printf("Email from token: %s", email)

        var user User
        err = collection.FindOne(context.TODO(), bson.M{"email": email}).Decode(&user)
        if err != nil {
            log.Printf("Error finding user in database: %v", err)
            http.Error(w, "Invalid email or code", http.StatusUnauthorized)
            return
        }

        if user.VerificationCode != reqBody.VerificationCode {
            log.Printf("Verification failed for email %s: invalid code", email)
            http.Error(w, "Invalid or expired code", http.StatusUnauthorized)
            return
        }

        updateResult, err := collection.UpdateOne(context.TODO(),
            bson.M{"email": email},
            bson.M{"$set": bson.M{"verified": true, "verificationCode": ""}},
        )
        if err != nil {
            log.Printf("Error updating user verification status: %v", err)
            http.Error(w, "Verification update failed", http.StatusInternalServerError)
            return
        }

        if updateResult.ModifiedCount == 0 {
            log.Printf("No document was updated for email %s", email)
            http.Error(w, "No document updated", http.StatusInternalServerError)
            return
        }

        // Update token to include verified claim
        newClaims := CustomClaims{
            Username: user.Username,
            Role:     user.Role,
            Email:    user.Email,
            Verified: true,
            RegisteredClaims: jwt.RegisteredClaims{
                ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
            },
        }
        newToken := jwt.NewWithClaims(jwt.SigningMethodHS256, newClaims)
        newTokenString, err := newToken.SignedString(secretKey)
        if err != nil {
            log.Printf("Error generating new token: %v", err)
            http.Error(w, "Token generation failed", http.StatusInternalServerError)
            return
        }

        http.SetCookie(w, &http.Cookie{
            Name:     "token",
            Value:    newTokenString,
            Expires:  time.Now().Add(24 * time.Hour),
            HttpOnly: true,
        })

        w.WriteHeader(http.StatusOK)
        log.Printf("User %s verified successfully", user.Username)
    }
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
			Email:    user.Email,
			Verified: user.Verified,
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
		claims, err := VerifyJWTToken(r)
		if err != nil {
			log.Printf("Unauthorized access attempt: %v", err)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		log.Printf("Authenticated user: %s with role: %s", claims.Username, claims.Role)

		next.ServeHTTP(w, r)
	})
}

func VerifyMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        tokenCookie, err := r.Cookie("token")
        if err != nil {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        tokenString := tokenCookie.Value
        claims := jwt.MapClaims{}

        token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
            return secretKey, nil
        })

        if err != nil || !token.Valid {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        verified, ok := claims["verified"]
        if !ok || verified != true {
            http.Error(w, "Verification required", http.StatusForbidden)
            return
        }

        next.ServeHTTP(w, r)
    })
}
