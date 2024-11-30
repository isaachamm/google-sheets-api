package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

var sheetsService *sheets.Service
const DEFAULT_FILE_PERMISSIONS = 0644

func main() {

	ctx := context.Background()
	b, err := os.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, "https://www.googleapis.com/auth/spreadsheets")
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(config)

	sheetsService, err = sheets.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to retrieve Sheets client: %v", err)
	}

	http.HandleFunc("/createSpreadsheet", createSpreadsheet)

	err = http.ListenAndServe("127.0.0.1:3333", nil)

	if errors.Is(err, http.ErrServerClosed) {
		fmt.Printf("server closed\n")
	} else if err != nil {
		fmt.Printf("error starting server: %s\n", err)
		os.Exit(1)
	}
}

type SpreadsheetCreation struct {
	Title string
}

func createSpreadsheet(w http.ResponseWriter, r *http.Request) {

	decoder := json.NewDecoder(r.Body)
	var requestBody SpreadsheetCreation
	err := decoder.Decode(&requestBody)
	if err != nil {
		panic(err)
	}

	var newTitle string = requestBody.Title

	// newTitle := ""
	newSpreadsheet, err := sheetsService.Spreadsheets.Create(&sheets.Spreadsheet{Properties: &sheets.SpreadsheetProperties{Title: newTitle}}).Do()
	if err != nil {
		log.Fatalf("Unable to create new sheet with title: %s. Error: %v", newTitle, err)
	}

	if newSpreadsheet.Properties.Title != "" && newSpreadsheet.SpreadsheetId != "" {

		spreadsheetJsonFilePath := "data/spreadsheetIDs.json"
		spreadsheetJsonFile, err := os.ReadFile(spreadsheetJsonFilePath)

		if err != nil {
			log.Fatalf("Unable to read Data Spreadsheet ID file: %v", err)
		}

		var spreadsheetIdObject map[string]string
		err = json.Unmarshal(spreadsheetJsonFile, &spreadsheetIdObject)
		if err != nil {
			log.Fatalf("Unable to decode Data Spreadsheet ID JSON: %v", err)
		}

		spreadsheetIdObject[newSpreadsheet.Properties.Title] = newSpreadsheet.SpreadsheetId

		dataBytes, err := json.Marshal((spreadsheetIdObject))
		if err != nil {
			log.Fatalf("Unable to encode Data Spreadsheet ID JSON: %v", err)
		}

		os.WriteFile(spreadsheetJsonFilePath, dataBytes, DEFAULT_FILE_PERMISSIONS)

		var responseBody map[string]string = make(map[string]string)

		responseBody["SpreadsheetID"] = newSpreadsheet.SpreadsheetId

		responseBodyBytes, err := json.Marshal(responseBody)
		if err != nil {
			log.Fatalf("Error turning response body to JSON bytes. Error: %v", err)
		}
		w.Write(responseBodyBytes)
	}

}
