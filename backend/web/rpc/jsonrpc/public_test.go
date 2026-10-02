package jsonrpc

import (
	"reflect"
	"testing"

	"github.com/komari-monitor/komari/database/models"
)

func TestPublicPingTaskClientsHideHiddenNodeUUIDsFromGuests(t *testing.T) {
	tasks := []models.PingTask{
		{Id: 1, Clients: models.StringArray{"visible-a", "hidden-b", "visible-c"}},
		{Id: 2, Clients: models.StringArray{"hidden-b"}},
	}
	hidden := map[string]bool{"hidden-b": true}
	guest := filterPublicPingTaskClients(tasks, hidden, false)
	if got := []string(guest[0].Clients); !reflect.DeepEqual(got, []string{"visible-a", "visible-c"}) {
		t.Fatalf("guest list contains hidden UUID or lost visible assignments: %v", got)
	}
	if len(guest[1].Clients) != 0 {
		t.Fatalf("hidden-only assignment leaked to guest: %v", guest[1].Clients)
	}
	admin := filterPublicPingTaskClients(tasks, hidden, true)
	if got := []string(admin[0].Clients); !reflect.DeepEqual(got, []string{"visible-a", "hidden-b", "visible-c"}) {
		t.Fatalf("admin list lost assignments: %v", got)
	}
	if got := []string(tasks[0].Clients); !reflect.DeepEqual(got, []string{"visible-a", "hidden-b", "visible-c"}) {
		t.Fatalf("filter mutated database model: %v", got)
	}
}
