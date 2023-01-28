package main

import (
	"fmt"
	"testing"
)

func Test(t *testing.T) {
	t.Skip()

	page, err := loadPage("https://oblenergo.cv.ua/shutdowns/")
	if err != nil {
		t.Fatal(err)
	}

	table, err := parseShutdownsPage(page)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Print(table)
}
