

# Post Endpoints

## Create Spreadsheet

URL: 

## Create Sheet

URL: `POST /createSheet`

Request body:
- string: SpreadsheetTitle
- string: NewSheetTitle
- []string: NewSheetColumnHeaders

Return body:
- []string: ColumnHeaders
- string: NewSheetTitle
- string: SpreadsheetID
- string: SpreadsheetUrl

## Add new object to sheet

URL: `POST /addObjectToSheet`

Request body:
- string: SpreadsheetTitle
- string: SheetTitle
- []string: NewObject

Return body:
- string: SheetUrl

----

# Get Endpoints

## Get Spreadsheet metadata

## Get Sheet data

## Get Spreadsheet titles

-- NOT IMPLEMENTED --