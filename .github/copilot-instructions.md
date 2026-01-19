# Copilot Instructions for API Call through HTTP

## Overview
This project is designed to perform API calls and save the results into CSV files. It supports both GET and POST methods, with the ability to handle JSON payloads and authentication via bearer tokens.

## Architecture
- **Main Components**: The main entry point is `main.go`, which handles command-line arguments, configuration loading, and API requests.
- **Configuration**: The `config.yml` file specifies the base URL and paths for input and output files. Ensure this file is correctly set up before running the application.
- **Data Flow**: The application reads input from CSV files, converts it to JSON, makes HTTP requests, and writes the output to CSV files.

## Developer Workflows
- **Building the Project**: Use the standard Go build command to compile the application.
- **Running the Application**: Execute the binary with the required flags. Example:
  ```bash
  call.exe -config=c:\api\config.yml -url=/resource -method=GET
  ```
- **Testing**: Ensure to test both GET and POST requests with valid and invalid data to verify error handling.

## Project-Specific Conventions
- **File Naming**: Input files for POST requests should be named `input.csv` or `object.csv` depending on the payload structure.
- **Error Logging**: Errors are logged to `errors.log` in the output directory. Check this file for any issues during execution.

## Integration Points
- **External Dependencies**: The project uses the `golang.org/x/text/encoding` package for character encoding conversions.
- **Cross-Component Communication**: The application communicates with external APIs via HTTP requests, handling responses and errors appropriately.

## Examples
- **GET Request**: To fetch data from an API:
  ```bash
  call.exe -url=/resource -method=GET
  ```
- **POST Request**: To send data:
  ```bash
  call.exe -url=/resource -method=POST
  ```

## Additional Notes
- Ensure that the input CSV files are formatted correctly to avoid parsing errors.
- Use the `-debug` flag to enable debug mode for more verbose output during development.

---