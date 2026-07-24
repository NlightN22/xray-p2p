package servercmd

import (
	"reflect"
	"testing"

	"github.com/spf13/cobra"
)

func TestForwardFlagsOmitsInheritedFlags(t *testing.T) {
	root := &cobra.Command{Use: "xp2p"}
	root.PersistentFlags().Bool("json", false, "")
	child := &cobra.Command{Use: "list"}
	child.Flags().Bool("pending", false, "")
	root.AddCommand(child)

	if err := root.ParseFlags([]string{"--json"}); err != nil {
		t.Fatal(err)
	}
	if err := child.Flags().Set("pending", "true"); err != nil {
		t.Fatal(err)
	}

	if got, want := forwardFlags(child, []string{"tail"}), []string{"--pending", "tail"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("forwardFlags() = %#v, want %#v", got, want)
	}
}
