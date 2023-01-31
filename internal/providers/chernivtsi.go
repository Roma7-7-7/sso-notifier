package providers

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/Roma7-7-7/sso-notifier/models"
)

const url = "https://oblenergo.cv.ua/shutdowns/"

func ChernivtsiShutdowns() (models.ShutdownsTable, error) {
	html, err := loadPage()
	if err != nil {
		return models.ShutdownsTable{}, fmt.Errorf("failed to load shutdowns page: %w", err)
	}

	res, err := parseShutdownsPage(html)
	if err != nil {
		return models.ShutdownsTable{}, fmt.Errorf("failed to parse shutdowns page: %w", err)
	}

	return res, nil
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

func parseShutdownsPage(html []byte) (models.ShutdownsTable, error) {
	var res models.ShutdownsTable
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
	items := make([][]models.Status, len(groups))
	for i, g := range groups {
		items[i] = parseItems(gsv, g.Number)
	}

	res.Groups = make(map[string]models.ShutdownGroup, len(groups))
	for i, g := range groups {
		res.Groups[strconv.Itoa(g.Number)] = models.ShutdownGroup{
			Number: g.Number,
			Items:  items[i],
		}
	}

	return res, res.Validate()
}

func parseGroups(s *goquery.Selection) ([]models.ShutdownGroup, error) {
	var err error
	groups := make([]models.ShutdownGroup, 0)

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
		groups = append(groups, models.ShutdownGroup{
			Number: groupNum,
		})

		return true
	})

	return groups, err
}

func parsePeriods(s *goquery.Selection) ([]models.Period, error) {
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

	periods := make([]models.Period, len(hours)-1)
	for i := 0; i < len(periods); i++ {
		periods[i] = models.Period{
			From: hours[i],
			To:   hours[i+1],
		}
	}

	return periods, err
}

func parseItems(gsv *goquery.Selection, groupNum int) []models.Status {
	items := make([]models.Status, 0)

	node := gsv.Find(fmt.Sprintf("div[data-id='%d']", groupNum)).First()
	for _, sn := range node.Children().Nodes {
		if sn.Data != "o" && sn.Data != "u" && sn.Data != "s" {
			continue
		}

		var status models.Status
		switch strings.ToLower(goquery.NewDocumentFromNode(sn).Text()) {
		case "в":
			status = models.OFF
		case "з":
			status = models.ON
		default:
			status = models.MAYBE
		}
		items = append(items, status)
	}

	return items
}
