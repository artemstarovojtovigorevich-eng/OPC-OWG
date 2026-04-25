package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	endpoint := "opc.tcp://127.0.0.1:4841"
	fmt.Printf("=== Connecting to %s ===\n", endpoint)

	c, err := opcua.NewClient(endpoint,
		opcua.SecurityPolicy("http://opcfoundation.org/UA/SecurityPolicy#None"),
		opcua.SecurityMode(ua.MessageSecurityModeNone),
		opcua.AuthAnonymous(),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	fmt.Println("Client created, connecting...")
	if err := c.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer c.Close(ctx)

	fmt.Println("Connected!")

	browseNamespaces(ctx, c)
	readAllUDStreamNodes(ctx, c)
}

func browseNamespaces(ctx context.Context, c *opcua.Client) {
	ns, err := c.NamespaceArray(ctx)
	if err != nil {
		fmt.Printf("NamespaceArray error: %v\n", err)
		return
	}

	fmt.Println("\n=== Namespaces ===")
	for i, n := range ns {
		fmt.Printf("  ns=%d: %s\n", i, n)
	}
}

func readAllUDStreamNodes(ctx context.Context, c *opcua.Client) {
	nodeIDs := []string{
		"ns=1;s=FastNumberOfUpdates",
		"ns=1;s=StepUp",
		"ns=1;s=AlternatingBoolean",
		"ns=1;s=DipData",
	}

	fmt.Println("\n=== Reading UDStream Nodes ===")
	for _, nodeStr := range nodeIDs {
		nodeID, err := ua.ParseNodeID(nodeStr)
		if err != nil {
			fmt.Printf("  %s: parse error - %v\n", nodeStr, err)
			continue
		}

		node := c.Node(nodeID)
		val, err := node.Value(ctx)
		if err != nil {
			fmt.Printf("  %s: read error - %v\n", nodeStr, err)
			continue
		}

		fmt.Printf("  %s = %v\n", nodeStr, val.Value())
	}
}