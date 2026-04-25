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
	readAllNodes(ctx, c)
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

func readAllNodes(ctx context.Context, c *opcua.Client) {
	fmt.Println("\n=== Browsing for nodes in ns=3 ===")

	var nodeIDs []string
	queue := []*ua.NodeID{ua.MustParseNodeID("ns=0;i=85")}
	visited := make(map[string]bool)

	for len(queue) > 0 && len(nodeIDs) < 50 {
		currentID := queue[0]
		queue = queue[1:]

		visitedKey := currentID.String()
		if visited[visitedKey] {
			continue
		}
		visited[visitedKey] = true

		refs, err := c.Node(currentID).ReferencedNodes(ctx, 0, ua.BrowseDirectionForward, ua.NodeClassVariable, true)
		if err != nil {
			continue
		}

		for _, ref := range refs {
			ns := ref.ID.Namespace()
			if ns == 3 {
				nodeIDs = append(nodeIDs, ref.ID.String())
			}
		}

		childRefs, _ := c.Node(currentID).ReferencedNodes(ctx, 0, ua.BrowseDirectionForward, ua.NodeClassObject, true)
		for _, child := range childRefs {
			if !visited[child.ID.String()] {
				queue = append(queue, child.ID)
			}
		}
	}

	if len(nodeIDs) == 0 {
		ns, _ := c.NamespaceArray(ctx)
		fmt.Printf("  No nodes found in ns=3. Available namespaces: %d\n", len(ns))
		return
	}

	fmt.Printf("  Found %d nodes\n\n", len(nodeIDs))

	fmt.Println("=== Reading Nodes ===")
	readValues(ctx, c, nodeIDs)
}

func readValues(ctx context.Context, c *opcua.Client, nodeIDs []string) {
	const batchSize = 20

	for i := 0; i < len(nodeIDs); i += batchSize {
		end := i + batchSize
		if end > len(nodeIDs) {
			end = len(nodeIDs)
		}

		batch := nodeIDs[i:end]
		nodesToRead := make([]*ua.ReadValueID, 0, len(batch))

		for _, nodeStr := range batch {
			nodeID := ua.MustParseNodeID(nodeStr)
			nodesToRead = append(nodesToRead, &ua.ReadValueID{
				NodeID:       nodeID,
				AttributeID:  ua.AttributeIDValue,
			})
		}

		req := &ua.ReadRequest{
			MaxAge:      0,
			NodesToRead: nodesToRead,
		}

		resp, err := c.Read(ctx, req)
		if err != nil {
			fmt.Printf("  Batch read error: %v\n", err)
			continue
		}

		for j, dr := range resp.Results {
			if dr.Status != ua.StatusOK {
				fmt.Printf("  %s: error %s\n", batch[j], dr.Status)
				continue
			}
			if dr.Value != nil {
				fmt.Printf("  %s = %v\n", batch[j], dr.Value.Value())
			}
		}
	}
}