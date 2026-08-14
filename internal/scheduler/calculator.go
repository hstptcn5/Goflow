package scheduler

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

const maxCalendarSearchDays = 370

func NextDailyAfter(localTime, timezone string, after time.Time) (time.Time, error) {
	hour, minute, err := parseLocalTime(localTime)
	if err != nil {
		return time.Time{}, err
	}
	location, err := time.LoadLocation(timezone)
	if err != nil || timezone == "Local" {
		return time.Time{}, fmt.Errorf("timezone must be an IANA timezone")
	}
	if after.IsZero() {
		return time.Time{}, fmt.Errorf("after instant is required")
	}
	localAfter := after.In(location)
	date := time.Date(localAfter.Year(), localAfter.Month(), localAfter.Day(), 12, 0, 0, 0, location)
	for day := 0; day < maxCalendarSearchDays; day++ {
		candidateDate := date.AddDate(0, 0, day)
		candidate, ok := firstLocalOccurrence(location, candidateDate.Year(), candidateDate.Month(), candidateDate.Day(), hour, minute)
		if ok && candidate.After(after) {
			return candidate.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("could not calculate a daily occurrence within %d days", maxCalendarSearchDays)
}

func parseLocalTime(value string) (int, int, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 || len(parts[0]) != 2 || len(parts[1]) != 2 {
		return 0, 0, fmt.Errorf("local time must use HH:MM")
	}
	hour, hourErr := strconv.Atoi(parts[0])
	minute, minuteErr := strconv.Atoi(parts[1])
	if hourErr != nil || minuteErr != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, 0, fmt.Errorf("local time must use HH:MM")
	}
	return hour, minute, nil
}

func firstLocalOccurrence(location *time.Location, year int, month time.Month, day, hour, minute int) (time.Time, bool) {
	naive := time.Date(year, month, day, hour, minute, 0, 0, time.UTC)
	offsets := map[int]struct{}{}
	for sample := -36; sample <= 36; sample++ {
		_, offset := naive.Add(time.Duration(sample) * time.Hour).In(location).Zone()
		offsets[offset] = struct{}{}
	}
	candidates := make([]time.Time, 0, len(offsets))
	for offset := range offsets {
		candidate := naive.Add(-time.Duration(offset) * time.Second)
		local := candidate.In(location)
		if local.Year() == year && local.Month() == month && local.Day() == day &&
			local.Hour() == hour && local.Minute() == minute && local.Second() == 0 {
			candidates = append(candidates, candidate)
		}
	}
	if len(candidates) == 0 {
		return time.Time{}, false
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Before(candidates[j]) })
	return candidates[0], true
}
