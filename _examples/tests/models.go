package main

import "fmt"

type User struct {
	Id     int64
	Name   string
	Emails []string
}

func (u User) String() string {
	return fmt.Sprintf("User<%d %s %v>", u.Id, u.Name, u.Emails)
}

type Story struct {
	Id       int64
	Title    string
	AuthorId int64
	Author   *User `pg:"rel:has-one"`
}

func (s Story) String() string {
	return fmt.Sprintf("Story<%d %s %s>", s.Id, s.Title, s.Author)
}
