package main

import (
	"fmt"

	"github.com/tepzxl/contentflow/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		fmt.Println("run contentflow server: %v", err)
	}
}
