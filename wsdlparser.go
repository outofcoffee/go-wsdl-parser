package wsdlparser

import (
	"encoding/xml"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/antchfx/xmlquery"
	"github.com/outofcoffee/go-wsdl-parser/wsdlmsg"
	"github.com/outofcoffee/go-wsdl-parser/xsd"
)

// WSDLVersion represents the version of WSDL being used
type WSDLVersion int

const (
	WSDL1 WSDLVersion = iota + 1
	WSDL2
)

const (
	WSDL1Namespace = "http://schemas.xmlsoap.org/wsdl/"
	WSDL2Namespace = "http://www.w3.org/ns/wsdl"
)

// WSDLDocProvider is the interface that provides the WSDL document
type WSDLDocProvider interface {
	GetWSDLDoc() *xmlquery.Node
	GetWSDLPath() string
	GetSchemaSystem() *xsd.SchemaSystem
}

// WSDLParser is the interface that all WSDL parsers must implement
type WSDLParser interface {
	WSDLDocProvider
	GetVersion() WSDLVersion
	GetOperations() map[string]*Operation
	GetOperation(name string) *Operation
	ValidateRequest(operation string, body []byte) error
	GetBindingName(op *Operation) string
	GetTargetNamespace() string
}

// BaseWSDLParser provides common functionality for WSDL parsers
type BaseWSDLParser struct {
	wsdlPath        string
	doc             *xmlquery.Node
	operations      map[string]*Operation
	schemas         *xsd.SchemaSystem
	targetNamespace string
}

// Operation style constants. Mirrors the WSDL 1.1 soap:binding / soap:operation
// style attribute values.
const (
	StyleDocument = "document"
	StyleRPC      = "rpc"
)

// Operation represents a WSDL operation
type Operation struct {
	Name       string
	SOAPAction string
	Input      *wsdlmsg.Message
	Output     *wsdlmsg.Message
	Fault      *wsdlmsg.Message
	Binding    string
	// Style is the SOAP binding style for this operation, either
	// "document" (default) or "rpc". Resolved from the soap:operation
	// element's style attribute, falling back to the enclosing
	// soap:binding element.
	Style string
}

func (p *BaseWSDLParser) GetWSDLPath() string {
	return p.wsdlPath
}

// GetBindingName returns the binding name for the given operation
func (p *BaseWSDLParser) GetBindingName(op *Operation) string {
	if op == nil {
		return ""
	}
	return op.Binding
}

// GetOperation returns the operation by name
func (p *BaseWSDLParser) GetOperation(name string) *Operation {
	return p.operations[name]
}

// GetOperations returns all operations
func (p *BaseWSDLParser) GetOperations() map[string]*Operation {
	return p.operations
}

// GetWSDLDoc returns the WSDL document
func (p *BaseWSDLParser) GetWSDLDoc() *xmlquery.Node {
	return p.doc
}

// GetSchemaSystem returns the schema system
func (p *BaseWSDLParser) GetSchemaSystem() *xsd.SchemaSystem {
	return p.schemas
}

// GetTargetNamespace returns the target namespace of the WSDL document
func (p *BaseWSDLParser) GetTargetNamespace() string {
	return p.targetNamespace
}

// GetNamespaceByPrefix returns the namespace URI for a given prefix
func (p *BaseWSDLParser) GetNamespaceByPrefix(prefix string) string {
	root := p.doc.SelectElement("*")
	if root == nil {
		return ""
	}
	for _, attr := range root.Attr {
		if attr.Name.Space == "xmlns" && attr.Name.Local == prefix {
			return attr.Value
		}
	}
	return ""
}

// toQName qualifies a node if it is not already qualified, and a targetNamespace is present
func (p *BaseWSDLParser) toQName(node string) string {
	if !strings.Contains(node, ":") {
		tns := p.GetTargetNamespace()
		if tns != "" {
			node = "tns:" + node
		}
	}
	return node
}

// resolveMessagesToElements generates synthetic schemas for non-element messages
func (p *BaseWSDLParser) resolveMessagesToElements() error {
	schemaSystem := *p.GetSchemaSystem()
	targetNamespace := p.GetTargetNamespace()

	for _, op := range p.GetOperations() {
		if op.Input != nil {
			inputMsg, err := resolveMessage(*op.Input, "input", op.Name, targetNamespace, schemaSystem, p.wsdlPath)
			if err != nil {
				return err
			}
			op.Input = &inputMsg
		}

		if op.Output != nil {
			outputMsg, err := resolveMessage(*op.Output, "output", op.Name, targetNamespace, schemaSystem, p.wsdlPath)
			if err != nil {
				return err
			}
			op.Output = &outputMsg
		}

		if op.Fault != nil {
			faultMsg, err := resolveMessage(*op.Fault, "fault", op.Name, targetNamespace, schemaSystem, p.wsdlPath)
			if err != nil {
				return err
			}
			op.Fault = &faultMsg
		}
	}
	return nil
}

// resolveMessage generates a synthetic schema for a message that is not an element
func resolveMessage(rawMsg wsdlmsg.Message, msgRole string, opName string, targetNamespace string, schemaSystem xsd.SchemaSystem, wsdlPath string) (wsdlmsg.Message, error) {
	filename := fmt.Sprintf("synthetic-%s-%s.xsd", opName, msgRole)

	switch rawMsg.GetMessageType() {
	case wsdlmsg.ElementMessageType:
		return rawMsg, nil

	case wsdlmsg.TypeMessageType:
		msg := rawMsg.(*wsdlmsg.TypeMessage)
		schema, elementName := wsdlmsg.CreateSinglePartSchema(msg.PartName, msg.Type, targetNamespace)

		if err := schemaSystem.ImportSchema(wsdlPath, filename, schema); err != nil {
			return nil, err
		}
		processedMsg := &wsdlmsg.ElementMessage{
			Element: &xml.Name{
				Local: GetLocalPart(elementName),
				Space: targetNamespace,
			},
		}
		return processedMsg, nil

	case wsdlmsg.CompositeMessageType:
		msg := rawMsg.(*wsdlmsg.CompositeMessage)
		elementName := msg.MessageName
		imports := schemaSystem.GetSchemasWithTargetNamespace(targetNamespace)
		schema := wsdlmsg.CreateCompositePartSchema(elementName, *msg.Parts, targetNamespace, imports)

		if err := schemaSystem.ImportSchema(wsdlPath, filename, schema); err != nil {
			return nil, err
		}
		processedMsg := &wsdlmsg.ElementMessage{
			Element: &xml.Name{
				Local: GetLocalPart(elementName),
				Space: targetNamespace,
			},
		}
		return processedMsg, nil
	}
	return nil, fmt.Errorf("unsupported message type: %T", rawMsg)
}

// NewWSDLParser creates a new version-aware WSDL parser instance
func NewWSDLParser(wsdlPath string) (WSDLParser, error) {
	slog.Debug("loading WSDL file", "path", wsdlPath)

	// Read and parse the WSDL file
	file, err := os.Open(wsdlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open WSDL file: %w", err)
	}
	defer file.Close()

	doc, err := xmlquery.Parse(file)
	if err != nil {
		return nil, fmt.Errorf("failed to parse WSDL file: %w", err)
	}

	// Detect WSDL version from root element namespace
	root := doc.SelectElement("*")
	if root == nil {
		return nil, fmt.Errorf("invalid WSDL document: no root element")
	}

	// Check if root has namespace attribute
	if len(root.Attr) == 0 {
		return nil, fmt.Errorf("invalid WSDL document: root element has no namespace")
	}

	var parser WSDLParser

	// Check for WSDL 2.0
	for _, attr := range root.Attr {
		if strings.Contains(attr.Value, WSDL2Namespace) {
			if parser, err = newWSDL2Parser(doc, wsdlPath); err != nil {
				return nil, err
			}
			break
		}
	}

	if parser == nil {
		// Check for WSDL 1.1
		for _, attr := range root.Attr {
			if strings.Contains(attr.Value, WSDL1Namespace) {
				if parser, err = newWSDL1Parser(doc, wsdlPath); err != nil {
					return nil, err
				}
				break
			}
		}
	}

	if parser == nil {
		return nil, fmt.Errorf("unsupported WSDL version")
	}
	return parser, nil
}

// GetLocalPart extracts the local part from a QName
func GetLocalPart(qname string) string {
	if idx := strings.Index(qname, ":"); idx != -1 {
		return qname[idx+1:]
	}
	return qname
}

// GetPrefix extracts the prefix from a QName
func GetPrefix(qname string) string {
	if idx := strings.Index(qname, ":"); idx != -1 {
		return qname[:idx]
	}
	return ""
}
