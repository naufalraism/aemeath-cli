package main

import (
	"log"

	"github.com/naufalraism/aemeath"
)

func main() {
	app := aemeath.New()

	aemeath.GET(app, "/", func(c *aemeath.Ctx, _ struct{}) (map[string]string, error) {
		return map[string]string{"message": "Halo dari Aemeath CLI!"}, nil
	})

	app.EnableDocs(aemeath.Info{Title: "Aemeath Project", Version: "1.0.0"})
	log.Println("🚀 Server berjalan di http://localhost:3000")
	log.Fatal(app.Listen(":3000"))
}
