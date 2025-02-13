package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/jung-kurt/gofpdf"
	gomail "gopkg.in/gomail.v2"
)

var (
	templateDir   string
	staticDir     string
	uploadTempDir string
)
// Payment Response Struct
type PaymentResponse struct {
	Status      string  `json:"status"`
	TotalAmount float64 `json:"total_amount"`
	Currency    string  `json:"currency"`
}

// Cart Item Struct
type CartItem struct {
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Quantity int     `json:"quantity"`
}

// Payment Request Struct
type PaymentRequest struct {
	Currency      string     `json:"currency"`
	Items         []CartItem `json:"items"`
	CustomerName  string     `json:"customer_name"`
	Email         string     `json:"email"`
	PaymentMethod string     `json:"payment_method"`
}

// Logger Setup
var logFile *os.File

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
func serveHTML(w http.ResponseWriter, r *http.Request, filename string) {
	filePath := filepath.Join(templateDir, filename)
	http.ServeFile(w, r, filePath)
}

func initLogger() {
	var err error
	logFile, err = os.OpenFile("payment_service.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0664)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	log.SetOutput(logFile)
	log.SetFlags(log.LstdFlags | log.Lshortfile) // Include timestamp & file info
	log.Println("Logger initialized.")
}

func generatePDF(req PaymentRequest, transactionID string) (string, error) {
	log.Printf("[INFO] Generating PDF receipt for transaction: %s", transactionID)

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(40, 10, "Fiscal Receipt")

	pdf.Ln(10)
	pdf.SetFont("Arial", "", 12)
	pdf.Cell(40, 10, fmt.Sprintf("Transaction ID: %s", transactionID))
	pdf.Ln(8)
	pdf.Cell(40, 10, fmt.Sprintf("Date: %s", time.Now().Format("2006-01-02 15:04:05")))
	pdf.Ln(8)
	pdf.Cell(40, 10, fmt.Sprintf("Customer: %s", req.CustomerName))
	pdf.Ln(8)
	pdf.Cell(40, 10, fmt.Sprintf("Payment Method: %s", req.PaymentMethod))
	pdf.Ln(8)

	totalAmount := 0.0
	for _, item := range req.Items {
		pdf.Cell(40, 10, fmt.Sprintf("%s - %d x %.2f %s", item.Name, item.Quantity, item.Price, req.Currency))
		pdf.Ln(8)
		totalAmount += item.Price * float64(item.Quantity)
	}

	pdf.Ln(10)
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(40, 10, fmt.Sprintf("Total Amount: %.2f %s", totalAmount, req.Currency))

	filename := fmt.Sprintf("receipt_%s.pdf", transactionID)
	err := pdf.OutputFileAndClose(filename)
	if err != nil {
		log.Printf("[ERROR] Failed to generate PDF: %v", err)
	}
	log.Printf("[INFO] PDF receipt saved: %s", filename)

	return filename, err
}

func sendEmail(to string, pdfPath string) error {
	log.Printf("[INFO] Sending email receipt to: %s", to)

	m := gomail.NewMessage()
	m.SetHeader("From", "230047@astanait.edu.kz")
	m.SetHeader("To", to)
	m.SetHeader("Subject", "Your Payment Receipt")
	m.SetBody("text/plain", "Please find your receipt attached.")
	m.Attach(pdfPath)

	d := gomail.NewDialer("smtp.office365.com", 587, "230047@astanait.edu.kz", "aRBmKl1O0G0kw")
	err := d.DialAndSend(m)

	if err != nil {
		log.Printf("[ERROR] Failed to send email: %v", err)
	} else {
		log.Printf("[INFO] Email sent successfully to %s", to)
	}
	return err
}

func handlePayment(w http.ResponseWriter, r *http.Request) {
	log.Println("[INFO] Received payment request")

	var req PaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[ERROR] Invalid request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Currency == "" || len(req.Items) == 0 || req.CustomerName == "" || req.Email == "" {
		log.Println("[ERROR] Missing required fields in request")
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	// Calculate total amount
	totalAmount := 0.0
	for _, item := range req.Items {
		totalAmount += item.Price * float64(item.Quantity)
	}
	transactionID := fmt.Sprintf("TXN%d", time.Now().Unix())

	log.Printf("[INFO] Processing payment for %s | Total: %.2f %s", req.CustomerName, totalAmount, req.Currency)

	// Generate PDF
	pdfPath, err := generatePDF(req, transactionID)
	if err != nil {
		log.Printf("[ERROR] Failed to generate receipt PDF: %v", err)
		http.Error(w, "Failed to generate receipt", http.StatusInternalServerError)
		return
	}

	// Send email
	err = sendEmail(req.Email, pdfPath)
	if err != nil {
		log.Printf("[ERROR] Failed to send receipt email: %v", err)
		http.Error(w, "Failed to send receipt", http.StatusInternalServerError)
		return
	}

	// Response
	response := PaymentResponse{
		Status:      "Completed",
		TotalAmount: totalAmount,
		Currency:    req.Currency,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

	log.Printf("[INFO] Payment completed | Transaction ID: %s | Customer: %s", transactionID, req.CustomerName)
}

func getTemplatePath(filename string) string {
	basePath, _ := os.Getwd() // Получаем текущую директорию
	return filepath.Join(basePath, "templates", filename)
}

func handleCardForm(w http.ResponseWriter, r *http.Request) {
		serveHTML(w, r, "card.html")
	email := r.URL.Query().Get("email")
	tmplPath := getTemplatePath("card.html")
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, struct{ Email string }{Email: email})
}

func main() {
	initPaths()
	initLogger()
	defer logFile.Close()
	staticDir, _ := os.Getwd()
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(filepath.Join(staticDir, "static")))))

	http.HandleFunc("/pay", handlePayment)
	http.HandleFunc("/card", handleCardForm)

	log.Println("[INFO] Payment service running on port 8082")
	log.Fatal(http.ListenAndServe(":8082", nil))
}
