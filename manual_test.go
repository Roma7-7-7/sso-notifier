package main

import (
	"log"
	"testing"
)

func Test(t *testing.T) {
	t.Skip()

	page, err := loadPage()
	if err != nil {
		t.Fatal(err)
	}

	table, err := parseShutdownsPage(page)
	if err != nil {
		t.Fatal(err)
	}

	group, err := renderGroup("1", table.Periods, table.Groups["1"].Items)
	if err != nil {
		t.Fatal(err)
	}
	log.Println(group)
}
