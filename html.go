package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const url = "https://oblenergo.cv.ua/shutdowns/"

type ShutdownsTable struct {
	Date    string                   `json:"date"`
	Periods []Period                 `json:"periods"`
	Groups  map[string]ShutdownGroup `json:"groups"`
}

func (s ShutdownsTable) Validate() error {
	if s.Date == "" {
		return fmt.Errorf("invalid shutdowns table date=%s", s.Date)
	}
	if len(s.Periods) == 0 {
		return fmt.Errorf("shutdowns table periods list is empty")
	}
	for _, g := range s.Groups {
		if err := g.Validate(len(s.Periods)); err != nil {
			return fmt.Errorf("invalid shutdowns table group=%v: %w", g, err)
		}
	}
	return nil
}

type Status string

const (
	ON    Status = "Y"
	OFF   Status = "N"
	MAYBE Status = "M"
)

func (s Status) Hash() string {
	return string(s)
}

type ShutdownGroup struct {
	Number int
	Items  []Status
}

func (g ShutdownGroup) Hash() string {
	var buf bytes.Buffer
	for _, i := range g.Items {
		buf.WriteString(i.Hash())
	}
	return buf.String()
}

func (g ShutdownGroup) Validate(expectedItemsNum int) error {
	if g.Number < 1 {
		return fmt.Errorf("invalid shutdown group number=%d", g.Number)
	}
	if len(g.Items) != expectedItemsNum {
		return fmt.Errorf("invalid shutdown group items size; expected=%d but actual=%d", expectedItemsNum, len(g.Items))
	}
	return nil
}

type Period struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func loadPage() ([]byte, error) {
	// nolint:gomnd
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get shutdowns from page=%s: %w", url, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get shutdowns from page=%s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get shutdowns from page=%s: status=%s", url, resp.Status)
	}

	var res bytes.Buffer
	_, err = res.ReadFrom(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read shutdowns from page=%s: %w", url, err)
	}

	return res.Bytes(), nil
}

func parseShutdownsPage(html []byte) (ShutdownsTable, error) {
	var res ShutdownsTable
	var err error

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		return res, fmt.Errorf("failed o parse shutdowns page: %w", err)
	}

	gsv := doc.Find("div#gsv").First()
	if gsv == nil || gsv.Length() == 0 {
		return res, fmt.Errorf("failed o find shutdowns table by [div#gsv] selector")
	}

	res.Date = strings.TrimSpace(gsv.Find("ul p").First().Text())

	periods, err := parsePeriods(gsv)
	if err != nil || len(periods) == 0 {
		return res, fmt.Errorf("failed o parse shutdowns periods: %w", err)
	}
	res.Periods = periods

	groups, err := parseGroups(gsv)
	if err != nil || len(groups) == 0 {
		return res, fmt.Errorf("failed o parse shutdowns groups: %w", err)
	}
	items := make([][]Status, len(groups))
	for i, g := range groups {
		items[i] = parseItems(gsv, g.Number)
	}

	res.Groups = make(map[string]ShutdownGroup, len(groups))
	for i, g := range groups {
		res.Groups[strconv.Itoa(g.Number)] = ShutdownGroup{
			Number: g.Number,
			Items:  items[i],
		}
	}

	return res, res.Validate()
}

func parseGroups(s *goquery.Selection) ([]ShutdownGroup, error) {
	var err error
	groups := make([]ShutdownGroup, 0)

	s.Find("ul > li").EachWithBreak(func(i int, s *goquery.Selection) bool {
		val, exists := s.Attr("data-id")
		if !exists {
			err = fmt.Errorf("data-id attribute not found")
			return false
		}

		groupNum, sErr := strconv.Atoi(val)
		if sErr != nil {
			err = fmt.Errorf("failed o parse shutdown group number=%s on li node=%d: %w", val, i, sErr)
			return false
		}
		groups = append(groups, ShutdownGroup{
			Number: groupNum,
		})

		return true
	})

	return groups, err
}

func parsePeriods(s *goquery.Selection) ([]Period, error) {
	var err error

	row := s.Find("div > p").First()
	if row == nil || row.Length() == 0 {
		return nil, fmt.Errorf("failed o find shutdowns periods by [div p] selector")
	}
	hours := make([]string, 0)
	row.Find("u").EachWithBreak(func(i int, s *goquery.Selection) bool {
		val := s.Text()
		// HH:mm
		// nolint:gomnd
		if len(val) == 5 {
			hours = append(hours, val)
			return true
		}

		// 23:0000:00
		if len(val) == 10 && strings.HasSuffix(val, "00:00") {
			hours = append(hours, val[:5])
			hours = append(hours, val[5:])
			return true
		}

		err = fmt.Errorf("invalid shutdowns period=%s", val)
		return false
	})

	periods := make([]Period, len(hours)-1)
	for i := 0; i < len(periods); i++ {
		periods[i] = Period{
			From: hours[i],
			To:   hours[i+1],
		}
	}

	return periods, err
}

func parseItems(gsv *goquery.Selection, groupNum int) []Status {
	items := make([]Status, 0)

	node := gsv.Find(fmt.Sprintf("div[data-id='%d']", groupNum)).First()
	for _, sn := range node.Children().Nodes {
		if sn.Data != "o" && sn.Data != "u" && sn.Data != "s" {
			continue
		}

		var status Status
		switch strings.ToLower(goquery.NewDocumentFromNode(sn).Text()) {
		case "в":
			status = OFF
		case "з":
			status = ON
		default:
			status = MAYBE
		}
		items = append(items, status)
	}

	return items
}
