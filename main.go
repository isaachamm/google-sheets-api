package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"slices"

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

	// GET endpoints
	http.HandleFunc("GET /readSpreadsheetMetaData", readSpreadsheetMetaData)
	http.HandleFunc("GET /readSheetData", readSheetData)

	// POST endpoints
	http.HandleFunc("POST /createSpreadsheet", createSpreadsheet)
	http.HandleFunc("POST /createSheet", createSheet)
	http.HandleFunc("POST /addObjectToSheet", addObjectToSheet)

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

	columnHeaders, sheetData, err := readNRowsFromSheetByTitle(10, sheetTitle, spreadsheetToRead)

	if err != nil {
		log.Fatalf("Error while reading rows from sheet: %v", err)
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

	}

	for _, i := range fileList.Files {
		if i.Name == newTitle {
			fmt.Printf("%s (%s)\n", i.Name, i.Id)
			fmt.Println("Request received with sheet title already exists (Status 409)")
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte("Sheet title already exists, please choose another"))
			return
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

type SheetCreationHttpRequest struct {
	SpreadsheetTitle      string
	NewSheetTitle         string
	NewSheetColumnHeaders []string
}

func createSheet(w http.ResponseWriter, r *http.Request) {

	// This step is necessary so that we can get the request body as a byte array to pass to json.Unmarshal()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Fatalf("Unable to parse request body. Error: %v", err)
	}

	requestBody := new(SheetCreationHttpRequest)
	err = json.Unmarshal([]byte(body), &requestBody)
	if err != nil {
		log.Fatalf("Unable to unmarshal request body JSON. Error: %v", err)
	}

	var newSheetTitle string = requestBody.NewSheetTitle
	var spreadsheetTitle string = requestBody.SpreadsheetTitle
	var columnHeadersStrings []string = requestBody.NewSheetColumnHeaders

	var newColumnHeaders []*sheets.CellData = make([]*sheets.CellData, 0)

	idString := "id"
	idHeader := &sheets.CellData{UserEnteredValue: &sheets.ExtendedValue{StringValue: &idString}}
	newColumnHeaders = append(newColumnHeaders, idHeader)

	for _, header := range columnHeadersStrings {
		newHeader := &sheets.CellData{UserEnteredValue: &sheets.ExtendedValue{StringValue: &header}}
		newColumnHeaders = append(newColumnHeaders, newHeader)
	}

	spreadsheetId := getSpreadsheetId(spreadsheetTitle)

	appendSheetResponse, err := sheetsService.Spreadsheets.BatchUpdate(spreadsheetId,
		&sheets.BatchUpdateSpreadsheetRequest{
			IncludeSpreadsheetInResponse: true,
			Requests: []*sheets.Request{
				{
					AddSheet: &sheets.AddSheetRequest{
						Properties: &sheets.SheetProperties{
							Title: newSheetTitle,
						},
					},
				},
			},
		},
	).Do()

	if err != nil {
		fmt.Printf("Error while trying to add sheet to spreadsheet: %v\n", err)
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte("Error while trying to add sheet to spreadsheet"))
		return
	}

	appendCellsResponse, err := sheetsService.Spreadsheets.BatchUpdate(spreadsheetId,
		&sheets.BatchUpdateSpreadsheetRequest{
			IncludeSpreadsheetInResponse: true,
			Requests: []*sheets.Request{
				{
					AppendCells: &sheets.AppendCellsRequest{
						Fields: "*",
						Rows: []*sheets.RowData{
							{
								Values: newColumnHeaders,
							},
						},
						SheetId: appendSheetResponse.Replies[0].AddSheet.Properties.SheetId,
					},
				},
			},
		},
	).Do()

	if err != nil {
		fmt.Printf("Error while trying to add sheet headers: %v\n", err)
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(string("Error while trying to add sheet headers")))
		return
	}

	var responseBody map[string]any = make(map[string]any)

	responseBody["SpreadsheetID"] = appendCellsResponse.SpreadsheetId
	responseBody["NewSheetTitle"] = appendSheetResponse.Replies[0].AddSheet.Properties.Title
	responseBody["SpreadsheetUrl"] = buildSpreadsheetUrl(spreadsheetId, appendSheetResponse.Replies[0].AddSheet.Properties.SheetId)
	// This isn't perfect because we're trusting that the data got applied rather than checking it, but it should be guaranteed to update with a non-error response, so good if not perfect. Not worth a third network call imo.
	responseBody["ColumnHeaders"] = columnHeadersStrings

	responseBodyBytes, err := json.Marshal(responseBody)
	if err != nil {
		log.Fatalf("Error turning response body to JSON bytes. Error: %v", err)
	}
	w.WriteHeader(http.StatusCreated)
	w.Write(responseBodyBytes)
}

type AddObjectToSheetRequest struct {
	SpreadsheetTitle string
	SheetTitle       string
	NewObject        []string
}

func addObjectToSheet(w http.ResponseWriter, r *http.Request) {

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Fatalf("Unable to parse request body. Error: %v", err)
	}

	requestBody := new(AddObjectToSheetRequest)
	err = json.Unmarshal([]byte(body), &requestBody)
	if err != nil {
		log.Fatalf("Unable to unmarshal request body JSON. Error: %v", err)
	}

	var spreadsheetTitle string = requestBody.SpreadsheetTitle
	var sheetTitle string = requestBody.SheetTitle
	var newObject []string = requestBody.NewObject

	spreadsheetId := getSpreadsheetId(spreadsheetTitle)
	spreadsheet, err := sheetsService.Spreadsheets.Get(spreadsheetId).Do(googleapi.QueryParameter("includeGridData", "true"))
	if err != nil {
		fmt.Printf("Unable to get spreadsheet from sheets service: %v", err)
		w.WriteHeader(http.StatusNotFound)
		w.Write(fmt.Appendf(nil, "Unable to get spreadsheet from sheets service: %v", err))
		return
	}

	sheetIndex := slices.IndexFunc(spreadsheet.Sheets, func(s *sheets.Sheet) bool {
		return s.Properties.Title == sheetTitle
	})
	var sheetId int64 = spreadsheet.Sheets[sheetIndex].Properties.SheetId

	var newObjectData []*sheets.CellData = make([]*sheets.CellData, 0)

	// Make an objectId that is incremental according to row length and store it in the first column
	var newObjectId float64 = float64(len(spreadsheet.Sheets[sheetIndex].Data[0].RowData) + 1)
	newObjectIdData := &sheets.CellData{UserEnteredValue: &sheets.ExtendedValue{NumberValue: &newObjectId}}
	newObjectData = append(newObjectData, newObjectIdData)

	for _, value := range newObject {
		newValue := &sheets.CellData{UserEnteredValue: &sheets.ExtendedValue{StringValue: &value}}
		newObjectData = append(newObjectData, newValue)
	}

	_, err = sheetsService.Spreadsheets.BatchUpdate(spreadsheetId,
		&sheets.BatchUpdateSpreadsheetRequest{
			IncludeSpreadsheetInResponse: false,
			Requests: []*sheets.Request{
				{
					AppendCells: &sheets.AppendCellsRequest{
						Fields: "*",
						Rows: []*sheets.RowData{
							{
								Values: newObjectData,
							},
						},
						SheetId: sheetId,
					},
				},
			},
		},
	).Do()

	if err != nil {
		fmt.Printf("Error while trying to add sheet headers: %v\n", err)
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(string("Error while trying to add sheet headers")))
		return
	}

	var responseBody map[string]any = make(map[string]any)

	responseBody["SheetUrl"] = buildSpreadsheetUrl(spreadsheetId, sheetId)

	responseBodyBytes, err := json.Marshal(responseBody)
	if err != nil {
		log.Fatalf("Error turning response body to JSON bytes. Error: %v", err)
	}
	w.WriteHeader(http.StatusCreated)
	w.Write(responseBodyBytes)

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

func buildSpreadsheetUrl(spreadsheetId string, sheetId int64) string {
	return fmt.Sprintf("https://docs.google.com/spreadsheets/d/%s?gid=%d", spreadsheetId, sheetId)
}

/*
return values: columnHeaders string[], sheetData map[int][]string, error
*/
func readNRowsFromSheetByTitle(numRows int, sheetTitle string, spreadsheet *sheets.Spreadsheet) ([]string, map[string]map[string]string, error) {
	var columnHeaders []string
	var sheetData map[string]map[string]string = make(map[string]map[string]string)
	for _, sheet := range spreadsheet.Sheets {
		if sheet.Properties.Title != sheetTitle {
			continue
		}

		if len(sheet.Data) == 0 {
			return columnHeaders, sheetData, nil
		}

		for index, row := range sheet.Data[0].RowData {
			if index > numRows {
				break
			}

			if index == 0 {
				for _, rowValue := range row.Values {
					columnHeaders = append(columnHeaders, rowValue.FormattedValue)
				}
				continue
			}

			var newRow map[string]string = make(map[string]string)
			for columnIndex, rowValue := range row.Values {
				newRow[columnHeaders[columnIndex]] = rowValue.FormattedValue
			}
			sheetData[fmt.Sprintf("Row%v", index)] = newRow
		}
		break
	}

	if columnHeaders == nil {
		return nil, nil, errors.New("Unable to find requested sheet " + sheetTitle + " in " + spreadsheet.Properties.Title)
	}

	return columnHeaders, sheetData, nil
}
