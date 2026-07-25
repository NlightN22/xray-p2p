package output

import (
	"errors"
	"testing"

	"github.com/spf13/cobra"
)

func TestWrapMutationResultPublishesSuccessfulJSONResult(t *testing.T) {
	cmd := &cobra.Command{
		Use:  "remove",
		RunE: func(*cobra.Command, []string) error { return nil },
	}
	WrapMutationResult(cmd, "server user remove", func(*cobra.Command, []string) string {
		return "user-7"
	})
	ctx, resultValue := CaptureResult(cmd.Context())
	cmd.SetContext(ctx)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	result, ok := resultValue().(MutationResult)
	if !ok || result.Operation != "server user remove" || result.Entity != "user-7" {
		t.Fatalf("unexpected result: %#v", resultValue())
	}
}

func TestWrapMutationResultDoesNotPublishAfterHandlerFailure(t *testing.T) {
	want := errors.New("failed")
	cmd := &cobra.Command{
		Use:  "remove",
		RunE: func(*cobra.Command, []string) error { return want },
	}
	WrapMutationResult(cmd, "server user remove", func(*cobra.Command, []string) string {
		return "user-7"
	})
	ctx, resultValue := CaptureResult(cmd.Context())
	cmd.SetContext(ctx)

	if err := cmd.RunE(cmd, nil); !errors.Is(err, want) {
		t.Fatalf("got %v, want %v", err, want)
	}
	if resultValue() != nil {
		t.Fatalf("failure published result: %#v", resultValue())
	}
}
