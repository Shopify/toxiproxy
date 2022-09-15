package main

import (
	"context"
	"log"
	"os"

	pg "github.com/go-pg/pg/v10"
)

func main() {
	err := run()
	if err != nil {
		log.Printf("ERROR: %v", err)
		os.Exit(1)
	}
}

func run() error {
	db, err := setupDB(":5432", "sample")
	if err != nil {
		return err
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.Ping(ctx); err != nil {
		return err
	}

	process(db)

	return nil
}

func process(db *pg.DB) error {
	var users []User
	err := db.Model(&users).Select()
	if err != nil {
		return err
	}

	for _, user := range users {
		log.Printf("user: %v", user)
	}

	// Select story and associated author in one query.
	story := new(Story)
	err = db.Model(story).
		Relation("Author").
		Where("story.id = ?", 1).
		Select()
	if err != nil {
		return err
	}

	log.Printf("story: %v", story)

	return nil
}
