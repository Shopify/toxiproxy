package main

import (
	pg "github.com/go-pg/pg/v10"
	"github.com/go-pg/pg/v10/orm"
)

func setupDB(addr, database string) (*pg.DB, error) {
	db := pg.Connect(&pg.Options{
		Addr:     addr,
		User:     "postgres",
		Database: database,
	})

	err := createSchema(db)
	if err != nil {
		return nil, err
	}

	err = seed(db)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func createSchema(db *pg.DB) error {
	models := []interface{}{
		(*User)(nil),
		(*Story)(nil),
	}

	for _, model := range models {
		err := db.Model(model).CreateTable(&orm.CreateTableOptions{
			Temp: true,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func seed(db *pg.DB) error {
	user1 := &User{
		Name:   "admin",
		Emails: []string{"admin1@admin", "admin2@admin"},
	}
	_, err := db.Model(user1).Insert()
	if err != nil {
		return err
	}

	_, err = db.Model(&User{
		Name:   "root",
		Emails: []string{"root1@root", "root2@root"},
	}).Insert()
	if err != nil {
		return err
	}

	story1 := &Story{
		Title:    "Cool story",
		AuthorId: user1.Id,
	}
	_, err = db.Model(story1).Insert()
	if err != nil {
		return err
	}

	return nil
}
