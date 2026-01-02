package main

import (
	"database/sql"
	"log"

	_ "modernc.org/sqlite"
)

var DB *sql.DB // Capitalized to be visible (though not strictly necessary if in same package)

func InitDB(path string) error {
	_db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	DB = _db // Assign to global DB variable
	// Create a simple table for intercepted data
	query := `
	    CREATE TABLE IF NOT EXISTS requests (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		endpoint TEXT,
		payload TEXT,
		response TEXT
	    );`

	_, err = DB.Exec(query)

	query = `
		CREATE TABLE IF NOT EXISTS overrides (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		endpoint_lookup TEXT,
		payload_lookup TEXT,
		response_lookup TEXT,
		response_override TEXT
		);`
	_, err = DB.Exec(query)

	return err
}

func saveRequest(requestEndPoint string, payload string, response string) {
	log.Println("Saving request to DB...")
	_, err := DB.Exec("INSERT INTO requests (endpoint, payload, response) VALUES (?, ?, ?)", requestEndPoint, payload, response)
	if err != nil {
		log.Println("Failed to save to DB: ", err)
	}
}

func getAllRequests() ([]map[string]interface{}, error) {
	rows, err := DB.Query("SELECT id, timestamp, endpoint, payload, response FROM requests")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id int
		var timestamp string
		var endpoint string
		var payload string
		var response string

		err := rows.Scan(&id, &timestamp, &endpoint, &payload, &response)
		if err != nil {
			return nil, err
		}

		record := map[string]interface{}{
			"id":        id,
			"timestamp": timestamp,
			"endpoint":  endpoint,
			"payload":   payload,
			"response":  response,
		}
		results = append(results, record)
	}
	return results, rows.Err()
}

func saveOverride(endpointLookup string, payloadLookup string, responseLookup string, responseOverride string) error {
	_, err := DB.Exec("INSERT INTO overrides (endpoint_lookup, payload_lookup, response_lookup, response_override) VALUES (?, ?, ?, ?)", endpointLookup, payloadLookup, responseLookup, responseOverride)
	return err
}

func getAllOverrides() ([]map[string]interface{}, error) {
	rows, err := DB.Query("SELECT id, endpoint_lookup, payload_lookup, response_lookup, response_override FROM overrides")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id int
		var endpointLookup string
		var payloadLookup string
		var reponseLookup string
		var responseOverride string

		err := rows.Scan(&id, &endpointLookup, &payloadLookup, &reponseLookup, &responseOverride)
		if err != nil {
			return nil, err
		}

		record := map[string]interface{}{
			"id":                id,
			"endpoint_lookup":   endpointLookup,
			"payload_lookup":    payloadLookup,
			"response_lookup":   reponseLookup,
			"response_override": responseOverride,
		}
		results = append(results, record)
	}
	return results, rows.Err()
}

func getOverride(endpoint string, payload string) (string, bool, error) {
	query := `
		SELECT response_override FROM overrides
		WHERE endpoint_lookup = ? AND (payload_lookup = ? OR payload_lookup IS NULL OR payload_lookup = '')
		ORDER BY id DESC LIMIT 1;
	`
	row := DB.QueryRow(query, endpoint, payload)
	var overrideResponse string
	err := row.Scan(&overrideResponse)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return overrideResponse, true, nil
}

func getOverrideBasedOnResponse(response string) (string, bool, error) {
	query := `
		SELECT response_override FROM overrides 
		WHERE INSTR(?, response_lookup) > 0;
	`
	log.Println("response", response)
	row := DB.QueryRow(query, response)
	log.Println("row:", row)
	var overrideResponse string
	err := row.Scan(&overrideResponse)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return overrideResponse, true, nil
}
