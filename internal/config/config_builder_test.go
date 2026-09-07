package config

import (
	"strings"
	"testing"
)

func TestBuilderMutationsPreserveOrder(t *testing.T) {
	builder, err := NewBuilder([]byte("port: 2015\ndevMode: true\nredis:\n  store:\n    db: 1\nmysql:\n  logic:\n    database: test\n"))
	if err != nil {
		t.Fatal(err)
	}

	builder.
		Delete("mysql.logic").
		Delete("redis.store").
		After("port", "stationId", "").
		After("stationId", "stationTranscodeProfiles", map[string]any{
			"balanced": map[string]any{
				"codec":      "aac",
				"container":  "mp4",
				"bitrate":    256000,
				"sampleRate": 48000,
				"channels":   2,
				"version":    1,
			},
		})

	if builder.Has("mysql") || builder.Has("redis") {
		t.Fatal("empty config parents should be pruned")
	}
	if !builder.Has("stationTranscodeProfiles.balanced") {
		t.Fatal("inserted transcode profile is missing")
	}

	data, err := builder.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	port := strings.Index(content, "port:")
	stationID := strings.Index(content, "stationId:")
	profiles := strings.Index(content, "stationTranscodeProfiles:")
	devMode := strings.Index(content, "devMode:")
	if !(port < stationID && stationID < profiles && profiles < devMode) {
		t.Fatalf("unexpected config order:\n%s", content)
	}
	if strings.Contains(content, "mysql: {}") || strings.Contains(content, "redis: {}") {
		t.Fatalf("empty config blocks should not remain:\n%s", content)
	}
}

func TestBuilderBeforeAndNestedSet(t *testing.T) {
	builder, err := NewBuilder([]byte("mysql:\n  logic:\n    host: 127.0.0.1\n    port: 3306\n"))
	if err != nil {
		t.Fatal(err)
	}

	builder.
		Before("mysql.logic.port", "mysql.logic.charset", "utf8mb4").
		Set("mysql.logic.database", "app")

	if got := builder.Get("mysql.logic.charset"); got != "utf8mb4" {
		t.Fatalf("unexpected charset: %#v", got)
	}
	if got := builder.Get("mysql.logic.database"); got != "app" {
		t.Fatalf("unexpected database: %#v", got)
	}

	data, err := builder.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if strings.Index(content, "charset:") > strings.Index(content, "port:") {
		t.Fatalf("charset should be before port:\n%s", content)
	}
}

func TestBuilderAfterFieldsPreservesStructOrderAndComments(t *testing.T) {
	type Profile struct {
		Codec   string `yaml:"codec"`
		Bitrate int    `yaml:"bitrate" comment:"Target audio bitrate."`
	}
	type Configs struct {
		StationID                string             `yaml:"stationId"`
		StationPairingToken      string             `yaml:"stationPairingToken"`
		StationPublicBaseURL     string             `yaml:"stationPublicBaseUrl"`
		StationFFmpegPath        string             `yaml:"stationFFmpegPath" comment:"Optional external FFmpeg path. Leave empty to use FFmpeg from PATH."`
		StationTranscodeProfiles map[string]Profile `yaml:"stationTranscodeProfiles"`
	}

	builder, err := NewBuilder([]byte("port: 20156\ndevMode: true\n"))
	if err != nil {
		t.Fatal(err)
	}

	configs := Configs{
		StationID:            "polemo-station-dev",
		StationPairingToken:  "",
		StationPublicBaseURL: "http://192.168.31.129:20156",
		StationFFmpegPath:    "/usr/local/bin/ffmpeg",
		StationTranscodeProfiles: map[string]Profile{
			"balanced": {Codec: "aac", Bitrate: 256000},
		},
	}
	builder.AfterFields("port", configs)

	data, err := builder.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	keys := []string{
		"port:",
		"stationId:",
		"stationPairingToken:",
		"stationPublicBaseUrl:",
		"stationFFmpegPath:",
		"stationTranscodeProfiles:",
		"devMode:",
	}
	last := -1
	for _, key := range keys {
		index := strings.Index(content, key)
		if index < 0 {
			t.Fatalf("missing %s in config:\n%s", key, content)
		}
		if index <= last {
			t.Fatalf("unexpected field order at %s:\n%s", key, content)
		}
		last = index
	}

	if !strings.Contains(content, "# Optional external FFmpeg path. Leave empty to use FFmpeg from PATH.\nstationFFmpegPath:") {
		t.Fatalf("struct comment tag was not rendered:\n%s", content)
	}
}

func TestBuilderCommentOverridesStructComment(t *testing.T) {
	type Configs struct {
		StationFFmpegPath string `yaml:"stationFFmpegPath" comment:"Original comment."`
	}

	builder, err := NewBuilder([]byte("port: 20156\n"))
	if err != nil {
		t.Fatal(err)
	}

	builder.
		AfterFields("port", Configs{}).
		Comment("stationFFmpegPath", "Override comment.")

	data, err := builder.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "# Override comment.\nstationFFmpegPath:") {
		t.Fatalf("comment override was not rendered:\n%s", content)
	}
	if strings.Contains(content, "Original comment") {
		t.Fatalf("original comment should have been replaced:\n%s", content)
	}
}

func TestBuilderAccumulatesMutationError(t *testing.T) {
	builder, err := NewBuilder([]byte("port: 20156\n"))
	if err != nil {
		t.Fatal(err)
	}

	builder.
		After("missing", "stationId", "").
		Set("later", true)

	if builder.Err() == nil {
		t.Fatal("expected builder to retain the first mutation error")
	}
	if _, err := builder.Bytes(); err == nil {
		t.Fatal("Bytes should return accumulated mutation error")
	}
	if builder.Has("later") {
		t.Fatal("mutations after the first error should not be applied")
	}
}

func TestBuilderBeforeFieldsPreservesStructOrder(t *testing.T) {
	type Configs struct {
		StationID string `yaml:"stationId"`
		Region    string `yaml:"region"`
	}

	builder, err := NewBuilder([]byte("port: 20156\nlog:\n  logFile: app.log\n"))
	if err != nil {
		t.Fatal(err)
	}
	builder.BeforeFields("log", Configs{StationID: "station-1", Region: "local"})

	data, err := builder.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	port := strings.Index(content, "port:")
	stationID := strings.Index(content, "stationId:")
	region := strings.Index(content, "region:")
	log := strings.Index(content, "log:")
	if !(port < stationID && stationID < region && region < log) {
		t.Fatalf("unexpected config order:\n%s", content)
	}
}
