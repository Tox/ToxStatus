package version

import (
	"fmt"
	"strconv"
	"time"
)

var (
	Number       string
	Revision     string
	RevisionTime string
)

func String() (string, error) {
	if Number == "" {
		return "toxstatus: development build", nil
	}

	return fmt.Sprintf("toxstatus: v%s-%s", Number, Revision), nil
}

func HumanRevisionTime() string {
	secs, err := strconv.ParseInt(RevisionTime, 10, 64)
	if err != nil {
		return ""
	}

	return time.Unix(secs, 0).UTC().String()
}
