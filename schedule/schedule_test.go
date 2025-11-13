package schedule

import (
	"testing"
)

func TestParseSpanishSchedule(t *testing.T) {
	// Your example input
	input := "jueves 8:00–13:00 15:00–18:00  viernes 8:00–13:00 15:00–18:00  sábado  domingo  lunes 8:00–13:00 15:00–18:00  martes 8:00–13:00 15:00–18:00  miércoles 8:00–13:00 15:00–18:00"

	schedule, err := Parse(input, true) // debug=true
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	formatted := schedule.Format(true) // debug=true
	t.Logf("Formatted output: %s", formatted)

	// Expected: "Mon-Fri 08:00-13:00, 15:00-18:00; Sat Closed; Sun Closed"
	// (or similar clean format)

	// Check Monday has correct ranges
	monday := schedule.Days[Monday]
	if monday.Closed {
		t.Errorf("Monday should not be closed")
	}
	if len(monday.Ranges) != 2 {
		t.Errorf("Monday should have 2 time ranges, got %d", len(monday.Ranges))
	}
	if len(monday.Ranges) >= 2 {
		if monday.Ranges[0].Start != "08:00" || monday.Ranges[0].End != "13:00" {
			t.Errorf("Monday first range incorrect: %v", monday.Ranges[0])
		}
		if monday.Ranges[1].Start != "15:00" || monday.Ranges[1].End != "18:00" {
			t.Errorf("Monday second range incorrect: %v", monday.Ranges[1])
		}
	}

	// Check Saturday is closed
	saturday := schedule.Days[Saturday]
	if !saturday.Closed {
		t.Errorf("Saturday should be closed")
	}

	// Check Sunday is closed
	sunday := schedule.Days[Sunday]
	if !sunday.Closed {
		t.Errorf("Sunday should be closed")
	}
}

func TestParseEnglishSchedule(t *testing.T) {
	input := "Monday 9:00-17:00 Tuesday 9:00-17:00 Wednesday closed Thursday 9:00-17:00 Friday 9:00-17:00 Saturday closed Sunday closed"

	schedule, err := Parse(input, true)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	formatted := schedule.Format(true)
	t.Logf("Formatted: %s", formatted)

	// Check Wednesday is closed
	wed := schedule.Days[Wednesday]
	if !wed.Closed {
		t.Errorf("Wednesday should be closed")
	}

	// Check Monday is open
	mon := schedule.Days[Monday]
	if mon.Closed {
		t.Errorf("Monday should be open")
	}
}

func TestFormatConsecutiveDays(t *testing.T) {
	// Manually create a schedule where Mon-Fri have same hours
	schedule := &WeekSchedule{}

	standardRanges := []TimeRange{
		{Start: "09:00", End: "17:00"},
	}

	// Mon-Fri: 9-5
	for day := Monday; day <= Friday; day++ {
		schedule.Days[day] = DaySchedule{
			Day:    day,
			Ranges: standardRanges,
			Closed: false,
		}
	}

	// Sat-Sun: Closed
	schedule.Days[Saturday] = DaySchedule{Day: Saturday, Closed: true}
	schedule.Days[Sunday] = DaySchedule{Day: Sunday, Closed: true}

	formatted := schedule.Format(true)
	t.Logf("Formatted: %s", formatted)

	// Should contain "Mon-Fri" as a range
	if !contains(formatted, "Mon-Fri") {
		t.Errorf("Expected 'Mon-Fri' in output, got: %s", formatted)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || contains(s[1:], substr)))
}
