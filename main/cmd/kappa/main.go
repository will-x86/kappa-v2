package main

import (
	"context"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"github.com/seal/kappa-v2/main/internal/repository"
	"github.com/seal/kappa-v2/main/internal/router"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		panic(err)
	}
	log.Println("Hey")
	router.ApiRouter()
	ctx := context.Background()
	log.Println(os.Getenv("DATABASE_URL"))
	con, err := pgx.Connect(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		panic(err)
	}
	defer con.Close(ctx)
	q := repository.New(con)
	p, err := q.ListPlayers(ctx)
	if err != nil {
		panic(err)
	}
	for k, v := range p {
		log.Println(k, v)
	}
}
