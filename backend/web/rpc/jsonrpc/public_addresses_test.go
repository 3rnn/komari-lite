package jsonrpc

import (
	"context"
	"testing"

	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/pkg/rpc"
)

func TestGuestNodeAPIsNeverExposePublicAddressInventory(t *testing.T) {
	flags.DatabaseType = "sqlite"
	flags.DatabaseFile = "file:public_address_inventory_privacy?mode=memory&cache=shared"
	db := dbcore.GetDBInstance()
	if err := db.AutoMigrate(&models.Client{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Client{
		UUID: "public-inventory-test", Token: "public-inventory-token", IPv4: "8.8.8.8", IPv6: "2001:4860::1",
		IPAddresses: models.IPAddresses{{Address: "8.8.8.8", Family: "ipv4", Primary: true}, {Address: "9.9.9.9", Family: "ipv4"}},
	}).Error; err != nil {
		t.Fatal(err)
	}
	guest := rpc.NewContextWithMeta(context.Background(), &rpc.ContextMeta{})
	listed, rpcErr := publicGetNodesInformation(guest, &rpc.JsonRpcRequest{})
	if rpcErr != nil {
		t.Fatalf("public node list: %+v", rpcErr)
	}
	found := false
	for _, node := range listed.([]models.Client) {
		if node.UUID != "public-inventory-test" {
			continue
		}
		found = true
		if node.IPv4 != "" || node.IPv6 != "" || len(node.IPAddresses) != 0 {
			t.Fatalf("public list leaked addresses: %+v", node.IPAddresses)
		}
	}
	if !found {
		t.Fatal("test node missing from public list")
	}
	result, rpcErr := getNodes(guest, &rpc.JsonRpcRequest{})
	if rpcErr != nil {
		t.Fatalf("guest getNodes: %+v", rpcErr)
	}
	node := result.(map[string]models.Client)["public-inventory-test"]
	if len(node.IPAddresses) != 0 {
		t.Fatalf("guest getNodes leaked addresses: %+v", node.IPAddresses)
	}
	admin := rpc.NewContextWithMeta(context.Background(), &rpc.ContextMeta{Principal: rpc.PrincipalFromRole(rpc.RoleAdmin)})
	result, rpcErr = getNodes(admin, &rpc.JsonRpcRequest{})
	if rpcErr != nil {
		t.Fatalf("admin getNodes: %+v", rpcErr)
	}
	adminNode := result.(map[string]models.Client)["public-inventory-test"]
	if adminNode.IPv4 != "8.8.8.8" || len(adminNode.IPAddresses) != 2 || adminNode.IPAddresses[1].Address != "9.9.9.9" {
		t.Fatalf("admin getNodes omitted public address inventory: %+v", adminNode.IPAddresses)
	}
}
