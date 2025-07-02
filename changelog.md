
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