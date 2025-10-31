package providers

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/Roma7-7-7/sso-notifier/internal/dal"
)

const url = "https://oblenergo.cv.ua/shutdowns/"

func ChernivtsiShutdowns(ctx context.Context) (dal.Shutdowns, error) {
	html, err := loadPage(ctx)
	if err != nil {
		return dal.Shutdowns{}, fmt.Errorf("load shutdowns page: %w", err)
	}

	res, err := parseShutdownsPage(html)
	if err != nil {
		return dal.Shutdowns{}, fmt.Errorf("parse shutdowns page: %w", err)
	}

	return res, nil
}

func loadPage(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("get shutdowns from page=%s: %w", url, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get shutdowns from page=%s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get shutdowns from page=%s: status=%s", url, resp.Status)
	}

	var res bytes.Buffer
	_, err = res.ReadFrom(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read shutdowns from page=%s: %w", url, err)
	}

	return res.Bytes(), nil
}

func parseShutdownsPage(html []byte) (dal.Shutdowns, error) {
	var res dal.Shutdowns
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
	items := make([][]dal.Status, len(groups))
	for i, g := range groups {
		items[i] = parseItems(gsv, g.Number)
	}

	res.Groups = make(map[string]dal.ShutdownGroup, len(groups))
	for i, g := range groups {
		res.Groups[strconv.Itoa(g.Number)] = dal.ShutdownGroup{
			Number: g.Number,
			Items:  items[i],
		}
	}

	return res, validateShutdowns(res)
}

func validateShutdowns(s dal.Shutdowns) error {
	if s.Date == "" {
		return fmt.Errorf("invalid shutdowns table date=%s", s.Date)
	}
	if len(s.Periods) == 0 {
		return fmt.Errorf("shutdowns table periods list is empty")
	}
	for _, g := range s.Groups {
		if err := validateShutdownGroup(g, len(s.Periods)); err != nil {
			return fmt.Errorf("invalid shutdowns table group=%v: %w", g, err)
		}
	}
	return nil
}

func validateShutdownGroup(g dal.ShutdownGroup, expectedItemsNum int) error {
	if g.Number < 1 {
		return fmt.Errorf("invalid shutdown group number=%d", g.Number)
	}
	if len(g.Items) != expectedItemsNum {
		return fmt.Errorf("invalid shutdown group items size; expected=%d but actual=%d", expectedItemsNum, len(g.Items))
	}
	return nil
}

func parseGroups(s *goquery.Selection) ([]dal.ShutdownGroup, error) {
	var err error
	groups := make([]dal.ShutdownGroup, 0)

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
		groups = append(groups, dal.ShutdownGroup{
			Number: groupNum,
		})

		return true
	})

	return groups, err
}

func parsePeriods(s *goquery.Selection) ([]dal.Period, error) {
	row := s.Find("div > p").First()
	if row == nil || row.Length() == 0 {
		return nil, fmt.Errorf("failed o find shutdowns periods by [div p] selector")
	}

	hours := make([]string, 0)

	// The new structure has nested <u> tags:
	// <u><b>HH<em>:00</em></b><u>HH<em>:30</em></u></u>
	// Special case at the end: <u><b>23<em>:00</em></b><b>00<em>:00</em></b><u>23<em>:30</em></u></u>
	// We need to extract both the hour mark (HH:00) and half-hour mark (HH:30)
	row.Find("u").Each(func(i int, outer *goquery.Selection) {
		// Check if this is a top-level <u> tag (contains <b> tag)
		if outer.Find("b").Length() > 0 {
			// Extract all <b> tags (normally one, but two in the last entry)
			outer.Find("b").Each(func(j int, b *goquery.Selection) {
				hourText := b.Text()
				hours = append(hours, hourText)
			})

			// Extract half-hour mark from nested <u>HH<em>:30</em></u>
			halfHourText := outer.Find("u").Text()
			if halfHourText != "" {
				hours = append(hours, halfHourText)
			}
		}
	})

	// Handle edge case: if we have "00:00" after "23:00" and "23:30", replace it with "24:00"
	// The order should be: ...23:00, 00:00, 23:30 -> ...23:00, 23:30, 24:00
	for i := 0; i < len(hours)-1; i++ {
		if hours[i] == "23:00" && hours[i+1] == "00:00" && i+2 < len(hours) && hours[i+2] == "23:30" {
			// Swap 00:00 and 23:30, then convert 00:00 to 24:00
			hours[i+1], hours[i+2] = hours[i+2], "24:00"
			break
		}
	}

	if len(hours) < 2 {
		return nil, fmt.Errorf("not enough time periods found: got %d", len(hours))
	}

	periods := make([]dal.Period, len(hours)-1)
	for i := 0; i < len(periods); i++ {
		periods[i] = dal.Period{
			From: hours[i],
			To:   hours[i+1],
		}
	}

	return periods, nil
}

func parseItems(gsv *goquery.Selection, groupNum int) []dal.Status {
	items := make([]dal.Status, 0)

	node := gsv.Find(fmt.Sprintf("div[data-id='%d']", groupNum)).First()
	for _, sn := range node.Children().Nodes {
		if sn.Data != "o" && sn.Data != "u" && sn.Data != "s" {
			continue
		}

		var status dal.Status
		switch strings.ToLower(goquery.NewDocumentFromNode(sn).Text()) {
		case "в":
			status = dal.OFF
		case "з":
			status = dal.ON
		default:
			status = dal.MAYBE
		}
		items = append(items, status)
	}

	return items
}
