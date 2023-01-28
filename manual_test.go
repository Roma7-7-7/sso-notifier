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

	group, err := renderGroup("1", table.Periods, table.Groups["1"].Items)
	fmt.Print(group)
}
