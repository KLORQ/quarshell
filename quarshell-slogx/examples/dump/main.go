// Package main demonstrates Dump / Serialize for nested structs via saferefl.
package main

import (
	"context"

	"github.com/KLORQ/quarshell-slogx"
)

type Address struct {
	City    string
	Country string
}

type User struct {
	Name     string
	Email    string
	Password string
	Address  Address
	Tags     []string
}

func main() {
	log := slogx.New(
		slogx.WithFormat(slogx.FormatJSON),
		slogx.WithMaskRules(
			slogx.NewMaskRules().Add("Email", slogx.MaskEmail),
		),
		slogx.WithRemoval(slogx.NewRemovalSet("Password")),
		slogx.WithDumpLimits(5, 64),
	)

	user := User{
		Name:     "Alice",
		Email:    "alice@example.com",
		Password: "secret",
		Address:  Address{City: "Berlin", Country: "DE"},
		Tags:     []string{"vip", "beta"},
	}

	ctx := context.Background()

	// DumpWith uses the logger masking/removal rules while serializing the value.
	log.InfoContext(ctx, "user snapshot",
		slogx.DumpWith(log, "user", user),
	)

	// DumpGroupWith emits nested fields as a slog group.
	log.InfoContext(ctx, "user group",
		slogx.DumpGroupWith(log, "profile", user),
	)

	// Extra dump options can override depth / masking for a single call.
	log.InfoContext(ctx, "shallow dump",
		slogx.DumpWith(log, "user", user, slogx.WithDumpDepth(1)),
	)
}
