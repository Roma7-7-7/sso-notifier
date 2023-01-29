package main

import "time"

var kyivTime = time.FixedZone("Kyiv", 2*60*60) //nolint:gomnd

func join(periods []Period, statuses []Status) ([]Period, []Status) {
	groupedPeriod := make([]Period, 0)
	groupedStatus := make([]Status, 0)

	currentFrom := periods[0].From
	currentTo := periods[0].To
	currentStatus := statuses[0]
	for i := 1; i < len(periods); i++ {
		if statuses[i] == currentStatus {
			currentTo = periods[i].To
			continue
		}
		groupedPeriod = append(groupedPeriod, Period{From: currentFrom, To: currentTo})
		groupedStatus = append(groupedStatus, currentStatus)
		currentFrom = periods[i].From
		currentTo = periods[i].To
		currentStatus = statuses[i]
	}
	groupedPeriod = append(groupedPeriod, Period{From: currentFrom, To: currentTo})
	groupedStatus = append(groupedStatus, currentStatus)

	return groupedPeriod, groupedStatus
}

func cutByKyivTime(periods []Period, items []Status) ([]Period, []Status) {
	currentKyivDateTime := time.Now().In(kyivTime).Format("15:04")

	cutPeriods := make([]Period, 0)
	cutItems := make([]Status, 0)
	for i := 0; i < len(periods); i++ {
		to := periods[i].To
		if to == "00:00" && i == len(periods)-1 { // yes, such a stupid hack
			to = "24:00"
		}
		if to > currentKyivDateTime {
			cutPeriods = append(cutPeriods, periods[i])
			cutItems = append(cutItems, items[i])
		}
	}

	return cutPeriods, cutItems
}
