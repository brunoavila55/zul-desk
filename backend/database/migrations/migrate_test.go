package migrations

import "testing"

func TestMigrationVersion(t *testing.T) {
	t.Parallel()
	got, err := migrationVersion("000006_whatsapp_interface_settings.up.sql")
	if err != nil || got != 6 {
		t.Fatalf("migrationVersion() = %d, %v", got, err)
	}
}
