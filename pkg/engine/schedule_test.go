package engine

import (
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestLaneSchedule_UnmarshalYAML_SingleString(t *testing.T) {
	yamlStr := `
frequency: daily
time: "02:00"
`
	var sched LaneSchedule
	if err := yaml.Unmarshal([]byte(yamlStr), &sched); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sched.Frequency != ScheduleDaily {
		t.Errorf("expected frequency daily, got %q", sched.Frequency)
	}

	if len(sched.Time) != 1 || sched.Time[0] != "02:00" {
		t.Errorf("expected time [\"02:00\"], got %v", sched.Time)
	}
}

func TestLaneSchedule_UnmarshalYAML_ArrayString(t *testing.T) {
	tests := []struct {
		name     string
		yamlStr  string
		wantTime []string
	}{
		{
			name: "single item array",
			yamlStr: `
frequency: daily
time: ["02:00"]
`,
			wantTime: []string{"02:00"},
		},
		{
			name: "multiple times",
			yamlStr: `
frequency: daily
time: ["02:00", "06:00", "10:00"]
`,
			wantTime: []string{"02:00", "06:00", "10:00"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sched LaneSchedule
			if err := yaml.Unmarshal([]byte(tt.yamlStr), &sched); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(sched.Time) != len(tt.wantTime) {
				t.Fatalf("expected %d times, got %d", len(tt.wantTime), len(sched.Time))
			}

			for i, got := range sched.Time {
				if got != tt.wantTime[i] {
					t.Errorf("time[%d]: expected %q, got %q", i, tt.wantTime[i], got)
				}
			}
		})
	}
}

func TestLaneSchedule_UnmarshalYAML_Weekly(t *testing.T) {
	yamlStr := `
frequency: weekly
time: "02:00"
daysOfWeek: ["Monday", "Wednesday", "Friday"]
`
	var sched LaneSchedule
	if err := yaml.Unmarshal([]byte(yamlStr), &sched); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sched.Frequency != ScheduleWeekly {
		t.Errorf("expected frequency weekly, got %q", sched.Frequency)
	}

	if len(sched.Time) != 1 || sched.Time[0] != "02:00" {
		t.Errorf("expected time [\"02:00\"], got %v", sched.Time)
	}

	if len(sched.DaysOfWeek) != 3 {
		t.Errorf("expected 3 days, got %d", len(sched.DaysOfWeek))
	}
}

func TestLaneSchedule_UnmarshalYAML_Cron(t *testing.T) {
	yamlStr := `
frequency: cron
cronExpression: "0 2 * * 1,3,5"
`
	var sched LaneSchedule
	if err := yaml.Unmarshal([]byte(yamlStr), &sched); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sched.Frequency != ScheduleCron {
		t.Errorf("expected frequency cron, got %q", sched.Frequency)
	}

	if sched.CronExpression != "0 2 * * 1,3,5" {
		t.Errorf("expected cronExpression %q, got %q", "0 2 * * 1,3,5", sched.CronExpression)
	}

	if len(sched.Time) != 0 {
		t.Errorf("expected empty time, got %v", sched.Time)
	}
}

func TestLaneSchedule_UnmarshalYAML_InvalidType(t *testing.T) {
	yamlStr := `
frequency: daily
time: 123
`
	var sched LaneSchedule
	err := yaml.Unmarshal([]byte(yamlStr), &sched)
	if err == nil {
		t.Fatal("expected error for invalid time type")
	}

	if !strings.Contains(err.Error(), "expected string or array") {
		t.Errorf("expected error about string or array, got: %v", err)
	}
}

func TestValidateSchedule_ValidDaily(t *testing.T) {
	tests := []struct {
		name  string
		sched *LaneSchedule
	}{
		{
			name: "single time",
			sched: &LaneSchedule{
				Frequency: ScheduleDaily,
				Time:      []string{"02:00"},
			},
		},
		{
			name: "multiple times",
			sched: &LaneSchedule{
				Frequency: ScheduleDaily,
				Time:      []string{"02:00", "06:00", "10:00", "14:00", "18:00", "22:00"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSchedule(tt.sched, "task", "inbox")
			if err != nil {
				t.Errorf("expected no error, got: %v", err)
			}
		})
	}
}

func TestValidateSchedule_ValidWeekly(t *testing.T) {
	sched := &LaneSchedule{
		Frequency:  ScheduleWeekly,
		Time:       []string{"02:00", "04:00"},
		DaysOfWeek: []string{"Monday", "Wednesday", "Friday"},
	}

	err := validateSchedule(sched, "task", "inbox")
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidateSchedule_ValidMonthly(t *testing.T) {
	sched := &LaneSchedule{
		Frequency:   ScheduleMonthly,
		Time:        []string{"00:00"},
		DaysOfMonth: []int{1, 15},
	}

	err := validateSchedule(sched, "task", "inbox")
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidateSchedule_ValidCron(t *testing.T) {
	tests := []struct {
		name string
		expr string
	}{
		{"5 fields", "0 2 * * 1,3,5"},
		{"every 4 hours", "0 */4 * * *"},
		{"6 fields with year", "0 2 * * 1,3,5 2026"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sched := &LaneSchedule{
				Frequency:      ScheduleCron,
				CronExpression: tt.expr,
			}

			err := validateSchedule(sched, "task", "inbox")
			if err != nil {
				t.Errorf("expected no error, got: %v", err)
			}
		})
	}
}

func TestValidateSchedule_InvalidFrequency(t *testing.T) {
	sched := &LaneSchedule{
		Frequency: "invalid",
	}

	err := validateSchedule(sched, "task", "inbox")
	if err == nil {
		t.Error("expected error for invalid frequency")
	}
}

func TestValidateSchedule_MissingTime(t *testing.T) {
	sched := &LaneSchedule{
		Frequency: ScheduleDaily,
	}

	err := validateSchedule(sched, "task", "inbox")
	if err == nil {
		t.Error("expected error for missing time")
	}
}

func TestValidateSchedule_InvalidTimeFormat(t *testing.T) {
	tests := []struct {
		name string
		time string
	}{
		{"hour 25", "25:00"},
		{"minute 60", "02:60"},
		{"no colon", "0200"},
		{"single digit", "2:00"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sched := &LaneSchedule{
				Frequency: ScheduleDaily,
				Time:      []string{tt.time},
			}

			err := validateSchedule(sched, "task", "inbox")
			if err == nil {
				t.Error("expected error for invalid time format")
			}
		})
	}
}

func TestValidateSchedule_WeeklyMissingDaysOfWeek(t *testing.T) {
	sched := &LaneSchedule{
		Frequency: ScheduleWeekly,
		Time:      []string{"02:00"},
	}

	err := validateSchedule(sched, "task", "inbox")
	if err == nil {
		t.Error("expected error for missing daysOfWeek")
	}
}

func TestValidateSchedule_InvalidDayName(t *testing.T) {
	sched := &LaneSchedule{
		Frequency:  ScheduleWeekly,
		Time:       []string{"02:00"},
		DaysOfWeek: []string{"Monday", "Funday"},
	}

	err := validateSchedule(sched, "task", "inbox")
	if err == nil {
		t.Error("expected error for invalid day name")
	}
}

func TestValidateSchedule_MonthlyMissingDaysOfMonth(t *testing.T) {
	sched := &LaneSchedule{
		Frequency: ScheduleMonthly,
		Time:      []string{"00:00"},
	}

	err := validateSchedule(sched, "task", "inbox")
	if err == nil {
		t.Error("expected error for missing daysOfMonth")
	}
}

func TestValidateSchedule_InvalidDayOfMonth(t *testing.T) {
	tests := []struct {
		name string
		day  int
	}{
		{"zero", 0},
		{"32", 32},
		{"negative", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sched := &LaneSchedule{
				Frequency:   ScheduleMonthly,
				Time:        []string{"00:00"},
				DaysOfMonth: []int{tt.day},
			}

			err := validateSchedule(sched, "task", "inbox")
			if err == nil {
				t.Errorf("expected error for day of month %d", tt.day)
			}
		})
	}
}

func TestValidateSchedule_CronMissingExpression(t *testing.T) {
	sched := &LaneSchedule{
		Frequency: ScheduleCron,
	}

	err := validateSchedule(sched, "task", "inbox")
	if err == nil {
		t.Error("expected error for missing cron expression")
	}
}

func TestValidateSchedule_CronInvalidFieldCount(t *testing.T) {
	tests := []struct {
		name string
		expr string
	}{
		{"too few", "0 2 *"},
		{"too many", "0 2 * * 1 2026 extra"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sched := &LaneSchedule{
				Frequency:      ScheduleCron,
				CronExpression: tt.expr,
			}

			err := validateSchedule(sched, "task", "inbox")
			if err == nil {
				t.Errorf("expected error for invalid cron expression: %q", tt.expr)
			}
		})
	}
}

func TestValidateSchedule_ConflictingFields_Daily(t *testing.T) {
	tests := []struct {
		name  string
		sched *LaneSchedule
	}{
		{
			name: "daysOfWeek on daily",
			sched: &LaneSchedule{
				Frequency:  ScheduleDaily,
				Time:       []string{"02:00"},
				DaysOfWeek: []string{"Monday"},
			},
		},
		{
			name: "daysOfMonth on daily",
			sched: &LaneSchedule{
				Frequency:   ScheduleDaily,
				Time:        []string{"02:00"},
				DaysOfMonth: []int{1},
			},
		},
		{
			name: "cronExpression on daily",
			sched: &LaneSchedule{
				Frequency:      ScheduleDaily,
				Time:           []string{"02:00"},
				CronExpression: "0 2 * * *",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSchedule(tt.sched, "task", "inbox")
			if err == nil {
				t.Error("expected error for conflicting fields on daily")
			}
		})
	}
}

func TestValidateSchedule_ConflictingFields_Weekly(t *testing.T) {
	tests := []struct {
		name  string
		sched *LaneSchedule
	}{
		{
			name: "daysOfMonth on weekly",
			sched: &LaneSchedule{
				Frequency:   ScheduleWeekly,
				Time:        []string{"02:00"},
				DaysOfWeek:  []string{"Monday"},
				DaysOfMonth: []int{1},
			},
		},
		{
			name: "cronExpression on weekly",
			sched: &LaneSchedule{
				Frequency:      ScheduleWeekly,
				Time:           []string{"02:00"},
				DaysOfWeek:     []string{"Monday"},
				CronExpression: "0 2 * * *",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSchedule(tt.sched, "task", "inbox")
			if err == nil {
				t.Error("expected error for conflicting fields on weekly")
			}
		})
	}
}

func TestValidateSchedule_ConflictingFields_Monthly(t *testing.T) {
	tests := []struct {
		name  string
		sched *LaneSchedule
	}{
		{
			name: "daysOfWeek on monthly",
			sched: &LaneSchedule{
				Frequency:   ScheduleMonthly,
				Time:        []string{"00:00"},
				DaysOfMonth: []int{1},
				DaysOfWeek:  []string{"Monday"},
			},
		},
		{
			name: "cronExpression on monthly",
			sched: &LaneSchedule{
				Frequency:      ScheduleMonthly,
				Time:           []string{"00:00"},
				DaysOfMonth:    []int{1},
				CronExpression: "0 2 * * *",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSchedule(tt.sched, "task", "inbox")
			if err == nil {
				t.Error("expected error for conflicting fields on monthly")
			}
		})
	}
}

func TestValidateSchedule_ConflictingFields_Cron(t *testing.T) {
	tests := []struct {
		name  string
		sched *LaneSchedule
	}{
		{
			name: "time on cron",
			sched: &LaneSchedule{
				Frequency:      ScheduleCron,
				CronExpression: "0 2 * * *",
				Time:           []string{"02:00"},
			},
		},
		{
			name: "daysOfWeek on cron",
			sched: &LaneSchedule{
				Frequency:      ScheduleCron,
				CronExpression: "0 2 * * *",
				DaysOfWeek:     []string{"Monday"},
			},
		},
		{
			name: "daysOfMonth on cron",
			sched: &LaneSchedule{
				Frequency:      ScheduleCron,
				CronExpression: "0 2 * * *",
				DaysOfMonth:    []int{1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSchedule(tt.sched, "task", "inbox")
			if err == nil {
				t.Error("expected error for conflicting fields on cron")
			}
		})
	}
}

func TestValidateSchedule_CaseInsensitiveDayNames(t *testing.T) {
	tests := []struct {
		name string
		days []string
	}{
		{"lowercase", []string{"monday", "wednesday", "friday"}},
		{"uppercase", []string{"MONDAY", "WEDNESDAY", "FRIDAY"}},
		{"mixed", []string{"Monday", "wednesday", "FRIDAY"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sched := &LaneSchedule{
				Frequency:  ScheduleWeekly,
				Time:       []string{"02:00"},
				DaysOfWeek: tt.days,
			}

			err := validateSchedule(sched, "task", "inbox")
			if err != nil {
				t.Errorf("expected no error for case-insensitive day names, got: %v", err)
			}
		})
	}
}

func TestValidateSchedule_ErrorMessages_Context(t *testing.T) {
	sched := &LaneSchedule{
		Frequency: "invalid",
	}

	err := validateSchedule(sched, "mymodule", "mylane")
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "module mymodule lane mylane") {
		t.Errorf("expected error to contain context, got: %v", err)
	}
}
