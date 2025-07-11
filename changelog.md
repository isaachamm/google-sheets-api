# 0.0.6

Changed Object IDs to be UUIDs and added creation timestamp to sheets as well

# 0.0.5

## Add data to sheet
POST /addObjectToSheet
- Creates a new object in an existing spreadsheet. In other words, this adds a new row where each cell is a value from the string array that gets passed in via the request body.

Requirements:
- Data object(s) must match sheet's column headers?
	- This I don't think is possible to ensure without making a separate endpoint for each object type (can't check property names of `any` type in golang)
- User must pass in correct sheet & spreadsheet name?
	- This is necessary to help keep data in order, and quite feasible. Though "correct" is quite vague and difficult to calculate. We just trust that what they pass in is what we need.

## Other 0.0.5
- Added `buildUrl` function that returns the url that points to a specific sheet, rather than just the default spreadsheet URL, which points to the first sheet in the spreadsheet
- Added default ID column header to new sheets when they get created
- Added object ID that matches the row number of the given object
	- Could be problematic for deletions, because it could lead to duplicate objects if one gets deleted. Consider changing method of ID generation in the future

# 0.0.4
Create new Sheet in spreadsheet
- POST /createSheet
- Creates a new sheet in an existing spreadsheet -- like a new table to a DB. Can add column headers (which act as properties of the DB's objects), but no data

Added function to read N rows from sheet 
- Made it so that it returns the row + number rather than just an index. Also now returns key-value pairs for each cell, mapping from columnHeader name to value.

# 0.0.3
Read Sheet data
- GET /readSheetData
- Returns the column headers (i.e., data types/identifiers) and first ten rows of data to see what example data looks like.

# 0.0.2
Read Spreadsheet Metadata
- GET /readSpreadsheetMetadata
- This returns the spreadsheet ID, sheet names within the spreadsheet, and column headers given a spreadsheet title

# 0.0.1
Create a Spreadsheet
- POST /createSpreadsheet
- Creates a spreadsheet and assigns it a spreadsheet ID given a spreadsheet title. The spreadsheet ID assignment is vital because the IDs are GUIDs, but I keep an association between title and GUID in a file that is only stored locally.