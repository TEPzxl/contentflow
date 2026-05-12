package response

import "github.com/gin-gonic/gin"

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func OK(c *gin.Context, data any) {
	c.JSON(200, gin.H{
		"data": data,
	})
}

func Error(c *gin.Context, status int, code string, message string) {
	c.JSON(status, ErrorResponse{
		Error: ErrorBody{
			Code:    code,
			Message: message,
		},
	})
}
