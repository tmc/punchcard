package main

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	IN = PunchType(iota)
	TempOUT
	BackIN
	OUT
)

type Punches []Punch

type PunchType int

type Punch struct {
	time.Time
	Type PunchType
}

// NewPunch attempts to create a punch from a date-time string and a PunchType
// The expected string format is:
// 	"01/02/2006 15:04:05"
func NewPunch(dateTime string, punchType PunchType) (Punch, error) {
	t, err := time.ParseInLocation("01/02/2006 15:04:05PM", dateTime, time.Local)
	if err != nil {
		return Punch{}, err
	}
	return Punch{t, punchType}, nil
}

func (p Punch) In() bool {
	return p.Type == IN || p.Type == BackIN
}

func (p Punch) Out() bool {
	return !p.In()
}

func (pt PunchType) String() string {
	switch pt {
	case IN:
		return "In"
	case TempOUT:
		return "Temp Out"
	case BackIN:
		return "In (from temp)"
	case OUT:
		fallthrough
	default:
		return "Out"
	}
}

func (p Punch) String() string {
	return p.Type.String() + p.Format(" 1/2 3:04:05PM")
}

func (p Punches) Len() int {
	return len(p)
}

func (p Punches) Less(i, j int) bool {
	return p[i].Time.Unix() < p[j].Time.Unix()
}
func (p Punches) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func (p Punches) Duration() time.Duration {
	reversed := make(Punches, len(p))
	copy(reversed, p)
	sort.Sort(sort.Reverse(reversed))

	if len(p) == 0 {
		return time.Duration(0)
	}

	var result time.Duration
	prevOut := time.Now()
	prevState := OUT

	for _, s := range reversed {
		if s.In() {
			if prevState == OUT || prevState == TempOUT {
				result += prevOut.Sub(s.Time)
			}
			prevState = IN
		}
		if s.Out() {
			prevOut = s.Time
			prevState = OUT
		}
	}

	return result
}

var prefixToType = map[string]PunchType{
	"PUNCH_TIME_1": IN,
	"PUNCH_TIME_2": TempOUT,
	"PUNCH_TIME_3": BackIN,
	"PUNCH_TIME_4": OUT,
}

func parsePunches(doc *goquery.Document) (Punches, error) {
	punches := Punches([]Punch{})

	rowToDate := map[int]time.Time{}
	baseDateText := doc.Find("span#DATE_DAY1").Text()
	baseDate, err := time.ParseInLocation("01/02/2006", baseDateText, time.Local)

	if err != nil {
		fmt.Println("text:", baseDateText, doc.Find("span#DATE_DAY1").Length())
		return nil, fmt.Errorf("Error parsing base date (%s): %s", baseDateText, err)
	}

	for prefix, punchType := range prefixToType {
		nodes := doc.Find(fmt.Sprintf("span[id^=%s]", prefix))
		for i := 0; i < nodes.Length(); i++ {
			punch := nodes.Eq(i)

			punchText := strings.TrimSpace(punch.Text())
			id, _ := punch.Attr("id")

			if punchText == "" {
				continue
			}

			rowNum, err := strconv.Atoi(id[len(id)-1:])
			if err != nil {
				return nil, fmt.Errorf("atoi error parsing row num: %s", err)
			}

			var date time.Time
			for ok := false; !ok; date, ok = rowToDate[rowNum] {
				rowDateQuery := doc.Find(fmt.Sprintf("[id='PUNCH_DATE_DISPLAY$%d']", rowNum))
				if rowDateQuery != nil && rowDateQuery.Text() != "" {
					date, err := time.ParseInLocation("1/2", rowDateQuery.Text(), time.Local)

					if err != nil {
						return nil, fmt.Errorf("error parsing new row date (%s): %s", rowDateQuery.Text(), err)
					}
					// TODO: breaks on year boundary?
					date = date.AddDate(baseDate.Year(), 0, 0)

					rowToDate[rowNum] = date
				} else {
					rowNum--
				}
			}

			dateTimeText := date.Format("01/02/2006 ") + punchText
			p, err := NewPunch(dateTimeText, punchType)
			if err != nil {
				return nil, fmt.Errorf("Error parsing punch (%s): %s", dateTimeText, err)
			}
			punches = append(punches, p)
		}
	}
	sort.Sort(punches)
	return punches, nil
}
