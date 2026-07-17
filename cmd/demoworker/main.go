// Command demoworker registers simulated agents against a running
// orchestrator and works any task they are assigned. Demo/dev use only —
// the worker loop lives in internal/simagent.
package main

import (
	"context"
	"log"
	"os"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/simagent"
)

var demoAgents = []struct {
	name string
	tier string
}{
	{"Pyromancer", "opus"},
	{"Runescribe", "sonnet"},
	{"Apprentice", "haiku"},
}

func main() {
	addr := "127.0.0.1:50051"
	if env := os.Getenv("GRPC_ADDR"); env != "" {
		addr = env
	}

	ctx := context.Background()
	for _, a := range demoAgents {
		id, err := simagent.Spawn(ctx, addr, a.name, a.tier)
		if err != nil {
			log.Fatalf("spawn %s: %v", a.name, err)
		}
		log.Printf("registered %s (%s) -> %s", a.name, a.tier, id)
	}

	select {}
}
