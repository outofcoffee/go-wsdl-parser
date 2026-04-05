package wsdlparser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWSDLParser(t *testing.T) {
	tests := []struct {
		name        string
		wsdlContent string
		wantVersion WSDLVersion
		wantErr     bool
	}{
		{
			name: "WSDL 1.1 with SOAP 1.1",
			wsdlContent: `<?xml version="1.0" encoding="UTF-8"?>
<definitions xmlns="http://schemas.xmlsoap.org/wsdl/"
             xmlns:soap="http://schemas.xmlsoap.org/wsdl/soap/"
             xmlns:tns="http://example.com/test">
    <binding name="TestBinding" type="tns:TestPortType">
        <soap:binding style="document" transport="http://schemas.xmlsoap.org/soap/http"/>
        <operation name="TestOperation">
            <soap:operation soapAction="http://example.com/test/action"/>
        </operation>
    </binding>
</definitions>`,
			wantVersion: WSDL1,
			wantErr:     false,
		},
		{
			name: "WSDL 1.1 with SOAP 1.2",
			wsdlContent: `<?xml version="1.0" encoding="UTF-8"?>
<definitions xmlns="http://schemas.xmlsoap.org/wsdl/"
             xmlns:soap12="http://schemas.xmlsoap.org/wsdl/soap12/"
             xmlns:tns="http://example.com/test">
    <binding name="TestBinding" type="tns:TestPortType">
        <soap12:binding style="document" transport="http://schemas.xmlsoap.org/soap/http"/>
        <operation name="TestOperation">
            <soap12:operation soapAction="http://example.com/test/action"/>
        </operation>
    </binding>
</definitions>`,
			wantVersion: WSDL1,
			wantErr:     false,
		},
		{
			name: "WSDL 2.0",
			wsdlContent: `<?xml version="1.0" encoding="UTF-8"?>
<description xmlns="http://www.w3.org/ns/wsdl"
             xmlns:tns="http://example.com/test">
    <interface name="TestInterface">
        <operation name="TestOperation">
            <input messageLabel="In" element="tns:TestRequest"/>
            <output messageLabel="Out" element="tns:TestResponse"/>
        </operation>
    </interface>
</description>`,
			wantVersion: WSDL2,
			wantErr:     false,
		},
		{
			name: "Invalid WSDL",
			wsdlContent: `<?xml version="1.0" encoding="UTF-8"?>
<invalid>
    <content>This is not a valid WSDL document</content>
</invalid>`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			wsdlPath := filepath.Join(tmpDir, "test.wsdl")
			err := os.WriteFile(wsdlPath, []byte(tt.wsdlContent), 0644)
			require.NoError(t, err)

			parser, err := NewWSDLParser(wsdlPath)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantVersion, parser.GetVersion())
		})
	}
}

func TestParseWSDL1Operations(t *testing.T) {
	wsdlPath := filepath.Join("testdata", "wsdl1-soap11", "service.wsdl")
	parser, err := NewWSDLParser(wsdlPath)
	require.NoError(t, err)

	assert.Equal(t, WSDL1, parser.GetVersion())

	ops := parser.GetOperations()
	assert.NotEmpty(t, ops, "should have at least one operation")

	for name, op := range ops {
		assert.NotEmpty(t, name, "operation name should not be empty")
		assert.Equal(t, name, op.Name)
	}
}

func TestParseWSDL1RPCStyle(t *testing.T) {
	wsdlPath := filepath.Join("testdata", "wsdl1-soap11-rpc", "service.wsdl")
	parser, err := NewWSDLParser(wsdlPath)
	require.NoError(t, err)

	assert.Equal(t, WSDL1, parser.GetVersion())

	ops := parser.GetOperations()
	require.Contains(t, ops, "getPetById")
	require.Contains(t, ops, "getPetByName")

	// getPetById declares style="rpc" at the operation level
	byId := ops["getPetById"]
	assert.Equal(t, StyleRPC, byId.Style, "operation-level style should be rpc")
	assert.Equal(t, "getPetById", byId.SOAPAction)

	// getPetByName inherits style from the binding (soap:operation omits style)
	byName := ops["getPetByName"]
	assert.Equal(t, StyleRPC, byName.Style, "binding-level style should be inherited")
}

func TestParseWSDL1DocumentStyleDefault(t *testing.T) {
	wsdlPath := filepath.Join("testdata", "wsdl1-soap11", "service.wsdl")
	parser, err := NewWSDLParser(wsdlPath)
	require.NoError(t, err)

	for name, op := range parser.GetOperations() {
		assert.Equal(t, StyleDocument, op.Style, "operation %s should default to document style", name)
	}
}

func TestParseWSDL2Operations(t *testing.T) {
	wsdlPath := filepath.Join("testdata", "wsdl2-soap12", "service.wsdl")
	parser, err := NewWSDLParser(wsdlPath)
	require.NoError(t, err)

	assert.Equal(t, WSDL2, parser.GetVersion())

	ops := parser.GetOperations()
	assert.NotEmpty(t, ops, "should have at least one operation")

	for name, op := range ops {
		assert.NotEmpty(t, name, "operation name should not be empty")
		assert.Equal(t, name, op.Name)
	}
}

func TestErrorCases(t *testing.T) {
	t.Run("non-existent file", func(t *testing.T) {
		_, err := NewWSDLParser("non_existent.wsdl")
		assert.Error(t, err)
	})

	t.Run("invalid XML", func(t *testing.T) {
		tmpDir := t.TempDir()
		wsdlPath := filepath.Join(tmpDir, "invalid.wsdl")
		err := os.WriteFile(wsdlPath, []byte("invalid xml content"), 0644)
		require.NoError(t, err)

		_, err = NewWSDLParser(wsdlPath)
		assert.Error(t, err)
	})

	t.Run("empty file", func(t *testing.T) {
		tmpDir := t.TempDir()
		wsdlPath := filepath.Join(tmpDir, "empty.wsdl")
		err := os.WriteFile(wsdlPath, []byte(""), 0644)
		require.NoError(t, err)

		_, err = NewWSDLParser(wsdlPath)
		assert.Error(t, err)
	})
}

func TestGetLocalPart(t *testing.T) {
	tests := []struct {
		name  string
		qname string
		want  string
	}{
		{"QName with prefix", "ns:localPart", "localPart"},
		{"QName without prefix", "localPart", "localPart"},
		{"Empty string", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetLocalPart(tt.qname)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetPrefix(t *testing.T) {
	tests := []struct {
		name  string
		qname string
		want  string
	}{
		{"QName with prefix", "ns:localPart", "ns"},
		{"QName without prefix", "localPart", ""},
		{"Empty string", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetPrefix(tt.qname)
			assert.Equal(t, tt.want, got)
		})
	}
}
