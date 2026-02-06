# API Call through HTTP
Perform simple API calls and save the results into XML/CSV files.

The program accepts command-line parameters that specify the method name and resource path: `-url` and `-method`.
Config file `config.yml` must contain a base URL for API calls.
If a method requires a JSON-formatted body, place an `input.csv`/ `input.xml` file adjacent to the application binary. Before executing the HTTP method, the CSV/XML file will be converted into a JSON payload.
Supports authentication with a bearer token.

The data received from the API call is saved into an `output.csv`/ `output.xml` file.

Logs and errors are recorded in an `errors.log` file.

Cyrillic values are converted from Windows-1251 to UTF-8 encoding before sending. 
Received data is converted vice versa (from UTF-8 to Windows-1251).

## Usage
Place the `config.yml` file in the same directory as the application binary, or use parameter `-config` to specify the path to the config file.
In the config file, specify the base URL for API calls.

````yml
---
base_url: https://my-test.site/api/v1
input_path: \\work_dir\
output_path: \\work_dir\
bearer_token: my_token
````

It's possible to use parameter `-path` to specify the working directory. In that case the config parameters `input_path` and `output_path` will be ignored.

Example command line with the path and config parameters:
```bash
call.exe -config=c:\api\config.yml -path=c:\work_dir\ -url=/resource -method=GET
```

### GET request

GET Request Flow
The application receives parameters from CLI.
A GET request is sent to the target API.
The response body is read.
The result can be saved as:
    CSV (default)
    XML (optional)
Output encoding may be converted to Windows-1251 if required.

```
Pagination Support
You can call multiple pages manually:
-url=/orders?page=1
-url=/orders?page=2
-url=/orders?page=3

The application can save each page as a separate file:
python
orders_1.csv/xml
orders_2.csv/xml
orders_3.csv/xml
```
To make a GET request on url https://my-test.site/api/v1/resourse, run the application with the following command:
To use the mode of operation via the XML exchange format, use the parameter `-xml`

Conversion Rules XML/JSON:

Objects → XML nodes
Arrays → repeated child elements
Values → text nodes
Root tag name is derived from the service URL

```Example:
URL:
/orders
XML root:
<orders>
  ...
</orders>
```

```bash
call.exe -url=/resource -method=GET 
```

### POST request (CSV)

To make POST requests, create an `input.csv` file in the input directory, `input_path` parameter of config file or `-path` in command line. The first row must contain the JSON keys. The following rows must contain the JSON values.

Example of `input.csv` file:

```csv
key1,key2,key3
value1,value2,value3
value4,value5,value6
```
This example will create the following JSON payload:
```json
[
    {
        "key1": "value1",
        "key2": "value2",
        "key3": "value3"
    },
    {
        "key1": "value4",
        "key2": "value5",
        "key3": "value6"
    }
]
```
Example of a POST request, body is taken from `input.csv` file:
```bash
call.exe -url=/resource -method=POST
```
To send a single JSON object, create an `object.csv` file with two rows, keys and values.

### POST file as boundary (CSV)

To send a file as a boundary, use parameter `-boundary` with the file name.
```bash
call.exe -url=/resource -method=POST -boundary=[fileName]
```
`fileName` is the name of the file to be sent as a boundary, it must be placed in the input directory, `input_path` parameter in config file or `-path` in command line.

### POST request (XML)

The XML file is read, converted to JSON, and then forwarded to the target service.
This approach allows legacy systems to send structured data without needing native JSON support.

To use the mode of operation via the XML exchange format, use the parameter `-xml`

How It Works
The application reads the XML file.
XML encoding (Windows-1251) is automatically converted to UTF-8.
The <data> section is transformed into a JSON object.
A POST request is sent to the configured API URL.
The JSON body is passed as the request payload.

```XML Contract (Minimal Structure)
<request>
  <meta>
    <service>brands</service>
    <url>/import</url>
    <url>/import</url>
  </meta>
  <data>
    <!-- Any structure required by the target API -->
  </data>
</request>
```

Example of a POST request, body is taken from `input.xml` file:
Example: Create Brands (POST)
XML Input:
```
<request>
  <meta>
    <service>brands</service>
    <url>/import</url>
  </meta>
  <data>
    <manufacturers>
      <manufacturer>
        <guid>ddd45e49-a050-11e7-a038-b599dbcd0f29</guid>
        <name>
          <item>
            <en>EN TEST</en>
          </item>
        </name>
        <description>
          <item>
            <en>EN TEST</en>
          </item>
        </description>
      </manufacturer>
    </manufacturers>
  </data>
</request>
```
Generated JSON Body
```
{
  "manufacturers": [
    {
      "guid": "ddd45e49-a050-11e7-a038-b599dbcd0f29",
      "name": [
        {
          "en": "EN TEST"
        }
      ],
      "description": [
        {
          "en": "EN TEST"
        }
      ],
    }
  ]
}
```

```bash
call.exe -url=/resource -method=POST -xml 
```

### Help
To display the help message, run the application with the following command:
```bash
call.exe -help
```
