package main
import (
	"log"
	"os"
	"context"
	"github.com/GoogleCloudPlatform/functions-framework-go/funcframework"
	ff "github.com/kofc7186/ff2021_function"
)
func main() {
	ctx := context.Background()
	if err := funcframework.RegisterHTTPFunctionContext(ctx, "/", ff.MakeDocAndPrint); err != nil {
		log.Fatalf("funcframework.RegisterHTTPFunctionContext: %v\n", err)
	}
	// Use PORT environment variable, or default to 8080.
	port := "8080"
	if envPort := os.Getenv("PORT"); envPort != "" {
		port = envPort
	}
	if err := funcframework.Start(port); err != nil {
		log.Fatalf("funcframework.Start: %v\n", err)
	}
}
