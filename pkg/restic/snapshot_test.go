package restic_test

import (
	"testing"
	"time"

	"github.com/restic/restic/pkg/restic"
	rtest "github.com/restic/restic/pkg/test"
)

func TestNewSnapshot(t *testing.T) {
	paths := []string{"/home/foobar"}

	_, err := restic.NewSnapshot(paths, nil, "foo", time.Now())
	rtest.OK(t, err)
}
