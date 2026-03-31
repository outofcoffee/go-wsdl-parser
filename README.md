# go-wsdl-parser

A Go library for parsing WSDL (Web Services Description Language) files. Supports WSDL 1.1 and 2.0 with SOAP 1.1/1.2 bindings.

## Features

- Automatic WSDL version detection (1.1 and 2.0)
- SOAP 1.1 and 1.2 binding support
- XSD schema extraction and resolution
- Handles element-based, type-based, and composite messages
- Generates synthetic XSD schemas for non-element message parts

## Installation

```bash
go get github.com/outofcoffee/go-wsdl-parser
```

## Usage

```go
import "github.com/outofcoffee/go-wsdl-parser"

parser, err := wsdlparser.NewWSDLParser("path/to/service.wsdl")
if err != nil {
    log.Fatal(err)
}

// List all operations
for name, op := range parser.GetOperations() {
    fmt.Printf("Operation: %s (SOAPAction: %s)\n", name, op.SOAPAction)
}

// Get a specific operation
op := parser.GetOperation("GetWeather")

// Access the schema system for XSD resolution
schemas := parser.GetSchemaSystem()
```

The parser returns `Operation` structs containing input, output, and fault messages. Each message references an XSD element that can be resolved through the schema system.

## Packages

| Package | Description |
|---------|-------------|
| `wsdlparser` | Main parser interface and WSDL 1.1/2.0 implementations |
| `wsdlmsg` | Message types and synthetic schema generation |
| `xsd` | XSD schema processing, imports, and resolution |

## Contributing

```bash
# Run tests
go test -v ./...

# Run tests with race detection
go test -race ./...
```

Requires Go 1.23+.

## Licence

Apache 2.0 - see [LICENSE](LICENSE).
