package cmd

import (
	"strconv"
	"time"

	"github.com/adityasaky/gittuf/internal/gitstore"
)

var (
	role         string
	roleKeyPaths []string
)

// Borrowed from go-tuf
func parseExpires(e string) (time.Time, error) {
	days, err := strconv.Atoi(e)
	if err != nil {
		return time.Time{}, err
	}
	return time.Now().AddDate(0, 0, days).UTC(), nil
}

func getGittufRepo() (*gitstore.Repository, error) {
	// FIXME: handle detached GITDIRs
	return gitstore.LoadRepository(".")
}
