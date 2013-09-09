package main

import "time"

// utcOffset returns the offset between the local timezone and UTC
// This follows Javascript's getTimezoneOffset() which is reverse from Go's default
func utcOffset() time.Duration {
	now := time.Now()
	_, offset := now.Zone()
	return time.Minute * time.Duration(offset/60) * -1
}
