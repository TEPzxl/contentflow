package api

import (
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed openapi.yaml
var openAPIYAML []byte

func RegisterRoutes(r *gin.Engine) {
	r.GET("/openapi.yaml", OpenAPIYAML)
	r.GET("/docs", Redoc)
}

func OpenAPIYAML(c *gin.Context) {
	c.Data(http.StatusOK, "application/yaml; charset=utf-8", openAPIYAML)
}

func Redoc(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(redocHTML))
}

const redocHTML = `<!doctype html>
<html>
  <head>
    <meta charset="utf-8">
    <title>contentflow API Docs</title>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>body { margin: 0; padding: 0; }</style>
  </head>
  <body>
    <redoc spec-url="/openapi.yaml"></redoc>
    <script src="https://cdn.redoc.ly/redoc/latest/bundles/redoc.standalone.js"></script>
  </body>
</html>`
