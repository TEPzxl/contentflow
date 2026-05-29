package api_test

import (
	"context"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

const refreshCookieName = "contentflow_refresh_token"

func TestOpenAPIValid(t *testing.T) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile("openapi.yaml")
	if err != nil {
		t.Fatalf("load openapi.yaml: %v", err)
	}

	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("validate openapi.yaml: %v", err)
	}
}

func TestOpenAPIAuthRefreshTokenCookieContract(t *testing.T) {
	doc := loadOpenAPI(t)
	cookieScheme := doc.Components.SecuritySchemes["refreshCookie"]
	if cookieScheme == nil || cookieScheme.Value == nil {
		t.Fatal("refreshCookie security scheme is missing")
	}
	if cookieScheme.Value.Type != "apiKey" || cookieScheme.Value.In != "cookie" || cookieScheme.Value.Name != refreshCookieName {
		t.Fatalf("refreshCookie scheme = %#v, want apiKey cookie %s", cookieScheme.Value, refreshCookieName)
	}

	assertOptionalRefreshBody(t, doc, "/api/v1/auth/refresh")
	assertOptionalRefreshBody(t, doc, "/api/v1/auth/logout")
	assertNoRefreshTokenInEnvelope(t, doc, "LoginEnvelope")
	assertNoRefreshTokenInEnvelope(t, doc, "RefreshEnvelope")
}

func loadOpenAPI(t *testing.T) *openapi3.T {
	t.Helper()
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile("openapi.yaml")
	if err != nil {
		t.Fatalf("load openapi.yaml: %v", err)
	}
	return doc
}

func assertOptionalRefreshBody(t *testing.T, doc *openapi3.T, path string) {
	t.Helper()
	operation := doc.Paths.Value(path).Post
	if operation == nil {
		t.Fatalf("%s POST operation is missing", path)
	}
	if operation.RequestBody == nil || operation.RequestBody.Value == nil {
		t.Fatalf("%s request body is missing", path)
	}
	if operation.RequestBody.Value.Required {
		t.Fatalf("%s request body is required, want optional cookie-compatible body", path)
	}
	if operation.Security == nil || len(*operation.Security) == 0 || (*operation.Security)[0]["refreshCookie"] == nil {
		t.Fatalf("%s security = %#v, want refreshCookie", path, operation.Security)
	}
}

func assertNoRefreshTokenInEnvelope(t *testing.T, doc *openapi3.T, schemaName string) {
	t.Helper()
	schemaRef := doc.Components.Schemas[schemaName]
	if schemaRef == nil || schemaRef.Value == nil {
		t.Fatalf("schema %s is missing", schemaName)
	}
	data := schemaRef.Value.Properties["data"]
	if data == nil || data.Value == nil {
		t.Fatalf("schema %s.data is missing", schemaName)
	}
	if _, ok := data.Value.Properties["refresh_token"]; ok {
		t.Fatalf("schema %s.data.refresh_token is documented, want cookie-only refresh token", schemaName)
	}
}
