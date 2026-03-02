package cli

import "testing"

func TestRootCommand_Subcommands(t *testing.T) {
	root := NewRootCommand()

	if _, _, err := root.Find([]string{"serve"}); err != nil {
		t.Fatalf("expected serve command, got error: %v", err)
	}

	auth, _, err := root.Find([]string{"auth"})
	if err != nil {
		t.Fatalf("expected auth command, got error: %v", err)
	}

	if _, _, err := auth.Find([]string{"login"}); err != nil {
		t.Fatalf("expected auth login command, got error: %v", err)
	}
}

func TestAuthLoginCommand_ModeFlagRemoved(t *testing.T) {
	root := NewRootCommand()
	login, _, err := root.Find([]string{"auth", "login"})
	if err != nil {
		t.Fatalf("find auth login command: %v", err)
	}

	if login.Flags().Lookup("mode") != nil {
		t.Fatal("expected auth login --mode flag to be removed from mainline")
	}
}
