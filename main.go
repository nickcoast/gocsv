package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	_ "embed"

	"github.com/gorilla/mux"
	vault "github.com/hashicorp/vault/api"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
	"github.com/nickcoast/gocsv/models"
	"github.com/rs/cors"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

type Env struct {
	upload models.UploadModel
}

func main() {
	// user=ogrego password=vagrant dbname=ogrego host=localhost sslmode=disable
	postgresCredentials, err := getPostgresCredentials()
	if err != nil {
		log.Fatalf("Failed to get the PostgreSQL password: %v", err)
	}
	//postgresDB := "ogrecsv"
	postgresDB := "ogrego"

	connStr := fmt.Sprintf("user=%s password=%s dbname=%s host=localhost sslmode=disable", postgresCredentials.Username, postgresCredentials.Password, postgresDB)

	db, err := models.NewDB(connStr)
	//var asdf *db.UploadModel
	env := &Env{
		upload: models.UploadModel{DB: db},
	}

	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	r := mux.NewRouter()

	r.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		handleFileUpload(w, r, db)
	}).Methods("POST", "OPTIONS")
	r.HandleFunc("/files", env.fetchUploadedFiles).Methods("GET", "OPTIONS")
	r.HandleFunc("/files/{id}", env.deleteFile).Methods("DELETE", "OPTIONS")
	r.HandleFunc("/import-formats", func(w http.ResponseWriter, r *http.Request) {
		getImportFormatsHandler(w, r, db)
	}).Methods("GET", "OPTIONS")
	r.HandleFunc("/update-file-format", func(w http.ResponseWriter, r *http.Request) {
		updateFileFormatHandler(w, r, db)
	}).Methods("GET", "OPTIONS")
	r.HandleFunc("/files/{fileId}", func(w http.ResponseWriter, r *http.Request) {
		fetchFileDetails(w, r, db)
	}).Methods("GET", "OPTIONS")

	/* r.HandleFunc("/upload", handleFileUpload).Methods("POST")
	r.HandleFunc("/files", fetchUploadedFiles).Methods("GET")
	r.HandleFunc("/files/{id}", func(w http.ResponseWriter, r *http.Request) {
		deleteFile(w, r, db)
	  }).Methods("DELETE")
	*/
	// Add CORS middleware
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000"}, // Change this to the appropriate origin in production
		AllowedMethods:   []string{"POST", "GET", "PUT", "DELETE", "OPTIONS"},
		AllowCredentials: true,
	})

	handler := c.Handler(r)

	port := ":8080"
	fmt.Println("Listening on port", port)
	log.Fatal(http.ListenAndServe(port, handler))
}

type PostgresCredentials struct {
	Username string
	Password string
}

func getPostgresCredentials() (PostgresCredentials, error) {
	// Check environment variables for PostgreSQL credentials
	username := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")

	// If credentials are found in environment variables, return them
	if username != "" && password != "" {
		return PostgresCredentials{
			Username: username,
			Password: password,
		}, nil
	}

	// If credentials are not found in environment variables, try fetching from Vault
	vaultAddr := os.Getenv("VAULT_ADDR")
	vaultToken := os.Getenv("VAULT_TOKEN")
	// vaultSecretID := os.Getenv("VAULT_SECRET_ID")

	config := &vault.Config{
		Address: vaultAddr,
	}

	client, err := vault.NewClient(config)
	if err != nil {
		return PostgresCredentials{}, fmt.Errorf("failed to create Vault client: %v", err)
	}

	client.SetToken(vaultToken)

	// Fetch dynamic credentials from the database secrets engine
	secret, err := client.Logical().Read("database/creds/gocsvdb")
	if err != nil {
		return PostgresCredentials{}, fmt.Errorf("failed to read secret from Vault: %v", err)
	}

	if secret == nil || secret.Data == nil {
		return PostgresCredentials{}, fmt.Errorf("no data in the secret")
	}

	username, ok := secret.Data["username"].(string)
	if !ok {
		return PostgresCredentials{}, fmt.Errorf("no username in the secret data")
	}

	password, ok = secret.Data["password"].(string)
	if !ok {
		return PostgresCredentials{}, fmt.Errorf("no password in the secret data")
	}

	return PostgresCredentials{
		Username: username,
		Password: password,
	}, nil
}

// Generic key retrieval. TODO: use this in getPostgresCredentials
func getSecret(key string) (string, error) {
	// Try to get the secret from environment variables
	secret := os.Getenv(key)
	if secret != "" {
		return secret, nil
	}

	// If the secret is not in the environment variables, try to get it from Vault
	vaultAddr := os.Getenv("VAULT_ADDR")
	vaultToken := os.Getenv("VAULT_TOKEN")
	vaultSecretID := os.Getenv("VAULT_SECRET_ID")

	if vaultAddr == "" || vaultToken == "" {
		return "", fmt.Errorf("Secret %s not found in environment variables or Vault configuration", key)
	}

	// Create a Vault client
	client, err := vault.NewClient(&vault.Config{Address: vaultAddr})
	if err != nil {
		return "", fmt.Errorf("Failed to create Vault client: %v", err)
	}

	// Authenticate with Vault
	if vaultSecretID != "" {
		// AppRole authentication
		secret, err := client.Logical().Write("auth/approle/login", map[string]interface{}{
			"role_id":   vaultToken,
			"secret_id": vaultSecretID,
		})
		if err != nil {
			return "", fmt.Errorf("Failed to log in with AppRole: %v", err)
		}
		client.SetToken(secret.Auth.ClientToken)
	} else {
		// Token authentication
		client.SetToken(vaultToken)
	}

	// Read the secret from Vault
	vaultSecret, err := client.Logical().Read("secret/gocsvdb")
	if err != nil {
		return "", fmt.Errorf("Failed to read secret from Vault: %v", err)
	}

	// Get the secret value
	secretValue, ok := vaultSecret.Data[key].(string)
	if !ok {
		return "", fmt.Errorf("Secret %s not found in Vault", key)
	}

	return secretValue, nil
}

func handleFileUpload(w http.ResponseWriter, r *http.Request, db *models.DB) {
	ctx := r.Context()
	tx, err := db.BeginTx(ctx)
	if err != nil {
		http.Error(w, "Error starting transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	err = r.ParseMultipartForm(32 << 20) // 32 MB
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	f, fhead, err := r.FormFile("file")
	file := &models.File{File: f, Header: fhead}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.File.Close()

	// Check the MIME type of the uploaded file
	buffer := make([]byte, 512)
	_, err = file.File.Read(buffer)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusBadRequest)
		return
	}
	contentType := http.DetectContentType(buffer)
	if contentType != "text/csv" && contentType != "text/plain; charset=utf-8" && !strings.HasPrefix(contentType, "image/") {
		http.Error(w, "Invalid file type. Only CSV and image files are allowed", http.StatusBadRequest)
		return
	}

	if contentType == "text/csv" || contentType == "text/plain; charset=utf-8" {
		file.File.Seek(0, 0)
		if err != nil {
			log.Println("Error getting max column lengths:", err)
			http.Error(w, "Error processing CSV file", http.StatusInternalServerError)
			return
		}
		//tableName := toPostgreSQLName(handler.Filename)

		sequenceName := "core_raw_tables_id_seq"
		lastValue, err := getLastSequenceValue(ctx, tx, sequenceName)
		if err != nil {
			log.Printf("Error getting last sequence value: %v", err)
			http.Error(w, "Error processing file", http.StatusInternalServerError)
			return
		}
		tableName := fmt.Sprintf("raw_table_%d", lastValue+1)

		// Calculate the file hash
		file.File.Seek(0, 0)
		fileHash, err := file.CalculateFileHash(file.File)
		if err != nil {
			log.Println("Error calculating file hash:", err)
			return
		}

		// Calculate the file hash without BOM
		file.File.Seek(0, 0)
		fileNoBOM := file.RemoveBOM(file.File)
		fileHashNoBOM, err := file.CalculateFileHash(fileNoBOM)
		fileTrimmedNoBOM, err := file.RemoveEmptyRows(fileNoBOM)
		fileHashTrimmedNoBOM, err := file.CalculateFileHash(fileTrimmedNoBOM)
		if err != nil {
			log.Println("Error calculating file hash without BOM:", err)
			return
		}

		query := "INSERT INTO core_raw_tables (source_filename, file_size, datetime_uploaded, name, file_hash, file_hash_no_bom, file_hash_trimmed_no_bom) VALUES ($1, $2, $3, $4, $5, $6, $7)" // could add "RETURNING id"
		_, err = tx.ExecContext(ctx, query, fhead.Filename, fhead.Size, time.Now(), tableName, fileHash, fileHashNoBOM, fileHashTrimmedNoBOM)
		fmt.Println(query+"\n", fhead.Filename+"\n", fhead.Size, fhead.Header, time.Now(), tableName+"\n")
		if err != nil {
			fmt.Print(err)
			http.Error(w, "Failed to save file information to the database", http.StatusInternalServerError)
			return
		}

		columnNames, err := createTableForCSV(ctx, tx, *file, tableName)
		if err != nil {
			tx.Rollback()
			log.Println("Error creating table:", err)
			http.Error(w, "Error creating table", http.StatusInternalServerError)
			return
		}

		err = importCSVDataToTable(ctx, tx, *file, tableName, columnNames)
		if err != nil {
			log.Println("Error importing data:", err)
			txErr := tx.Rollback()
			log.Println("Tx error:", txErr)
			http.Error(w, "Error importing data", http.StatusInternalServerError)
			return
		}
		err = tx.Commit()
		if err != nil {
			http.Error(w, "Error committing transaction", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}

	_, err = file.File.Seek(0, io.SeekStart) // Reset the file read position
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusBadRequest)
		return
	}

	uploadPath := filepath.Join("uploads", fhead.Filename)
	err = os.MkdirAll(filepath.Dir(uploadPath), os.ModePerm)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tempFile, err := os.CreateTemp("uploads", fhead.Filename+"_tmp_*")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer tempFile.Close()

	fileBytes, err := io.ReadAll(file.File)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = tempFile.Write(fileBytes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = os.Rename(tempFile.Name(), uploadPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "File uploaded successfully: %s", fhead.Filename)
}

// Add a new function to fetch file information from the database
func (env *Env) fetchUploadedFiles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uploads, err := env.upload.All(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(uploads)
}

func (env *Env) deleteFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	id := vars["id"]
	idInt, err := strconv.Atoi(id)
	err = env.upload.Delete(ctx, idInt)
	if err != nil {
		http.Error(w, "Failed to delete file", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("File deleted successfully"))
}

// Returns column names
// Creates table in DB, skipping completely empty columns and rows
// For zero-length columns with headers, sets to VARCHAR(1)
func createTableForCSV(ctx context.Context, tx *models.Tx, file models.File, tableName string) ([]string, error) {
	// Read the first line of the CSV file to get the column headers
	file.File.Seek(0, 0)
	maxLengths, headerLengths, err := file.GetMaxColumnLengths()
	if err != nil {
		log.Println("Error getting max column lengths:", err)
		return nil, err
	}
	// Reset the reader position before reading headers
	file.File.Seek(0, 0)

	reader := csv.NewReader(file.File)
	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("error reading CSV file: %w", err)
	}

	// Create the table schema using the column headers
	columns := []string{"_id SERIAL PRIMARY KEY"} // use underscore prefix for system column names
	columnNames := []string{}                     // exclude system column names
	for i, header := range headers {
		if maxLengths[i] == 0 && headerLengths[i] == 0 {
			continue // skip this column
		}
		columnName := toPostgreSQLName(header)
		columnLength := maxLengths[i]
		if columnLength < 1 { // 0-length varchar not allowed in Postgres? (allowed in MySQL)
			columnLength = 1
		}
		columns = append(columns, fmt.Sprintf("\"%s\" VARCHAR(%d)", columnName, columnLength))
		columnNames = append(columnNames, columnName)
	}
	schema := strings.Join(columns, ", ")

	_, err = tx.ExecContext(ctx, fmt.Sprintf("CREATE TABLE %s (%s);", tableName, schema))
	if err != nil {
		return nil, fmt.Errorf("error creating table: %w", err)
	}

	return columnNames, nil
}

func toPostgreSQLName(s string) string {
	if strings.ToLower(filepath.Ext(s)) == ".csv" {
		s = strings.TrimSuffix(s, filepath.Ext(s))
	}
	// Convert non-ASCII characters to their ASCII equivalents
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	asciiStr, _, _ := transform.String(t, s)

	// Make all characters lowercase
	lowercaseStr := strings.ToLower(asciiStr)

	asciiRe := regexp.MustCompile(`[^a-zA-Z0-9\n]`)
	underscoreStr := asciiRe.ReplaceAllString(lowercaseStr, "_")

	// Replace all spaces with underscores
	//underscoreStr := strings.ReplaceAll(asciiString, " ", "_")

	re := regexp.MustCompile(`^[0-9]+`)
	cleanStr := re.ReplaceAllString(underscoreStr, "")
	// Remove extra underscores (at the beginning, end, and consecutive underscores)
	re = regexp.MustCompile(`^[0-9_]+|_+$`)
	cleanStr = re.ReplaceAllString(cleanStr, "")

	re = regexp.MustCompile(`_{2,}`)
	cleanStr = re.ReplaceAllString(cleanStr, "_")

	maxLen := 59 // 63 is max, but cutting 4 more off to leave room for table name prefixes
	runeCont := utf8.RuneCountInString(cleanStr)

	if runeCont > maxLen {
		r := []rune(cleanStr)
		trunc := r[:maxLen]
		cleanStr = string(trunc)
	}

	return cleanStr
}

func importCSVDataToTable(ctx context.Context, tx *models.Tx, file models.File, tableName string, columnNames []string) error {
	// Reset the file position to the beginning
	file.File.Seek(0, 0)

	stmt, err := tx.PrepareContext(ctx, pq.CopyIn(tableName, columnNames...))
	if err != nil {
		return fmt.Errorf("error preparing COPY statement: %w", err)
	}

	reader := csv.NewReader(file.File)
	_, err = reader.Read() // Skip header row
	if err != nil {
		return fmt.Errorf("error reading CSV file: %w", err)
	}

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading CSV file: %w", err)
		}

		// Convert the record slice of string to a slice of interface{}
		recordInterface := make([]interface{}, len(record))
		for i, v := range record {
			recordInterface[i] = v
		}

		_, err = stmt.ExecContext(ctx, recordInterface...)
		if err != nil {
			return fmt.Errorf("error executing COPY statement: %w", err)
		}
	}

	_, err = stmt.ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("error executing COPY statement: %w", err)
	}

	err = stmt.Close()
	if err != nil {
		return fmt.Errorf("error closing COPY statement: %w", err)
	}

	return nil
}

func getImportFormatsHandler(w http.ResponseWriter, r *http.Request, db *models.DB) {
	formats := []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}{}

	ctx := r.Context()

	rows, err := db.QueryWithContext(ctx, "SELECT id, name FROM core_import_formats;")
	if err != nil {
		http.Error(w, "Failed to retrieve import formats", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var format struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}
		err := rows.Scan(&format.ID, &format.Name)
		if err != nil {
			http.Error(w, "Error enumerating import formats", http.StatusInternalServerError)
			return
		}
		formats = append(formats, format)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(formats)
}

func updateFileFormatHandler(w http.ResponseWriter, r *http.Request, db *models.DB) {
	fileID := r.FormValue("file_id")
	formatID := r.FormValue("format_id")

	ctx := r.Context()
	tx, err := db.BeginTx(ctx)

	_, err = tx.ExecContext(ctx, "UPDATE core_raw_tables SET format_id = $1 WHERE id = $2;", formatID, fileID)
	if err != nil {
		http.Error(w, "Failed to set import format", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func getLastSequenceValue(ctx context.Context, tx *models.Tx, sequenceName string) (int, error) {
	if sequenceName == "" {
		sequenceName = "core_raw_tables_id_seq"
	}
	var lastValue int

	err := tx.QueryRowContext(ctx, `SELECT last_value FROM `+sequenceName).Scan(&lastValue)
	if err != nil {
		return 0, fmt.Errorf("error getting last value from sequence %s: %w", sequenceName, err)
	}
	return lastValue, nil
}

func fetchFileDetails(w http.ResponseWriter, r *http.Request, db *models.DB) {
	vars := mux.Vars(r)
	fileId, err := strconv.Atoi(vars["fileId"])
	if err != nil {
		http.Error(w, "Invalid file ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	tx, err := db.BeginTx(ctx)

	// Retrieve the table name from core_raw_tables based on the file ID
	var tableName string
	err = tx.QueryRowContext(ctx, "SELECT name FROM core_raw_tables WHERE id = $1", fileId).Scan(&tableName)
	if err != nil {
		http.Error(w, "Error retrieving table name", http.StatusInternalServerError)
		return
	}

	// Retrieve column names
	columns, err := tx.QueryContext(ctx, "SELECT column_name FROM information_schema.columns WHERE table_name = $1", tableName)
	if err != nil {
		http.Error(w, "Error retrieving column names", http.StatusInternalServerError)
		return
	}
	defer columns.Close()

	columnNames := make([]string, 0)
	for columns.Next() {
		var columnName string
		if err := columns.Scan(&columnName); err != nil {
			http.Error(w, "Error reading column names", http.StatusInternalServerError)
			return
		}
		columnNames = append(columnNames, columnName)
	}

	// Retrieve rows data
	rows, err := tx.QueryContext(ctx, fmt.Sprintf("SELECT * FROM %s LIMIT 100", tableName))
	if err != nil {
		http.Error(w, "Error retrieving rows data", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	rowsData := make([]map[string]interface{}, 0)
	for rows.Next() {
		rowData := make(map[string]interface{})
		values := make([]interface{}, len(columnNames))
		scanArgs := make([]interface{}, len(columnNames))

		for i := range values {
			scanArgs[i] = &values[i]
		}

		if err := rows.Scan(scanArgs...); err != nil {
			http.Error(w, "Error reading row data", http.StatusInternalServerError)
			return
		}

		for i, columnName := range columnNames {
			rowData[columnName] = values[i]
		}

		rowsData = append(rowsData, rowData)
	}

	// Create the response JSON
	response := map[string]interface{}{
		"columns": columnNames,
		"rows":    rowsData,
	}

	// Send the JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
