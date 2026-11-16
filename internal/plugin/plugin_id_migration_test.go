package plugin

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/Pet-Max/cpa-plugin-tokens-statistic/internal/plugin/store"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

func TestForkPluginIDPreservesDataAcrossMigrationAndRollback(t *testing.T) {
	database := filepath.ToSlash(filepath.Join(t.TempDir(), "tokens-statistic.db"))
	request, err := json.Marshal(lifecycleRequest{
		ConfigYAML:    []byte("db: " + database + "\nsecret: migration-test-secret-with-32-bytes\n"),
		SchemaVersion: 3,
	})
	if err != nil {
		t.Fatal(err)
	}

	openRuntime := func(id string) *pluginRuntime {
		t.Helper()
		runtime := &pluginRuntime{}
		t.Cleanup(func() { _ = runtime.shutdown() })
		if _, err := runtime.register(request); err != nil {
			t.Fatalf("register %s: %v", id, err)
		}
		registration, err := json.Marshal(pluginapi.ManagementRegistrationRequest{
			ResourceBasePath: "/v0/resource/plugins/" + id,
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := runtime.registerManagement(registration); err != nil {
			t.Fatalf("register management for %s: %v", id, err)
		}
		if runtime.routes.pluginID != id || runtime.routes.statsPath != "/v0/management/plugins/"+id+"/stats" {
			t.Fatalf("unexpected routes for %s: %+v", id, runtime.routes)
		}
		if runtime.config.DataPath != database {
			t.Fatalf("database for %s = %q, want %q", id, runtime.config.DataPath, database)
		}
		return runtime
	}

	original := openRuntime("tokens-statistic")
	if err := original.store.Record(store.Usage{
		Dimensions:  store.Dimensions{Provider: "migration-test", Model: "test-model"},
		RequestedAt: time.Now().UTC(),
		Counters:    store.Counters{Requests: 1, TotalTokens: 17},
	}); err != nil {
		t.Fatal(err)
	}
	generation := original.apiKeyGeneration
	if err := original.shutdown(); err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{"tokens-statistic-fix", "tokens-statistic"} {
		runtime := openRuntime(id)
		stats, err := runtime.store.Query("retention")
		if err != nil {
			t.Fatal(err)
		}
		if stats.Summary.Requests != 1 || stats.Summary.TotalTokens != 17 {
			t.Fatalf("%s lost historical usage: %+v", id, stats.Summary)
		}
		if generation == 0 || runtime.apiKeyGeneration != generation {
			t.Fatalf("%s changed crypto generation from %d to %d", id, generation, runtime.apiKeyGeneration)
		}
		raw, err := json.Marshal(pluginapi.ManagementRequest{
			Method: http.MethodGet,
			Path:   runtime.routes.statsPath,
		})
		if err != nil {
			t.Fatal(err)
		}
		response, err := runtime.handleManagement(raw)
		if err != nil || response.StatusCode != http.StatusOK {
			t.Fatalf("%s stats route returned status %d, error %v", id, response.StatusCode, err)
		}
		if err := runtime.shutdown(); err != nil {
			t.Fatal(err)
		}
	}
}
