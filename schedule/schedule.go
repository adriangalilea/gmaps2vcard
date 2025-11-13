package schedule

import (
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
)

// DayOfWeek represents a day (0=Sunday, 6=Saturday for consistency)
type DayOfWeek int

const (
	Sunday DayOfWeek = iota
	Monday
	Tuesday
	Wednesday
	Thursday
	Friday
	Saturday
)

// String returns the English 3-letter abbreviation
func (d DayOfWeek) String() string {
	return [...]string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}[d]
}

// FullName returns the full English day name
func (d DayOfWeek) FullName() string {
	return [...]string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}[d]
}

// TimeRange represents a single time period (e.g., "8:00-13:00")
type TimeRange struct {
	Start string // "8:00" or "08:00"
	End   string // "13:00" or "18:00"
}

func (tr TimeRange) String() string {
	return fmt.Sprintf("%s-%s", tr.Start, tr.End)
}

// DaySchedule represents all time ranges for a single day
type DaySchedule struct {
	Day    DayOfWeek
	Ranges []TimeRange
	Closed bool
}

// WeekSchedule represents a full week of business hours
type WeekSchedule struct {
	Days [7]DaySchedule
}

// Parse extracts structured schedule from raw text
func Parse(rawText string, debug bool) (*WeekSchedule, error) {
	if debug {
		log.Printf("[DEBUG] === Schedule Parser Start ===")
		log.Printf("[DEBUG] Raw input: %q", rawText)
	}

	schedule := &WeekSchedule{}

	// Initialize all days
	for i := 0; i < 7; i++ {
		schedule.Days[i] = DaySchedule{
			Day:    DayOfWeek(i),
			Ranges: []TimeRange{},
			Closed: true, // Default to closed
		}
	}

	// Normalize the input
	normalized := normalizeText(rawText, debug)
	if debug {
		log.Printf("[DEBUG] Normalized: %q", normalized)
	}

	// Extract day-hour pairs
	parsed := parseScheduleText(normalized, debug)

	// Populate schedule
	for day, ranges := range parsed {
		if dayNum, ok := dayNameToNumber(day); ok {
			schedule.Days[dayNum].Ranges = ranges
			schedule.Days[dayNum].Closed = len(ranges) == 0
			if debug {
				log.Printf("[DEBUG] Set %s (%d): %v (closed=%v)",
					day, dayNum, ranges, schedule.Days[dayNum].Closed)
			}
		}
	}

	if debug {
		log.Printf("[DEBUG] === Schedule Parser Complete ===")
	}

	return schedule, nil
}

// normalizeText cleans and standardizes the input text
func normalizeText(text string, debug bool) string {
	if debug {
		log.Printf("[DEBUG] Normalizing text...")
	}

	// Remove special Unicode characters (en-dash, em-dash → hyphen)
	text = strings.ReplaceAll(text, "–", "-")
	text = strings.ReplaceAll(text, "—", "-")
	text = strings.ReplaceAll(text, "\u2013", "-") // en-dash
	text = strings.ReplaceAll(text, "\u2014", "-") // em-dash

	// Normalize whitespace
	text = strings.ReplaceAll(text, "\t", " ")
	text = strings.ReplaceAll(text, "\r", "")
	text = regexp.MustCompile(` +`).ReplaceAllString(text, " ")

	// Remove newlines (everything on one line for easier parsing)
	text = strings.ReplaceAll(text, "\n", " ")

	// Translate Spanish day names to English
	dayTranslations := map[string]string{
		"lunes":     "monday",
		"martes":    "tuesday",
		"miércoles": "wednesday",
		"miercoles": "wednesday", // without accent
		"jueves":    "thursday",
		"viernes":   "friday",
		"sábado":    "saturday",
		"sabado":    "saturday", // without accent
		"domingo":   "sunday",
	}

	lower := strings.ToLower(text)
	for spanish, english := range dayTranslations {
		lower = strings.ReplaceAll(lower, spanish, english)
	}

	// Also handle "closed" / "cerrado"
	lower = strings.ReplaceAll(lower, "cerrado", "closed")

	if debug {
		log.Printf("[DEBUG] After translation: %q", lower)
	}

	return lower
}

// parseScheduleText extracts day -> time ranges mapping
func parseScheduleText(text string, debug bool) map[string][]TimeRange {
	result := make(map[string][]TimeRange)

	if debug {
		log.Printf("[DEBUG] Parsing schedule text...")
	}

	// Pattern: day_name followed by time ranges or "closed"
	// e.g., "monday 8:00-13:00 15:00-18:00" or "sunday closed"

	days := []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}

	for _, day := range days {
		// Find the day in the text
		dayIdx := strings.Index(text, day)
		if dayIdx == -1 {
			if debug {
				log.Printf("[DEBUG] Day %s not found in text", day)
			}
			continue
		}

		// Extract everything after the day name until the next day or end
		afterDay := text[dayIdx+len(day):]

		// Find where the next day starts
		nextDayIdx := len(afterDay)
		for _, otherDay := range days {
			if otherDay != day {
				if idx := strings.Index(afterDay, otherDay); idx != -1 && idx < nextDayIdx {
					nextDayIdx = idx
				}
			}
		}

		dayContent := strings.TrimSpace(afterDay[:nextDayIdx])

		if debug {
			log.Printf("[DEBUG] %s content: %q", day, dayContent)
		}

		// Check if closed
		if strings.Contains(dayContent, "closed") {
			result[day] = []TimeRange{}
			if debug {
				log.Printf("[DEBUG] %s: CLOSED", day)
			}
			continue
		}

		// Extract time ranges (format: HH:MM-HH:MM)
		timeRegex := regexp.MustCompile(`(\d{1,2}:\d{2})\s*-\s*(\d{1,2}:\d{2})`)
		matches := timeRegex.FindAllStringSubmatch(dayContent, -1)

		if len(matches) > 0 {
			ranges := make([]TimeRange, 0, len(matches))
			for _, match := range matches {
				tr := TimeRange{
					Start: normalizeTime(match[1]),
					End:   normalizeTime(match[2]),
				}
				ranges = append(ranges, tr)
				if debug {
					log.Printf("[DEBUG] %s: found range %s", day, tr)
				}
			}
			result[day] = ranges
		} else {
			if debug {
				log.Printf("[DEBUG] %s: no time ranges found", day)
			}
		}
	}

	return result
}

// normalizeTime ensures consistent time format (HH:MM)
func normalizeTime(t string) string {
	parts := strings.Split(t, ":")
	if len(parts) != 2 {
		return t
	}

	hour := parts[0]
	minute := parts[1]

	// Pad hour to 2 digits
	if len(hour) == 1 {
		hour = "0" + hour
	}

	return hour + ":" + minute
}

// dayNameToNumber converts English day name to DayOfWeek
func dayNameToNumber(name string) (DayOfWeek, bool) {
	name = strings.ToLower(name)
	mapping := map[string]DayOfWeek{
		"sunday":    Sunday,
		"monday":    Monday,
		"tuesday":   Tuesday,
		"wednesday": Wednesday,
		"thursday":  Thursday,
		"friday":    Friday,
		"saturday":  Saturday,
	}

	day, ok := mapping[name]
	return day, ok
}

// Format produces clean, human-readable output
func (ws *WeekSchedule) Format(debug bool) string {
	if debug {
		log.Printf("[DEBUG] === Formatting Schedule ===")
	}

	// Group consecutive days with identical schedules
	groups := ws.groupConsecutiveDays(debug)

	parts := make([]string, 0, len(groups))
	for _, group := range groups {
		formatted := formatGroup(group, debug)
		parts = append(parts, formatted)
	}

	result := strings.Join(parts, "; ")

	if debug {
		log.Printf("[DEBUG] Final formatted output: %q", result)
		log.Printf("[DEBUG] === Formatting Complete ===")
	}

	return result
}

// dayGroup represents consecutive days with the same schedule
type dayGroup struct {
	StartDay DayOfWeek
	EndDay   DayOfWeek
	Ranges   []TimeRange
	Closed   bool
}

// groupConsecutiveDays finds days with identical schedules
func (ws *WeekSchedule) groupConsecutiveDays(debug bool) []dayGroup {
	groups := []dayGroup{}

	// Start with Monday for business-friendly ordering
	orderedDays := []DayOfWeek{Monday, Tuesday, Wednesday, Thursday, Friday, Saturday, Sunday}

	i := 0
	for i < len(orderedDays) {
		current := orderedDays[i]
		currentSched := ws.Days[current]

		// Find how many consecutive days have the same schedule
		endIdx := i
		for endIdx+1 < len(orderedDays) {
			next := orderedDays[endIdx+1]
			nextSched := ws.Days[next]

			if schedulesEqual(currentSched, nextSched) {
				endIdx++
			} else {
				break
			}
		}

		group := dayGroup{
			StartDay: orderedDays[i],
			EndDay:   orderedDays[endIdx],
			Ranges:   currentSched.Ranges,
			Closed:   currentSched.Closed,
		}

		groups = append(groups, group)

		if debug {
			log.Printf("[DEBUG] Group: %s-%s, Closed=%v, Ranges=%v",
				group.StartDay, group.EndDay, group.Closed, group.Ranges)
		}

		i = endIdx + 1
	}

	return groups
}

// schedulesEqual checks if two day schedules are identical
func schedulesEqual(a, b DaySchedule) bool {
	if a.Closed != b.Closed {
		return false
	}

	if len(a.Ranges) != len(b.Ranges) {
		return false
	}

	for i := range a.Ranges {
		if a.Ranges[i] != b.Ranges[i] {
			return false
		}
	}

	return true
}

// formatGroup formats a day group
func formatGroup(g dayGroup, debug bool) string {
	// Day range
	var dayPart string
	if g.StartDay == g.EndDay {
		dayPart = g.StartDay.String()
	} else {
		dayPart = fmt.Sprintf("%s-%s", g.StartDay, g.EndDay)
	}

	// Hours part
	if g.Closed || len(g.Ranges) == 0 {
		return fmt.Sprintf("%s Closed", dayPart)
	}

	// Sort ranges by start time for consistency
	ranges := make([]TimeRange, len(g.Ranges))
	copy(ranges, g.Ranges)
	sort.Slice(ranges, func(i, j int) bool {
		return ranges[i].Start < ranges[j].Start
	})

	rangeParts := make([]string, len(ranges))
	for i, r := range ranges {
		rangeParts[i] = r.String()
	}

	return fmt.Sprintf("%s %s", dayPart, strings.Join(rangeParts, ", "))
}
