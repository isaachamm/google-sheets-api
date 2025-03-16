package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"

	// Sheets API
	"google.golang.org/api/sheets/v4"

	// Drive API
	"google.golang.org/api/drive/v3"
)

var sheetsService *sheets.Service
var driveService *drive.Service

const DEFAULT_FILE_PERMISSIONS = 0644

var SCOPES []string = []string{"https://www.googleapis.com/auth/spreadsheets", "https://www.googleapis.com/auth/drive"}

func main() {

	ctx := context.Background()
	b, err := os.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, SCOPES...)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(config)

	sheetsService, err = sheets.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to retrieve Sheets client: %v", err)
	}

	driveService, err = drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to retrieve Drive client: %v", err)
	}

	// It's essential to understand Google's distinction between the term Spreadsheet (A Sheets document) and Sheet (an individual sheet, of which a spreadsheet may contain many). It may be helpful to think of a spreadsheet as a DB and a sheet as a table.

	http.HandleFunc("GET /readSpreadsheetMetaData", readSpreadsheetMetaData)
	http.HandleFunc("GET /readSheetData", readSheetData)

	http.HandleFunc("POST /createSpreadsheet", createSpreadsheet)

	fmt.Println("Starting server . . .")
	err = http.ListenAndServe("127.0.0.1:3333", nil)

	if errors.Is(err, http.ErrServerClosed) {
		fmt.Printf("server closed\n")
	} else if err != nil {
		fmt.Printf("error starting server: %s\n", err)
		os.Exit(1)
	}
}

func readSheetData(w http.ResponseWriter, r *http.Request) {

	u, err := url.Parse(r.URL.String())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	queryParams := u.Query()

	spreadsheetTitle := queryParams.Get("spreadsheetTitle")
	sheetTitle := queryParams.Get("sheetTitle")

	spreadsheetId := getSpreadsheetId(spreadsheetTitle)
	spreadsheetToRead, err := sheetsService.Spreadsheets.Get(spreadsheetId).Do(googleapi.QueryParameter("includeGridData", "true"))
	if err != nil {
		log.Fatalf("Unable to get spreadsheet from sheets service: %v", err)
	}

	var columnHeaders []string
	var sheetData map[int][]string = make(map[int][]string)
	for _, sheet := range spreadsheetToRead.Sheets {
		if sheet.Properties.Title != sheetTitle {
			continue
		}

		if len(sheet.Data[0].RowData) == 0 {
			w.WriteHeader(http.StatusNoContent)
			w.Write([]byte("No data available in requested Spreadsheet: " + spreadsheetTitle + ": " + sheetTitle))
			break
		}

		for index, row := range sheet.Data[0].RowData {
			if index > 10 {
				break
			}

			if index == 0 {
				for _, rowValue := range row.Values {
					columnHeaders = append(columnHeaders, rowValue.FormattedValue)
				}
				continue
			}

			for _, rowValue := range row.Values {
				sheetData[index] = append(sheetData[index], rowValue.FormattedValue)
			}
		}
		break
	}

	if columnHeaders == nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Unable to find requested sheet " + sheetTitle + " in " + spreadsheetTitle))
	}

	var responseBody map[string]any = make(map[string]any)

	responseBody["ColumnHeaders"] = columnHeaders
	responseBody["SheetData"] = sheetData

	responseBodyBytes, err := json.Marshal(responseBody)
	if err != nil {
		log.Fatalf("Error turning response body to JSON bytes. Error: %v", err)
	}
	w.WriteHeader(http.StatusOK)
	w.Write(responseBodyBytes)

}

func readSpreadsheetMetaData(w http.ResponseWriter, r *http.Request) {

	u, err := url.Parse(r.URL.String())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	queryParams := u.Query()

	var spreadsheetTitle string = queryParams.Get("title")

	spreadsheetId := getSpreadsheetId(spreadsheetTitle)

	spreadsheetToRead, err := sheetsService.Spreadsheets.Get(spreadsheetId).Do(googleapi.QueryParameter("includeGridData", "true"))
	if err != nil {
		log.Fatalf("Unable to get spreadsheet from sheets service: %v", err)
	}

	var sheetTitles map[string][]string = make(map[string][]string)
	for _, sheet := range spreadsheetToRead.Sheets {
		sheetTitles[sheet.Properties.Title] = []string{}
		for _, columnHeaders := range sheet.Data[0].RowData[0].Values {
			sheetTitles[sheet.Properties.Title] = append(sheetTitles[sheet.Properties.Title], columnHeaders.FormattedValue)
		}
	}

	var responseBody map[string]any = make(map[string]any)

	responseBody["SpreadsheetTitle"] = spreadsheetToRead.Properties.Title
	responseBody["SpreadsheetID"] = spreadsheetToRead.SpreadsheetId
	responseBody["SheetTitlesWithColumnHeaders"] = sheetTitles

	responseBodyBytes, err := json.Marshal(responseBody)
	if err != nil {
		log.Fatalf("Error turning response body to JSON bytes. Error: %v", err)
	}
	w.WriteHeader(http.StatusOK)
	w.Write(responseBodyBytes)

}

type SpreadsheetCreationHttpRequest struct {
	Title string
}

func createSpreadsheet(w http.ResponseWriter, r *http.Request) {

	decoder := json.NewDecoder(r.Body)
	var requestBody SpreadsheetCreationHttpRequest
	err := decoder.Decode(&requestBody)
	if err != nil {
		panic(err)
	}

	var newTitle string = requestBody.Title

	fileList, err := driveService.Files.List().Do()
	if err != nil {
		log.Fatalf("Unable to perform query for drive files. Error: %v", err)
	}

	if len(fileList.Files) == 0 {

		fmt.Println("No files found.")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Could not find any files in the drive"))
		return

	} else {
		for _, i := range fileList.Files {
			if i.Name == newTitle {
				fmt.Printf("%s (%s)\n", i.Name, i.Id)
				fmt.Println("Request received with sheet title already exists (Status 409)")
				w.WriteHeader(http.StatusConflict)
				w.Write([]byte("Sheet title already exists, please choose another"))
				return
			}
		}
	}

	newSpreadsheet, err := sheetsService.Spreadsheets.Create(&sheets.Spreadsheet{Properties: &sheets.SpreadsheetProperties{Title: newTitle}}).Do()
	if err != nil {
		log.Fatalf("Unable to create new sheet with title: %s. Error: %v", newTitle, err)
	}

	if newSpreadsheet.Properties.Title != "" && newSpreadsheet.SpreadsheetId != "" {

		// Add new Spreadsheet title/ID pair to data.json file
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
		w.WriteHeader(http.StatusCreated)
		w.Write(responseBodyBytes)
		return
	}

}

func getSpreadsheetId(spreadsheetTitle string) string {
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

	spreadsheetId := spreadsheetIdObject[spreadsheetTitle]
	return spreadsheetId
}
