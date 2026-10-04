package memstore_test

import (
	"testing"

	"github.com/FreddieTheObserver/stubborn/store"
	"github.com/FreddieTheObserver/stubborn/store/memstore"
	"github.com/FreddieTheObserver/stubborn/store/storetest"
)

func TestConformance(t *testing.T) {
	storetest.Test(t, func(*testing.T) store.Store { return memstore.New() })
}
