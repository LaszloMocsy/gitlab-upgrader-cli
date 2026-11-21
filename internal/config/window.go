package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	// Embed the time zone database so that windows with an IANA zone also
	// validate on hosts without one (Windows, minimal containers).
	_ "time/tzdata"
)

// MaintenanceWindow restricts mutating runs to recurring weekly time ranges.
// The zero value (an empty string in the config) means "any time".
//
// Syntax: DAYS HH:MM-HH:MM [ZONE]
//
//	DAYS  "daily", or a comma-separated list of days and day ranges:
//	      "Sat", "Sat,Sun", "Mon-Fri". Days are Mon Tue Wed Thu Fri Sat Sun.
//	ZONE  an IANA zone such as "Europe/Budapest", or "UTC" (the default).
//
// A window whose end is not after its start crosses midnight: "Fri 22:00-02:00"
// runs from Friday 22:00 to Saturday 02:00. "24:00" is allowed as an end.
type MaintenanceWindow struct {
	spec  string
	days  [7]bool // indexed by time.Weekday
	start int     // minutes after midnight
	end   int     // minutes after midnight, 1..1440
	loc   *time.Location
}

var weekdays = map[string]time.Weekday{
	"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday, "wed": time.Wednesday,
	"thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday,
}

// IsZero reports whether no window is configured.
func (w MaintenanceWindow) IsZero() bool { return w.spec == "" }

func (w MaintenanceWindow) String() string { return w.spec }

func (w MaintenanceWindow) MarshalText() ([]byte, error) { return []byte(w.spec), nil }

func (w *MaintenanceWindow) UnmarshalText(b []byte) error {
	fields := strings.Fields(string(b))
	if len(fields) == 0 {
		*w = MaintenanceWindow{}
		return nil
	}
	bad := func(why string) error {
		return fmt.Errorf(`%s; expected "DAYS HH:MM-HH:MM [ZONE]", for example "Sat 02:00-06:00 UTC", not %q`, why, string(b))
	}
	if len(fields) < 2 || len(fields) > 3 {
		return bad("wrong number of parts")
	}

	var nw MaintenanceWindow
	if err := nw.parseDays(fields[0]); err != nil {
		return bad(err.Error())
	}

	from, to, ok := strings.Cut(fields[1], "-")
	if !ok {
		return bad("missing time range")
	}
	var err error
	if nw.start, err = parseClock(from, false); err != nil {
		return bad(err.Error())
	}
	if nw.end, err = parseClock(to, true); err != nil {
		return bad(err.Error())
	}
	if nw.start == nw.end {
		return bad("start and end are the same")
	}

	nw.loc = time.UTC
	if len(fields) == 3 {
		if nw.loc, err = time.LoadLocation(fields[2]); err != nil {
			return bad(fmt.Sprintf("unknown time zone %q", fields[2]))
		}
	}
	nw.spec = strings.Join(fields, " ")
	*w = nw
	return nil
}

func (w *MaintenanceWindow) parseDays(s string) error {
	if strings.EqualFold(s, "daily") {
		for i := range w.days {
			w.days[i] = true
		}
		return nil
	}
	for item := range strings.SplitSeq(s, ",") {
		from, to, isRange := strings.Cut(item, "-")
		a, ok := weekdays[strings.ToLower(from)]
		if !ok {
			return fmt.Errorf("unknown day %q", from)
		}
		if !isRange {
			w.days[a] = true
			continue
		}
		z, ok := weekdays[strings.ToLower(to)]
		if !ok {
			return fmt.Errorf("unknown day %q", to)
		}
		for d := a; ; d = (d + 1) % 7 {
			w.days[d] = true
			if d == z {
				break
			}
		}
	}
	return nil
}

func parseClock(s string, isEnd bool) (int, error) {
	hh, mm, ok := strings.Cut(s, ":")
	h, errH := strconv.Atoi(hh)
	m, errM := strconv.Atoi(mm)
	if !ok || len(hh) != 2 || len(mm) != 2 || errH != nil || errM != nil || m > 59 || h > 24 || (h == 24 && (m != 0 || !isEnd)) {
		return 0, fmt.Errorf("invalid time %q", s)
	}
	return h*60 + m, nil
}

// Contains reports whether t falls inside the window. A zero window contains
// every time.
func (w MaintenanceWindow) Contains(t time.Time) bool {
	if w.IsZero() {
		return true
	}
	t = t.In(w.loc)
	length := w.end - w.start
	if length <= 0 {
		length += 24 * 60
	}
	// A window that crosses midnight may have started on the previous day.
	for _, back := range []int{0, 1} {
		day := t.AddDate(0, 0, -back)
		if !w.days[day.Weekday()] {
			continue
		}
		y, mo, d := day.Date()
		start := time.Date(y, mo, d, w.start/60, w.start%60, 0, 0, w.loc)
		end := start.Add(time.Duration(length) * time.Minute)
		if !t.Before(start) && t.Before(end) {
			return true
		}
	}
	return false
}
