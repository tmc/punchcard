package main

import (
	"github.com/PuerkitoBio/goquery"

	"io"
)

type Timesheet struct {
	EmployeeID string
	Punches    Punches
}

func (t *Timesheet) Status() PunchType {
	if len(t.Punches) > 0 {
		return t.Punches[len(t.Punches)-1].Type
	}
	return OUT
}

func parseTimesheet(timesheetMarkup io.Reader) (*Timesheet, error) {
	d, err := goquery.NewDocumentFromReader(timesheetMarkup)

	if err != nil {
		return nil, err
	}

	punches, err := parsePunches(d)
	if err != nil {
		return nil, err
	}

	return &Timesheet{
		EmployeeID: d.Find("#DERIVED_TL_WEEK_EMPLID").Text(),
		Punches:    punches,
	}, nil
}
